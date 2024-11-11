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
	out func() any
}

type Client struct {
	pluginID     string
	pluginSecret string
	pmsPort      int
	extensions   map[string]map[string]*pluginstypes.ExtensionRuntimeInfo
	channel      *websocket.Conn
	mu           *sync.Mutex
	waiters      map[string]*WaiterInfo
}

func NewClient(
	pluginID string,
	pluginSecret string,
	pmsPort int,
	extensions map[string]map[string]*pluginstypes.ExtensionRuntimeInfo,
) *Client {
	return &Client{
		pluginID:     pluginID,
		pluginSecret: pluginSecret,
		pmsPort:      pmsPort,
		extensions:   extensions,
		mu:           &sync.Mutex{},
		waiters:      make(map[string]*WaiterInfo),
	}
}

func (s *Client) Start() error {
	c, err := s.initConnection()
	if err != nil {
		return fmt.Errorf("initConnection: %w", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			panic(err)
		}
	}()

	if err := s.registerPlugin(c); err != nil {
		return err
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
				if err := s.processExecutionResultMessage(msg); err != nil {
					log.Fatal(err)
				}
			} else {
				// plugin received invocation request
				go func() {
					if err := s.processRequest(msg, c, ctx); err != nil {
						log.Fatal(err)
					}
				}()
			}
		}
	}
}

func (s *Client) initConnection() (*websocket.Conn, error) {
	serverAddr := fmt.Sprintf("127.0.0.1:%d", s.pmsPort)

	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	s.channel = c
	return c, err
}

func (s *Client) writeMessage(c *websocket.Conn, messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return c.WriteMessage(messageType, data)
}

func (s *Client) registerPlugin(c *websocket.Conn) error {
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
	if err := s.writeMessage(c, websocket.TextMessage, msgRegisterBytes); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}
	return nil
}

func (s *Client) processRequest(msg pluginstypes.Message, c *websocket.Conn, ctx context.Context) error {
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

func (s *Client) processExecutionResultMessage(msg pluginstypes.Message) error {
	if msg.IsFinal {
		defer delete(s.waiters, msg.CorrelationID)
	}
	s.mu.Lock()
	waiter, ok := s.waiters[msg.CorrelationID]
	defer s.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown correlationID %s", msg.CorrelationID)
	}

	if msg.Error != nil {
		waiter.ch <- msg.Error
		return msg.Error
	}

	outResult := waiter.out()
	if err := json.Unmarshal(msg.Data, outResult); err != nil {
		waiter.ch <- err
		return err
	}
	waiter.ch <- outResult
	if msg.IsFinal {
		close(waiter.ch)
	}
	return nil
}

func (s *Client) sendPluginErrorResponse(msg pluginstypes.Message, err error, c *websocket.Conn) error {
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

func (s *Client) sendExtensionErrorResponse(msg pluginstypes.Message, ext pluginstypes.ExtensionRuntimeInfo, err error, c *websocket.Conn) error {
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

func (s *Client) writeResponse(msgResponse pluginstypes.Message, c *websocket.Conn) error {
	msgResponseBytes, err := json.Marshal(msgResponse)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if err := s.writeMessage(c, websocket.TextMessage, msgResponseBytes); err != nil {
		return fmt.Errorf("write message to channel: %w", err)
	}
	return nil
}

func ExecuteExtensions[IN any, OUT any](
	s *Client,
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
			ch <- fmt.Errorf("marshal ExecuteExtensionData: %w", err)
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
			ch <- fmt.Errorf("marshal plugins.Message: %w", err)
			return
		}

		s.mu.Lock()
		s.waiters[msgID] = &WaiterInfo{
			ch: ch,
			out: func() any {
				var out OUT
				return &out
			},
		}
		s.mu.Unlock()

		if err := s.writeMessage(s.channel, websocket.TextMessage, sendMsgBytes); err != nil {
			ch <- fmt.Errorf("write message: %w", err)
			delete(s.waiters, msgID)
			return
		}
	}()

	go func() {
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
		close(res)
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
