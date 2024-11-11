package extensionmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-host/pkg/random"
	pluginstypes "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

type WaiterInfo struct {
	ch  chan any
	out any
}

type extensionRuntimeInfo struct {
	conn               *websocket.Conn
	connWaiters        map[string]*WaiterInfo
	cfg                pluginstypes.ExtensionConfig
	hostImplementation func(ctx context.Context, in any) (any, error)
}

type failureProcessor func(err error)

// WSManager is a websocket manager that manages websocket connections
// between the server and the plugins.
type WSManager struct {
	debug                                   bool
	logger                                  *slog.Logger
	failureProcessor                        failureProcessor
	lis                                     net.Listener
	pmsPort                                 int
	mu                                      *sync.Mutex
	pluginRegistrationChannel               chan string
	managerErrorsChannel                    chan error
	waitersByRequestID                      map[string]*WaiterInfo
	pluginIDBySecret                        map[string]string
	channelByPluginID                       map[string]*websocket.Conn
	extensionRuntimeInfoByExtensionPointIDs map[string][]extensionRuntimeInfo
	pluginsOrdered                          bool
}

// NewWSManager creates a new WSManager instance.
func NewWSManager() *WSManager {
	m := &WSManager{
		mu:                                      &sync.Mutex{},
		logger:                                  slog.Default(),
		pluginRegistrationChannel:               make(chan string),
		managerErrorsChannel:                    make(chan error),
		waitersByRequestID:                      make(map[string]*WaiterInfo),
		pluginIDBySecret:                        make(map[string]string),
		channelByPluginID:                       make(map[string]*websocket.Conn),
		extensionRuntimeInfoByExtensionPointIDs: make(map[string][]extensionRuntimeInfo),
	}

	return m.WithFailureProcessor(m.DefaultFailureProcessor)
}

// WithDebug enables debug mode for the WSManager.
func (m *WSManager) WithDebug() *WSManager {
	m.debug = true
	return m
}

// WithFixedPort sets the fixed port for the WSManager.
func (m *WSManager) WithFixedPort(port int) *WSManager {
	m.pmsPort = port
	return m
}

// WithLogger sets the logger for the WSManager.
func (m *WSManager) WithLogger(logger *slog.Logger) *WSManager {
	m.logger = logger
	return m
}

// WithFailureProcessor sets the custom failure processor for the WSManager.
func (m *WSManager) WithFailureProcessor(p failureProcessor) *WSManager {
	m.failureProcessor = p
	return m
}

// Init initializes the WSManager.
func (m *WSManager) Init() (*WSManager, error) {
	err := m.listen()
	if err != nil {
		return m, err
	}
	go func() {
		err := m.startServer()
		if err != nil {
			panic(fmt.Errorf("init plugins manager: %w", err))
		}
	}()
	return m, nil
}

func (m *WSManager) handle(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.logger.Error("upgrade:", slog.String("err", err.Error()))
		return
	}
	connWaiters := make(map[string]*WaiterInfo)
	defer c.Close()
	for {
		mt, inMsg, err := c.ReadMessage()
		if err != nil {
			if m.logger.Enabled(context.Background(), slog.LevelDebug) {
				m.logger.Debug("read message", slog.String("err", err.Error()))
			}
			connWaiters = m.processChannelClosing(connWaiters)
			break
		}

		if m.debug {
			m.logger.Info(
				"Received message",
				slog.String("localAddr", c.LocalAddr().String()),
				slog.String("remoteAddr", c.RemoteAddr().String()),
				slog.String("msg", string(inMsg)),
			)
		}
		ctx := context.Background()

		if exit := func() bool {
			if mt == websocket.TextMessage {
				var msg pluginstypes.Message
				if err := json.Unmarshal(inMsg, &msg); err != nil {
					//  TODO: handle error
					return true
				}

				switch msg.Type {
				case pluginstypes.CommandTypeRegisterPlugin:
					var registerData pluginstypes.RegisterPluginData
					if err := json.Unmarshal(msg.Data, &registerData); err != nil {
						//  TODO: handle error
						break
					}

					m.mu.Lock()
					m.channelByPluginID[registerData.PluginID] = c
					for _, extensionConfig := range registerData.Extensions {
						currentExtensionRuntimeInfos, ok := m.extensionRuntimeInfoByExtensionPointIDs[extensionConfig.ExtensionPointID]
						if !ok {
							currentExtensionRuntimeInfos = make([]extensionRuntimeInfo, 0)
						}
						currentExtensionRuntimeInfos = append(currentExtensionRuntimeInfos, extensionRuntimeInfo{
							conn:        c,
							connWaiters: connWaiters,
							cfg:         extensionConfig,
						})

						m.extensionRuntimeInfoByExtensionPointIDs[extensionConfig.ExtensionPointID] = currentExtensionRuntimeInfos
					}
					ch := c.CloseHandler()
					c.SetCloseHandler(func(code int, text string) error {
						connWaiters = m.processChannelClosing(connWaiters)
						return ch(code, text)
					})
					m.mu.Unlock()
					m.started(registerData.Secret)
				case pluginstypes.CommandTypeExecuteExtension:
					if msg.CorrelationID != "" {
						m.mu.Lock()
						func() bool {
							defer m.mu.Unlock()
							defer delete(m.waitersByRequestID, msg.CorrelationID)
							defer delete(connWaiters, msg.CorrelationID)
							waiter, ok := m.waitersByRequestID[msg.CorrelationID]
							if !ok {
								m.Failure(fmt.Errorf("unknown correlationID %s", msg.CorrelationID))
								return true
							}

							if msg.Error != nil {
								waiter.ch <- msg.Error
								return true
							}

							if err := json.Unmarshal(msg.Data, waiter.out); err != nil {
								waiter.ch <- err
								return true
							}
							waiter.ch <- waiter.out
							return false
						}()
					} else {
						go m.processExecuteExtensionRequest(ctx, msg, c)
					}
				}
			}
			return false
		}(); exit {
			break
		}
	}
}

func (m *WSManager) processChannelClosing(connWaiters map[string]*WaiterInfo) map[string]*WaiterInfo {
	var wis []*WaiterInfo
	m.mu.Lock()
	for _, wi := range connWaiters {
		wis = append(wis, wi)
	}
	connWaiters = make(map[string]*WaiterInfo)
	m.mu.Unlock()

	for _, wi := range wis {
		wi.ch <- fmt.Errorf("plugin failed before processing finished")
	}
	return connWaiters
}

func (m *WSManager) processExecuteExtensionRequest(ctx context.Context, msg pluginstypes.Message, c *websocket.Conn) {
	var executeExtensionData pluginstypes.ExecuteExtensionData
	if err := json.Unmarshal(msg.Data, &executeExtensionData); err != nil {
		if errWrite := m.sendErrorResponse(msg, err, c); errWrite != nil {
			m.Failure(errWrite)
		}
		return
	}
	results := ExecuteExtensions[json.RawMessage, json.RawMessage](
		ctx,
		m,
		executeExtensionData.ExtensionPointID,
		executeExtensionData.Data,
	)
	var lastResult *pluginstypes.Message
	for result := range results {
		if lastResult != nil {
			if errWrite := m.writeResponse(*lastResult, c); errWrite != nil {
				lastResult = nil
				m.Failure(errWrite)
				break
			}
			lastResult = nil
		}

		if result.Err != nil {
			msgResponse := pluginstypes.Message{
				CorrelationID: msg.MsgID,
				Type:          pluginstypes.CommandTypeExecuteExtension,
				Error: &pluginstypes.PluginError{
					Type:    fmt.Sprintf("%s::%T", "plugins", result.Err),
					Message: result.Err.Error(),
				},
				IsFinal: true,
			}
			if errWrite := m.writeResponse(msgResponse, c); errWrite != nil {
				m.Failure(errWrite)
			}
			break
		}

		dataBytes, err := json.Marshal(result.Out)
		if err != nil {
			if errWrite := m.sendErrorResponse(
				msg, fmt.Errorf("marshal output: %w", err), c); errWrite != nil {
				m.Failure(errWrite)
			}
			break
		}
		msgResponse := pluginstypes.Message{
			CorrelationID: msg.MsgID,
			Type:          pluginstypes.CommandTypeExecuteExtension,
			Data:          dataBytes,
			IsFinal:       false,
		}
		lastResult = &msgResponse
	}

	if lastResult != nil {
		lastResult.IsFinal = true
		if errWrite := m.writeResponse(*lastResult, c); errWrite != nil {
			m.Failure(errWrite)
			return
		}
	}
}

func (m *WSManager) sendErrorResponse(msg pluginstypes.Message, err error, c *websocket.Conn) error {
	msgResponse := pluginstypes.Message{
		CorrelationID: msg.MsgID,
		Type:          pluginstypes.CommandTypeExecuteExtension,
		Error: &pluginstypes.PluginError{
			Type:    fmt.Sprintf("%s::%T", "plugins", err),
			Message: err.Error(),
		},
		IsFinal: true,
	}
	errWrite := m.writeResponse(msgResponse, c)
	return errWrite
}

func (m *WSManager) writeMessage(c *websocket.Conn, messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return c.WriteMessage(messageType, data)
}

func (m *WSManager) writeResponse(msgResponse pluginstypes.Message, c *websocket.Conn) error {
	msgResponseBytes, err := json.Marshal(msgResponse)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if m.debug {
		m.logger.Info(
			"Write message",
			slog.String("localAddr", c.LocalAddr().String()),
			slog.String("remoteAddr", c.RemoteAddr().String()),
			slog.String("msg", string(msgResponseBytes)),
		)
	}
	if err := m.writeMessage(c, websocket.TextMessage, msgResponseBytes); err != nil {
		return fmt.Errorf("write message to channel: %w", err)
	}
	return nil
}

// ExecuteExtensions executes the extensions for the given extension point ID and input.
// It returns a channel that will receive the results of the execution.
// The channel will be closed when all the extensions have been executed or after first error returned.
func ExecuteExtensions[IN any, OUT any](ctx context.Context, m *WSManager, extensionPointID string, in IN) chan pluginstypes.ExecuteExtensionResult[OUT] {
	m.mu.Lock()
	extensionRuntimeInfos := m.extensionRuntimeInfoByExtensionPointIDs[extensionPointID]
	m.mu.Unlock()

	res := make(chan pluginstypes.ExecuteExtensionResult[OUT])
	go func() {
		for _, runtimeInfo := range extensionRuntimeInfos {
			if runtimeInfo.conn == nil {
				// host extension
				out, err := runtimeInfo.hostImplementation(ctx, in)
				res <- pluginstypes.ExecuteExtensionResult[OUT]{
					Out: out.(OUT),
					Err: err,
				}
				if err != nil {
					close(res)
					return
				}
				continue
			}

			inBytes, err := json.Marshal(in)
			if err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}

			msgID := uuid.NewString()
			msgData := pluginstypes.ExecuteExtensionData{
				ExtensionPointID: extensionPointID,
				ExtensionID:      runtimeInfo.cfg.ID,
				Data:             inBytes,
			}
			msgDataBytes, err := json.Marshal(msgData)
			if err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}

			sendMsg := &pluginstypes.Message{
				Type:    pluginstypes.CommandTypeExecuteExtension,
				MsgID:   msgID,
				Data:    msgDataBytes,
				IsFinal: true,
			}
			sendMsgBytes, err := json.Marshal(sendMsg)
			if err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}

			ch := make(chan any)
			m.mu.Lock()
			var out OUT
			newWaiterInfo := &WaiterInfo{
				ch:  ch,
				out: &out,
			}
			m.waitersByRequestID[msgID] = newWaiterInfo
			runtimeInfo.connWaiters[msgID] = newWaiterInfo
			m.mu.Unlock()
			if m.debug {
				m.logger.Info(
					"Write message",
					slog.String("localAddr", runtimeInfo.conn.LocalAddr().String()),
					slog.String("remoteAddr", runtimeInfo.conn.RemoteAddr().String()),
					slog.String("msg", string(sendMsgBytes)),
				)
			}
			if err := m.writeMessage(runtimeInfo.conn, websocket.TextMessage, sendMsgBytes); err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}
			o := <-ch
			if err, ok := o.(error); ok {
				sendErrorExecuteExtensionResult(res, err)
				return
			}
			oOut := o.(*OUT)
			res <- pluginstypes.ExecuteExtensionResult[OUT]{
				Out: *oOut,
				Err: nil,
			}
		}
		close(res)
	}()

	return res
}

func (m *WSManager) listen() error {
	var err error
	address := "127.0.0.1:"
	if m.pmsPort != 0 {
		address += strconv.Itoa(m.pmsPort)
	}
	m.lis, err = net.Listen("tcp", address)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	m.pmsPort = m.lis.Addr().(*net.TCPAddr).Port
	return nil
}

func (m *WSManager) startServer() error {
	mux := &http.ServeMux{}
	mux.HandleFunc("/", m.handle)
	return http.Serve(m.lis, mux)
}

type WSRegisterArgs struct {
	Secret   string
	HttpPort int
}

func (m *WSManager) started(secret string) *int {
	m.pluginRegistrationChannel <- secret
	i := 0
	return &i
}

// LoadPlugins loads the plugins specified by the given commands.
//
// The function starts a new goroutine for each plugin command, and
// waits for all plugins to finish loading before returning.
//
// If the context is canceled, the function returns an error.
//
// The function returns an error if any of the plugin commands
// fail to start.
func (m *WSManager) LoadPlugins(ctx context.Context, cmds ...string) error {
	var secrets []string

	for _, cmd := range cmds {
		pluginCommand := cmd
		go func() {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						m.managerErrorsChannel <- fmt.Errorf("can't start plugin, panic %s: %w", pluginCommand, err)
					} else {
						m.managerErrorsChannel <- fmt.Errorf("can't start plugin, panic %s: %v", pluginCommand, r)
					}
				}
			}()
			secret := random.GenerateRandomString(64)
			command := exec.Command(pluginCommand, "-pms-port", strconv.Itoa(m.pmsPort), "-pms-secret", secret)
			if m.debug {
				command.Stdout = os.Stdout
				command.Stderr = os.Stderr
			}
			if 1 == 1 {
				panic(fmt.Errorf("some unexpected error"))
			}
			if err := command.Start(); err != nil {
				m.managerErrorsChannel <- fmt.Errorf("can't start plugin %s: %w", pluginCommand, err)
			}
			m.mu.Lock()
			secrets = append(secrets, secret)
			m.mu.Unlock()
		}()
	}

	if len(cmds) == 0 {
		m.updateExtensionsOrder()
		return nil
	}

	return m.awaitPlugins(ctx, secrets)
}

func (m *WSManager) awaitPlugins(ctx context.Context, secrets []string) error {
	waitingSecrets := make(map[string]struct{})
	for _, secret := range secrets {
		waitingSecrets[secret] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("awaiting plugins initialization: %w", ctx.Err())
		case req := <-m.pluginRegistrationChannel:
			m.mu.Lock()
			delete(waitingSecrets, req)
			if len(waitingSecrets) == 0 {
				m.mu.Unlock()
				m.updateExtensionsOrder()
				m.mu.Lock()
				m.pluginsOrdered = true
				m.mu.Unlock()
				return nil
			}
			m.mu.Unlock()
		case err := <-m.managerErrorsChannel:
			return err
		}
	}
}

func (m *WSManager) updateExtensionsOrder() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// reorder extensions according to order
	for s, v := range m.extensionRuntimeInfoByExtensionPointIDs {
		prioritizedExtensionRuntimeInfos, err := OrderExtensionRuntimeInfo(v)
		if err != nil {
			m.mu.Unlock()
			m.Failure(err)

			// lock to unlock after in defer
			m.mu.Lock()
			break
		}
		m.extensionRuntimeInfoByExtensionPointIDs[s] = prioritizedExtensionRuntimeInfos
	}
}

func (m *WSManager) Failure(err error) {
	m.failureProcessor(err)
}

func (m *WSManager) DefaultFailureProcessor(err error) {
	m.managerErrorsChannel <- err
}

func sendErrorExecuteExtensionResult[OUT any](res chan pluginstypes.ExecuteExtensionResult[OUT], err error) {
	var o OUT
	res <- pluginstypes.ExecuteExtensionResult[OUT]{
		Out: o,
		Err: err,
	}
	close(res)
}
