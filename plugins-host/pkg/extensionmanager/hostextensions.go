package extensionmanager

import (
	"context"
	"encoding/json"
	types "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
)

func Extension[IN any, OUT any](m *WSManager, cfg types.ExtensionConfig, implementation func(ctx context.Context, in IN) (OUT, error)) {
	m.mu.Lock()
	currentExtensionRuntimeInfos, ok := m.extensionRuntimeInfoByExtensionPointIDs[cfg.ExtensionPointID]
	if !ok {
		currentExtensionRuntimeInfos = make([]extensionRuntimeInfo, 0)
	}
	currentExtensionRuntimeInfos = append(currentExtensionRuntimeInfos, extensionRuntimeInfo{
		conn: nil,
		cfg:  cfg,
		hostImplementation: func(ctx context.Context, in any) (any, error) {
			jsonInput := false
			var i IN
			if inBytes, ok := in.(json.RawMessage); ok {
				jsonInput = true
				// remote invocation
				if err := json.Unmarshal(inBytes, &i); err != nil {
					return nil, err
				}
			} else {
				// local invocation
				i = in.(IN)
			}

			o, e := implementation(ctx, i)
			if !jsonInput {
				return o, e
			}

			return json.Marshal(o)
		},
	})

	m.extensionRuntimeInfoByExtensionPointIDs[cfg.ExtensionPointID] = currentExtensionRuntimeInfos
	m.mu.Unlock()
}
