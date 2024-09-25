package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
)

var cfg1 = plugins.ExtensionConfig{
	ID:                 "plugina.hello",
	ExtensionPointID:   "hello",
	BeforeExtensionIDs: nil,
	AfterExtensionIDs:  nil,
}

var cfg2 = plugins.ExtensionConfig{
	ID:                 "plugina.hello.welcome",
	ExtensionPointID:   "hello",
	BeforeExtensionIDs: nil,
	AfterExtensionIDs:  []string{"plugina.hello"},
}

func main() {
	const pluginID = "plugin.A"
	plugins.Extension[string, HelloData](cfg1, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`"Hello %s from plugin A! %s"`, in, plugins.PluginContextID()),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg2, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`"Welcome to ordered plugins world! %s"`, in, plugins.PluginContextID()),
		}, nil
	})
	plugins.Start(pluginID)
}

type HelloData struct {
	Message string `json:"message"`
}
