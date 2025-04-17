package extensionmanager

import (
	"context"
	pluginstypes "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
	"testing"
)

func TestLoadingOrderingError(t *testing.T) {
	ctx := context.Background()

	// init plugins manager
	pluginsManager, err := NewWSManager().Init()
	if err != nil {
		panic(err)
	}

	// declare host extensions before loading plugins
	Extension[string, int](pluginsManager, pluginstypes.ExtensionConfig{
		ID:               "app.getRandomNumber.default",
		ExtensionPointID: "qwe",
	}, func(ctx context.Context, in string) (int, error) {
		return 6, nil
	})

	Extension[string, int](pluginsManager, pluginstypes.ExtensionConfig{
		ID:               "app.getRandomNumber.default",
		ExtensionPointID: "qwe",
	}, func(ctx context.Context, in string) (int, error) {
		return 6, nil
	})

	err = pluginsManager.LoadPlugins(ctx)
	if err == nil {
		t.Logf("error should be returned")
	}
}
