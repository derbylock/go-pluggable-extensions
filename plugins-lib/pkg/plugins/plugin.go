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

var extensions = make(map[string]ExtensionImplementation[any, any], 0)

var pluginSecret string

func PluginContextID() string {
	return pluginSecret
}

type ExtensionImplementation[IN any, OUT any] struct {
	Process     func(ctx context.Context, in IN) (OUT, error)
	Unmarshaler func(bytes []byte) (IN, error)
	Marshaller  func(out OUT) ([]byte, error)
}

func Extension[IN any, OUT any](extensionID string, implementation func(ctx context.Context, in IN) (OUT, error)) {
	extensions[extensionID] = ExtensionImplementation[any, any]{
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
	}
}

func Start(pluginID string) {
	pmsSecret := flag.String("pms-secret", "", "")
	pmsPort := flag.Int("pms-port", 0, "")
	flag.Parse()
	pluginSecret = *pmsSecret

	fmt.Println("1")

	serverAddr := fmt.Sprintf("127.0.0.1:%d", *pmsPort)

	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	fmt.Println("2")

	msgRegister := RegisterPluginMessage{
		Type: CommandTypeRegisterPlugin,
		Data: RegisterPluginData{
			PluginID:     pluginID,
			Secret:       pluginSecret,
			ExtensionIDs: []string{"hello"},
		},
		IsFinal: true,
	}
	msgRegisterBytes, _ := json.Marshal(msgRegister)
	// TODO: process error
	c.WriteMessage(websocket.TextMessage, msgRegisterBytes)
	// TODO: process error

	fmt.Println("3")
	for {
		_, msgBytes, err := c.ReadMessage()
		if err != nil {
			return
		}

		fmt.Println("4")
		ctx := context.Background()

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			// TODO: process error
		}

		fmt.Println("5")

		// TODO: process error

		// TODO: Process commands
		if msg.Type == CommandTypeExecuteExtension {
			fmt.Println("8")
			var executeExtensionData ExecuteExtensionData
			if err := json.Unmarshal(msg.Data, &executeExtensionData); err != nil {
				// TODO: process error
			}
			fmt.Println("9")
			fmt.Println(executeExtensionData.ExtensionID)
			if ext, ok := extensions[executeExtensionData.ExtensionID]; ok {
				in, err := ext.Unmarshaler(executeExtensionData.Data)
				if err != nil {
					// TODO: process error
				}
				fmt.Println("6")
				out, err := ext.Process(ctx, in)
				if err != nil {
					// TODO: process error
				}
				fmt.Println("7")
				outBytes, err := ext.Marshaller(out)
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
