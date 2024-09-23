package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
)

func main() {
	const pluginID = "plugin.A"
	plugins.Extension[string, HelloData]("hello", func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`"Hello %s from plugin A! %s"`, in, plugin.PluginContextID()),
		}, nil
	})
	plugins.Start(pluginID)
}

type HelloData struct {
	Message string `json:"message"`
}
