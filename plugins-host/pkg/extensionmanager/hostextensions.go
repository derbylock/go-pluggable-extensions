package extensionmanager

import (
	"context"
	"encoding/json"
	types "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
)

// Extension registers an extension with the WSManager.
//
// The extension is registered with the WSManager using the provided
// ExtensionConfig and implementation function. The implementation function
// is used to handle incoming requests for the extension.
//
// The ExtensionConfig contains information about the extension, such as its
// ID and the extension point ID it is registered with.
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

			var rawJson json.RawMessage
			rawJson, err := json.Marshal(o)
			return rawJson, err
		},
	})

	m.extensionRuntimeInfoByExtensionPointIDs[cfg.ExtensionPointID] = currentExtensionRuntimeInfos
	m.mu.Unlock()
}
