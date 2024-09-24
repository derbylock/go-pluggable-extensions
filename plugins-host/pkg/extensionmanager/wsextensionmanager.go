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

type WSManager struct {
	debug                                   bool
	lis                                     net.Listener
	pmsPort                                 int
	mu                                      *sync.Mutex
	pluginRegistrationChannel               chan string
	waiters                                 map[string]*WaiterInfo
	pluginIDBySecret                        map[string]string
	channelByPluginID                       map[string]*websocket.Conn
	extensionRuntimeInfoByExtensionPointIDs map[string][]extensionRuntimeInfo
}

func NewWSManager() *WSManager {
	return &WSManager{
		mu:                                      &sync.Mutex{},
		pluginRegistrationChannel:               make(chan string),
		waiters:                                 make(map[string]*WaiterInfo),
		pluginIDBySecret:                        make(map[string]string),
		channelByPluginID:                       make(map[string]*websocket.Conn),
		extensionRuntimeInfoByExtensionPointIDs: make(map[string][]extensionRuntimeInfo),
	}
}

func (m *WSManager) WithDebug() *WSManager {
	m.debug = true
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

					m.Started(registerData.Secret)
					m.mu.Lock()
					m.channelByPluginID[registerData.PluginID] = c
					for _, extensionConfig := range registerData.Extensions {
						currentChannels, ok := m.extensionRuntimeInfoByExtensionPointIDs[extensionConfig.ExtensionPointID]
						if !ok {
							currentChannels = make([]extensionRuntimeInfo, 0)
						}
						currentChannels = append(currentChannels, extensionRuntimeInfo{
							conn: c,
							cfg:  extensionConfig,
						})

						// reorder channels according to order

						m.extensionRuntimeInfoByExtensionPointIDs[extensionConfig.ExtensionPointID] = currentChannels
					}
					m.mu.Unlock()
				case plugins.CommandTypeExecuteExtension:
					m.mu.Lock()
					if exit := func() bool {
						defer m.mu.Unlock()
						defer delete(m.waiters, msg.CorrelationID)
						waiter, ok := m.waiters[msg.CorrelationID]
						if !ok {
							// TODO: handle error
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
				}
			}
			return false
		}(); exit {
			break
		}
		// log.Printf("recv: %s", message)
	}
}

type ExecuteExtensionResult[OUT any] struct {
	Out OUT
	Err error
}

func ExecuteExtension[IN any, OUT any](m *WSManager, extensionID string, in IN) chan ExecuteExtensionResult[OUT] {
	m.mu.Lock()
	extensionRuntimeInfos := m.extensionRuntimeInfoByExtensionPointIDs[extensionID]
	m.mu.Unlock()

	res := make(chan ExecuteExtensionResult[OUT])
	go func() {
		for _, runtimeInfo := range extensionRuntimeInfos {
			inBytes, err := json.Marshal(in)
			if err != nil {
				// TODO: process error
				var o OUT
				res <- ExecuteExtensionResult[OUT]{
					Out: o,
					Err: err,
				}
				close(res)
				return
			}

			msgID := uuid.NewString()
			msgData := plugins.ExecuteExtensionData{
				ExtensionID: extensionID,
				Data:        inBytes,
			}
			msgDataBytes, err := json.Marshal(msgData)
			if err != nil {
				// TODO: process error
				var o OUT
				res <- ExecuteExtensionResult[OUT]{
					Out: o,
					Err: err,
				}
				close(res)
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
				// TODO: process error
				var o OUT
				res <- ExecuteExtensionResult[OUT]{
					Out: o,
					Err: err,
				}
				close(res)
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
				// TODO: process error
				var o OUT
				res <- ExecuteExtensionResult[OUT]{
					Out: o,
					Err: err,
				}
				close(res)
				return
			}
			o := <-ch
			if err, ok := o.(error); ok {
				// TODO: process error
				var o OUT
				res <- ExecuteExtensionResult[OUT]{
					Out: o,
					Err: err,
				}
				close(res)
				return
			}
			oOut := o.(*OUT)
			res <- ExecuteExtensionResult[OUT]{
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
		}
	}
}
