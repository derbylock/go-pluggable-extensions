package extensionmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-host/pkg/random"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
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
	conn *websocket.Conn
	cfg  plugins.ExtensionConfig
}

type failureProcessor func(err error)

type WSManager struct {
	debug                                   bool
	failureProcessor                        failureProcessor
	lis                                     net.Listener
	pmsPort                                 int
	mu                                      *sync.Mutex
	pluginRegistrationChannel               chan string
	managerErrorsChannel                    chan error
	waiters                                 map[string]*WaiterInfo
	pluginIDBySecret                        map[string]string
	channelByPluginID                       map[string]*websocket.Conn
	extensionRuntimeInfoByExtensionPointIDs map[string][]extensionRuntimeInfo
}

func NewWSManager() *WSManager {
	m := &WSManager{
		mu:                                      &sync.Mutex{},
		pluginRegistrationChannel:               make(chan string),
		managerErrorsChannel:                    make(chan error),
		waiters:                                 make(map[string]*WaiterInfo),
		pluginIDBySecret:                        make(map[string]string),
		channelByPluginID:                       make(map[string]*websocket.Conn),
		extensionRuntimeInfoByExtensionPointIDs: make(map[string][]extensionRuntimeInfo),
	}

	return m.WithFailureProcessor(m.DefaultFailureProcessor)
}

func (m *WSManager) WithDebug() *WSManager {
	m.debug = true
	return m
}

func (m *WSManager) WithFailureProcessor(p failureProcessor) *WSManager {
	m.failureProcessor = p
	return m
}

func (m *WSManager) Init() (*WSManager, error) {
	err := m.Listen()
	if err != nil {
		return m, err
	}
	go func() {
		err := m.StartServer()
		if err != nil {
			panic(fmt.Errorf("init plugins manager: %w", err))
		}
	}()
	return m, nil
}

func (m *WSManager) Handle(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, inMsg, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		if exit := func() bool {
			if mt == websocket.TextMessage {
				var msg plugins.Message
				if err := json.Unmarshal(inMsg, &msg); err != nil {
					//  TODO: handle error
					return true
				}

				switch msg.Type {
				case plugins.CommandTypeRegisterPlugin:
					var registerData plugins.RegisterPluginData
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
							conn: c,
							cfg:  extensionConfig,
						})

						// reorder channels according to order
						prioritizedExtensionRuntimeInfos, err := OrderExtensionRuntimeInfo(currentExtensionRuntimeInfos)
						if err != nil {
							m.mu.Unlock()
							m.Failure(err)

							// lock to unlock after
							m.mu.Lock()
							break
						}

						m.extensionRuntimeInfoByExtensionPointIDs[extensionConfig.ExtensionPointID] = prioritizedExtensionRuntimeInfos
					}
					m.mu.Unlock()
					m.Started(registerData.Secret)
				case plugins.CommandTypeExecuteExtension:
					if msg.CorrelationID != "" {
						m.mu.Lock()
						if exit := func() bool {
							defer m.mu.Unlock()
							defer delete(m.waiters, msg.CorrelationID)
							waiter, ok := m.waiters[msg.CorrelationID]
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
						}(); exit {
							break
						}
					} else {
						var executeExtensionData plugins.ExecuteExtensionData
						if err := json.Unmarshal(msg.Data, &executeExtensionData); err != nil {
							if errWrite := m.sendErrorResponse(msg, err, c); errWrite != nil {
								m.Failure(errWrite)
							}
							break
						}
						results := ExecuteExtension[json.RawMessage, json.RawMessage](
							m,
							executeExtensionData.ExtensionPointID,
							executeExtensionData.Data,
						)
						var lastResult *plugins.Message
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
								msgResponse := plugins.Message{
									CorrelationID: msg.MsgID,
									Type:          plugins.CommandTypeExecuteExtension,
									Error: &plugins.PluginError{
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
							msgResponse := plugins.Message{
								CorrelationID: msg.MsgID,
								Type:          plugins.CommandTypeExecuteExtension,
								Data:          dataBytes,
								IsFinal:       true,
							}
							lastResult = &msgResponse
						}

						if lastResult != nil {
							if errWrite := m.writeResponse(*lastResult, c); errWrite != nil {
								m.Failure(errWrite)
								break
							}
						}
					}
				}
			}
			return false
		}(); exit {
			break
		}
	}
}

func (w *WSManager) sendErrorResponse(msg plugins.Message, err error, c *websocket.Conn) error {
	msgResponse := plugins.Message{
		CorrelationID: msg.MsgID,
		Type:          plugins.CommandTypeExecuteExtension,
		Error: &plugins.PluginError{
			Type:    fmt.Sprintf("%s::%T", "plugins", err),
			Message: err.Error(),
		},
		IsFinal: true,
	}
	errWrite := w.writeResponse(msgResponse, c)
	return errWrite
}

func (w *WSManager) writeResponse(msgResponse plugins.Message, c *websocket.Conn) error {
	msgResponseBytes, err := json.Marshal(msgResponse)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, msgResponseBytes); err != nil {
		return fmt.Errorf("write message to channel: %w", err)
	}
	return nil
}

func ExecuteExtension[IN any, OUT any](m *WSManager, extensionPointID string, in IN) chan plugins.ExecuteExtensionResult[OUT] {
	m.mu.Lock()
	extensionRuntimeInfos := m.extensionRuntimeInfoByExtensionPointIDs[extensionPointID]
	m.mu.Unlock()

	res := make(chan plugins.ExecuteExtensionResult[OUT])
	go func() {
		for _, runtimeInfo := range extensionRuntimeInfos {
			inBytes, err := json.Marshal(in)
			if err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}

			msgID := uuid.NewString()
			msgData := plugins.ExecuteExtensionData{
				ExtensionPointID: extensionPointID,
				ExtensionID:      runtimeInfo.cfg.ID,
				Data:             inBytes,
			}
			msgDataBytes, err := json.Marshal(msgData)
			if err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}

			sendMsg := &plugins.Message{
				Type:    plugins.CommandTypeExecuteExtension,
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
			m.waiters[msgID] = &WaiterInfo{
				ch:  ch,
				out: &out,
			}
			m.mu.Unlock()
			if err := runtimeInfo.conn.WriteMessage(websocket.TextMessage, sendMsgBytes); err != nil {
				sendErrorExecuteExtensionResult(res, err)
				return
			}
			o := <-ch
			if err, ok := o.(error); ok {
				sendErrorExecuteExtensionResult(res, err)
				return
			}
			oOut := o.(*OUT)
			res <- plugins.ExecuteExtensionResult[OUT]{
				Out: *oOut,
				Err: nil,
			}
		}
		close(res)
	}()

	return res
}

func (m *WSManager) Listen() error {
	var err error
	m.lis, err = net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		log.Fatal("listen error:", err)
	}
	m.pmsPort = m.lis.Addr().(*net.TCPAddr).Port
	return nil
}

func (m *WSManager) StartServer() error {
	http.HandleFunc("/", m.Handle)
	return http.Serve(m.lis, nil)
}

type WSRegisterArgs struct {
	Secret   string
	HttpPort int
}

func (m *WSManager) Started(secret string) *int {
	m.pluginRegistrationChannel <- secret
	i := 0
	return &i
}

func (m *WSManager) LoadPlugins(ctx context.Context, cmds ...string) error {
	var secrets []string

	for _, cmd := range cmds {
		go func() error {
			secret := random.GenerateRandomString(64)
			command := exec.Command(cmd, "-pms-port", strconv.Itoa(m.pmsPort), "-pms-secret", secret)
			if m.debug {
				command.Stdout = os.Stdout
				command.Stderr = os.Stderr
			}
			if err := command.Start(); err != nil {
				// TODO send via channel
				return fmt.Errorf("can't start plugin %s: %w", cmd, err)
			}
			m.mu.Lock()
			secrets = append(secrets, secret)
			m.mu.Unlock()
			return nil
		}()
	}

	return m.AwaitPlugins(ctx, secrets)
}

func (m *WSManager) AwaitPlugins(ctx context.Context, secrets []string) error {
	waitingSecrets := make(map[string]struct{})
	for _, secret := range secrets {
		waitingSecrets[secret] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("awaiting plugins inititialization: %w", ctx.Err())
		case req := <-m.pluginRegistrationChannel:
			m.mu.Lock()
			delete(waitingSecrets, req)
			if len(waitingSecrets) == 0 {
				m.mu.Unlock()
				return nil
			}
			m.mu.Unlock()
		case err := <-m.managerErrorsChannel:
			m.mu.Lock()
			return err
			m.mu.Unlock()
		}
	}
}

func (m *WSManager) Failure(err error) {
	m.failureProcessor(err)
}

func (m *WSManager) DefaultFailureProcessor(err error) {
	m.managerErrorsChannel <- err
}

func sendErrorExecuteExtensionResult[OUT any](res chan plugins.ExecuteExtensionResult[OUT], err error) {
	var o OUT
	res <- plugins.ExecuteExtensionResult[OUT]{
		Out: o,
		Err: err,
	}
	close(res)
}
