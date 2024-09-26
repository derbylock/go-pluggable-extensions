package plugins

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/transport/websocket"
)

type ExecuteExtensionResult[OUT any] struct {
	Out OUT
	Err error
}

type ExtensionRuntimeInfo struct {
	cfg  ExtensionConfig
	impl ExtensionImplementation[any, any]
}

func (e *ExtensionRuntimeInfo) Cfg() ExtensionConfig {
	return e.cfg
}

func (e *ExtensionRuntimeInfo) Impl() ExtensionImplementation[any, any] {
	return e.impl
}

var extensions = make(map[string]map[string]ExtensionRuntimeInfo, 0)

var pluginSecret string

var websocketServer *websocket.Server

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

func Start(pluginID string) error {
	pmsSecret := flag.String("pms-secret", "", "")
	pmsPort := flag.Int("pms-port", 0, "")
	flag.Parse()
	pluginSecret = *pmsSecret

	websocketServer = websocket.NewServer(pluginID, pluginSecret, *pmsPort, extensions)
	return websocketServer.Start()
}

func ExecuteExtension[IN any, OUT any](extensionPointID string, in IN) chan ExecuteExtensionResult[OUT] {
	return websocket.ExecuteExtension[IN, OUT](websocketServer, extensionPointID, in)
}
