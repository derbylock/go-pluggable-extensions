package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"sync"
)

type WaiterInfo struct {
	ch  chan any
	out any
}

type Server struct {
	pluginID     string
	pluginSecret string
	pmsPort      int
	extensions   map[string]map[string]*pluginstypes.ExtensionRuntimeInfo
	channel      *websocket.Conn
	mu           *sync.Mutex
	waiters      map[string]*WaiterInfo
}

func NewServer(
	pluginID string,
	pluginSecret string,
	pmsPort int,
	extensions map[string]map[string]*pluginstypes.ExtensionRuntimeInfo,
) *Server {
	return &Server{
		pluginID:     pluginID,
		pluginSecret: pluginSecret,
		pmsPort:      pmsPort,
		extensions:   extensions,
		mu:           &sync.Mutex{},
		waiters:      make(map[string]*WaiterInfo),
	}
}

func (s *Server) Start() error {
	serverAddr := fmt.Sprintf("127.0.0.1:%d", s.pmsPort)

	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			panic(err)
		}
	}()
	s.channel = c

	implementedExtensions := make([]pluginstypes.ExtensionConfig, 0)
	for _, extensionInfos := range s.extensions {
		for _, info := range extensionInfos {
			implementedExtensions = append(implementedExtensions, info.Cfg())
		}
	}

	msgRegister := pluginstypes.RegisterPluginMessage{
		Type: pluginstypes.CommandTypeRegisterPlugin,
		Data: pluginstypes.RegisterPluginData{
			PluginID:   s.pluginID,
			Secret:     s.pluginSecret,
			Extensions: implementedExtensions,
		},
		IsFinal: true,
	}
	msgRegisterBytes, err := json.Marshal(msgRegister)
	if err != nil {
		return fmt.Errorf("marshal register message: %w", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, msgRegisterBytes); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}

	for {
		_, msgBytes, err := c.ReadMessage()
		if err != nil {
			e := fmt.Errorf("read message failed: %w", err)
			log.Fatal(e)
			return e
		}

		ctx := context.Background()

		var msg pluginstypes.Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			return s.sendPluginErrorResponse(msg, fmt.Errorf("unmarshal message: %w", err), c)
		}

		switch msg.Type {
		case pluginstypes.CommandTypeExecuteExtension:
			if msg.CorrelationID != "" {
				// plugin received invocation result
				if exit := func() bool {
					if msg.IsFinal {
						defer delete(s.waiters, msg.CorrelationID)
					}
					s.mu.Lock()
					waiter, ok := s.waiters[msg.CorrelationID]
					defer s.mu.Unlock()
					if !ok {
						log.Fatal(fmt.Errorf("unknown correlationID %s", msg.CorrelationID))
						return true
					}

					if msg.Error != nil {
						waiter.ch <- msg.Error
						return true
					}

					var outResult any
					if err := json.Unmarshal(msg.Data, outResult); err != nil {
						waiter.ch <- err
						return true
					}
					waiter.ch <- outResult
					if msg.IsFinal {
						close(waiter.ch)
					}
					return false
				}(); exit {
					break
				}
			} else {
				go func() {
					if err := s.processRequest(msg, c, ctx); err != nil {
						log.Fatal(err)
					}
				}()
			}
		}
	}
}

func (s *Server) processRequest(msg pluginstypes.Message, c *websocket.Conn, ctx context.Context) error {
	// plugin extension invoked
	var executeExtensionData pluginstypes.ExecuteExtensionData
	if err := json.Unmarshal(msg.Data, &executeExtensionData); err != nil {
		return s.sendPluginErrorResponse(msg, err, c)
	}
	if exts, ok := s.extensions[executeExtensionData.ExtensionPointID]; ok {
		if ext, ok := exts[executeExtensionData.ExtensionID]; ok {
			extension := *ext
			in, err := ext.Impl().Unmarshaler(executeExtensionData.Data)
			if err != nil {
				return s.sendExtensionErrorResponse(msg, extension, err, c)
			}
			out, err := ext.Impl().Process(ctx, in)
			if err != nil {
				return s.sendExtensionErrorResponse(msg, extension, err, c)
			}
			outBytes, err := ext.Impl().Marshaller(out)
			if err != nil {
				return s.sendExtensionErrorResponse(msg, extension, err, c)
			}

			msgResponse := pluginstypes.Message{
				CorrelationID: msg.MsgID,
				Type:          pluginstypes.CommandTypeExecuteExtension,
				Data:          outBytes,
				IsFinal:       true,
			}
			if errWrite := s.writeResponse(msgResponse, c); errWrite != nil {
				return errWrite
			}
		}
	}
	return nil
}

func (s *Server) sendPluginErrorResponse(msg pluginstypes.Message, err error, c *websocket.Conn) error {
	msgResponse := pluginstypes.Message{
		CorrelationID: msg.MsgID,
		Type:          pluginstypes.CommandTypeExecuteExtension,
		Error: &pluginstypes.PluginError{
			Type:    fmt.Sprintf("%s::%T", s.pluginID, err),
			Message: err.Error(),
		},
		IsFinal: true,
	}
	errWrite := s.writeResponse(msgResponse, c)
	return errWrite
}

func (s *Server) sendExtensionErrorResponse(msg pluginstypes.Message, ext pluginstypes.ExtensionRuntimeInfo, err error, c *websocket.Conn) error {
	msgResponse := pluginstypes.Message{
		CorrelationID: msg.MsgID,
		Type:          pluginstypes.CommandTypeExecuteExtension,
		Error: &pluginstypes.PluginError{
			Type:    fmt.Sprintf("%s::%s::%T", s.pluginID, ext.Cfg().ID, err),
			Message: err.Error(),
		},
		IsFinal: true,
	}
	errWrite := s.writeResponse(msgResponse, c)
	return errWrite
}

func (s *Server) writeResponse(msgResponse pluginstypes.Message, c *websocket.Conn) error {
	msgResponseBytes, err := json.Marshal(msgResponse)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, msgResponseBytes); err != nil {
		return fmt.Errorf("write message to channel: %w", err)
	}
	return nil
}

func ExecuteExtensions[IN any, OUT any](
	s *Server,
	extensionPointID string,
	in IN,
) chan pluginstypes.ExecuteExtensionResult[OUT] {
	res := make(chan pluginstypes.ExecuteExtensionResult[OUT])
	inBytes, err := json.Marshal(in)
	if err != nil {
		sendErrorExecuteExtensionResult(res, fmt.Errorf("marshal input: %w", err))
		return res
	}
	ch := make(chan any)
	go func() {
		msgID := uuid.NewString()
		msgData := pluginstypes.ExecuteExtensionData{
			ExtensionPointID: extensionPointID,
			Data:             inBytes,
		}
		msgDataBytes, err := json.Marshal(msgData)
		if err != nil {
			sendErrorExecuteExtensionResult(res, fmt.Errorf("marshal ExecuteExtensionData: %w", err))
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
			sendErrorExecuteExtensionResult(res, fmt.Errorf("marshal plugins.Message: %w", err))
			return
		}

		s.mu.Lock()
		var out OUT
		s.waiters[msgID] = &WaiterInfo{
			ch:  ch,
			out: &out,
		}
		s.mu.Unlock()

		if err := s.channel.WriteMessage(websocket.TextMessage, sendMsgBytes); err != nil {
			sendErrorExecuteExtensionResult(res, fmt.Errorf("write message: %w", err))
			delete(s.waiters, msgID)
			return
		}
	}()

	go func() {
		defer close(res)
		for o := range ch {
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
	}()

	return res
}

func sendErrorExecuteExtensionResult[OUT any](res chan pluginstypes.ExecuteExtensionResult[OUT], err error) {
	var o OUT
	res <- pluginstypes.ExecuteExtensionResult[OUT]{
		Out: o,
		Err: err,
	}
	close(res)
}
