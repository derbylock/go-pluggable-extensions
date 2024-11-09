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

var websocketServer *websocket.Client

// PluginContextID returns the plugin initialization secret.
func PluginContextID() string {
	return pluginSecret
}

// Extension registers an extension with the given configuration and implementation.
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

// Start starts the plugin with the given context and plugin ID.
func Start(ctx context.Context, pluginID string) error {
	pmsSecret := flag.String("pms-secret", "", "")
	pmsPort := flag.Int("pms-port", 0, "")
	flag.Parse()
	pluginSecret = *pmsSecret

	websocketServer = websocket.NewClient(pluginID, pluginSecret, *pmsPort, extensions)
	return websocketServer.Start()
}

// ExecuteExtensions executes the extensions with the given extension point ID and input.
func ExecuteExtensions[IN any, OUT any](ctx context.Context, extensionPointID string, in IN) chan types.ExecuteExtensionResult[OUT] {
	return websocket.ExecuteExtensions[IN, OUT](websocketServer, extensionPointID, in)
}
