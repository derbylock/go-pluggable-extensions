package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
	"time"
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
	BeforeExtensionIDs: []string{"plugina.hello.currentDate"},
	AfterExtensionIDs:  []string{"plugina.hello"},
}

var cfg3 = plugins.ExtensionConfig{
	ID:                 "plugina.hello.currentDate",
	ExtensionPointID:   "hello",
	BeforeExtensionIDs: []string{"plugina.hello.welcome"},
	AfterExtensionIDs:  []string{"plugina.hello", "plugina.init"},
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
			Message: fmt.Sprintf(`"Welcome to an ordered plugins world, %s!"`, in),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg3, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`"Current date is %s."`, time.Now().Format(time.RFC3339)),
		}, nil
	})
	plugins.Start(pluginID)
}

type HelloData struct {
	Message string `json:"message"`
}
