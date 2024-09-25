// Here is an example of the plugin which provides 3 extensions for the "hello" extension point.
// These plugins has partial order described via the AfterExtensionIDs and BeforeExtensionIDs
//
// So, the output would be like:
//
//	Hello Anton from plugin A! 0wVcAFjHFaXaFDAzDFsj1fH4UyP7FQ15ohLAoE81eTzbSzzGjQMkxaUaRGS4fjV8
//	Current date is 2024-09-25T20:36:43+03:00.
//	Welcome to an ordered plugins world, Anton!
//
// The extensions order would be: "plugina.hello" -> "plugina.hello.currentDate" -> "plugina.hello.welcome".
package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
	"time"
)

var cfg1 = plugins.ExtensionConfig{
	ID:               "plugina.hello",
	ExtensionPointID: "hello",
}

var cfg2 = plugins.ExtensionConfig{
	ID:                "plugina.hello.welcome",
	ExtensionPointID:  "hello",
	AfterExtensionIDs: []string{"plugina.hello"},
}

var cfg3 = plugins.ExtensionConfig{
	ID:                 "plugina.hello.currentDate",
	ExtensionPointID:   "hello",
	BeforeExtensionIDs: []string{"plugina.hello.welcome"},
	AfterExtensionIDs: []string{
		"plugina.hello",
		"plugina.init", // we could refer not declared plugins as extensions are optional
	},
}

func main() {
	const pluginID = "plugin.A"
	plugins.Extension[string, HelloData](cfg1, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`Hello %s from plugin A! %s`, in, plugins.PluginContextID()),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg2, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`Welcome to an ordered plugins world, %s!`, in),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg3, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`Current date is %s.`, time.Now().Format(time.RFC3339)),
		}, nil
	})
	plugins.Start(pluginID)
}

type HelloData struct {
	Message string `json:"message"`
}
