package plugins

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
)

type ExtensionRuntimeInfo struct {
	cfg  ExtensionConfig
	impl ExtensionImplementation[any, any]
}

var extensions = make(map[string]map[string]ExtensionRuntimeInfo, 0)

var pluginSecret string

func PluginContextID() string {
	return pluginSecret
}

type ExtensionImplementation[IN any, OUT any] struct {
	Process     func(ctx context.Context, in IN) (OUT, error)
	Unmarshaler func(bytes []byte) (IN, error)
	Marshaller  func(out OUT) ([]byte, error)
}

func Extension[IN any, OUT any](cfg ExtensionConfig, implementation func(ctx context.Context, in IN) (OUT, error)) {
	currentExtensions, ok := extensions[cfg.ExtensionPointID]
	if !ok {
		currentExtensions = make(map[string]ExtensionRuntimeInfo)
	}

	extensions[cfg.ExtensionPointID] = currentExtensions
	currentExtensions[cfg.ID] = ExtensionRuntimeInfo{
		cfg: cfg,
		impl: ExtensionImplementation[any, any]{
			Process: func(ctx context.Context, in any) (any, error) {
				inTyped := in.(IN)
				out, err := implementation(ctx, inTyped)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
			Unmarshaler: func(bytes []byte) (any, error) {
				var in IN
				err := json.Unmarshal(bytes, &in)
				return in, err
			},
			Marshaller: func(out any) ([]byte, error) {
				bytes, err := json.Marshal(out)
				return bytes, err
			},
		},
	}
}

func Start(pluginID string) {
	pmsSecret := flag.String("pms-secret", "", "")
	pmsPort := flag.Int("pms-port", 0, "")
	flag.Parse()
	pluginSecret = *pmsSecret

	serverAddr := fmt.Sprintf("127.0.0.1:%d", *pmsPort)

	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	implementedExtensions := make([]ExtensionConfig, 0)
	for _, extensionInfos := range extensions {
		for _, info := range extensionInfos {
			implementedExtensions = append(implementedExtensions, info.cfg)
		}
	}

	msgRegister := RegisterPluginMessage{
		Type: CommandTypeRegisterPlugin,
		Data: RegisterPluginData{
			PluginID:   pluginID,
			Secret:     pluginSecret,
			Extensions: implementedExtensions,
		},
		IsFinal: true,
	}
	msgRegisterBytes, _ := json.Marshal(msgRegister)
	// TODO: process error
	c.WriteMessage(websocket.TextMessage, msgRegisterBytes)
	// TODO: process error

	for {
		_, msgBytes, err := c.ReadMessage()
		if err != nil {
			return
		}

		ctx := context.Background()

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			// TODO: process error
		}

		// TODO: process error

		// TODO: Process commands
		if msg.Type == CommandTypeExecuteExtension {
			var executeExtensionData ExecuteExtensionData
			if err := json.Unmarshal(msg.Data, &executeExtensionData); err != nil {
				// TODO: process error
			}
			if exts, ok := extensions[executeExtensionData.ExtensionPointID]; ok {
				if ext, ok := exts[executeExtensionData.ExtensionID]; ok {
					in, err := ext.impl.Unmarshaler(executeExtensionData.Data)
					if err != nil {
						// TODO: process error
					}
					out, err := ext.impl.Process(ctx, in)
					if err != nil {
						// TODO: process error
					}
					outBytes, err := ext.impl.Marshaller(out)
					if err != nil {
						// TODO: process error
					}

					msgResponse := Message{
						CorrelationID: msg.MsgID,
						Type:          CommandTypeExecuteExtension,
						Data:          outBytes,
						IsFinal:       true,
					}
					msgResponseBytes, err := json.Marshal(msgResponse)
					if err != nil {
						log.Fatal(err)
					}
					// TODO: process error
					c.WriteMessage(websocket.TextMessage, msgResponseBytes)
				}
			}
		}
	}

}
