package extensionmanager

import (
	"context"
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
			return implementation(ctx, in.(IN))
		},
	})

	m.extensionRuntimeInfoByExtensionPointIDs[cfg.ExtensionPointID] = currentExtensionRuntimeInfos
	m.mu.Unlock()
}
