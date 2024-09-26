package plugins

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/transport/websocket"
	types "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
)

var extensions = make(map[string]map[string]*types.ExtensionRuntimeInfo, 0)

var pluginSecret string

var websocketServer *websocket.Server

func PluginContextID() string {
	return pluginSecret
}

func Extension[IN any, OUT any](cfg types.ExtensionConfig, implementation func(ctx context.Context, in IN) (OUT, error)) {
	currentExtensions, ok := extensions[cfg.ExtensionPointID]
	if !ok {
		currentExtensions = make(map[string]*types.ExtensionRuntimeInfo)
	}

	extensions[cfg.ExtensionPointID] = currentExtensions
	currentExtensions[cfg.ID] = types.NewExtensionRuntimeInfo(
		cfg,
		types.ExtensionImplementation[any, any]{
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
		})
}

func Start(pluginID string) error {
	pmsSecret := flag.String("pms-secret", "", "")
	pmsPort := flag.Int("pms-port", 0, "")
	flag.Parse()
	pluginSecret = *pmsSecret

	websocketServer = websocket.NewServer(pluginID, pluginSecret, *pmsPort, extensions)
	return websocketServer.Start()
}

func ExecuteExtensions[IN any, OUT any](extensionPointID string, in IN) chan types.ExecuteExtensionResult[OUT] {
	return websocket.ExecuteExtensions[IN, OUT](websocketServer, extensionPointID, in)
}
