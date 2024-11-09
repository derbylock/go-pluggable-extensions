// Here is an example of the plugin which provides 3 extensions for the "hello" extension point.
// These plugins has partial order described via the AfterExtensionIDs and BeforeExtensionIDs
//
// So, the output would be like:
//
//	Hello Anton from plugin A!
//	Current date is 2024-09-25T20:36:43+03:00.
//	Welcome to an ordered plugins world, Anton!
//
// The extensions order would be: "plugina.hello" -> "plugina.hello.currentDate" -> "plugina.hello.welcome".
package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins"
	pluginstypes "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
	"log"
	"time"
)

var cfg1 = pluginstypes.ExtensionConfig{
	ID:               "plugina.hello",
	ExtensionPointID: "hello",
}

var cfg2 = pluginstypes.ExtensionConfig{
	ID:                "plugina.hello.welcome",
	ExtensionPointID:  "hello",
	AfterExtensionIDs: []string{"plugina.hello"},
}

var cfg3 = pluginstypes.ExtensionConfig{
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
			Message: fmt.Sprintf(`Hello %s from the plugin A!`, in),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg2, func(ctx context.Context, in string) (HelloData, error) {
		randomNumber, err := getRandomNumber(ctx)
		if err != nil {
			return HelloData{}, err
		}

		return HelloData{
			Message: fmt.Sprintf(`Welcome to an ordered plugins world, %s! Random number is: %d`, in, randomNumber),
		}, nil
	})
	plugins.Extension[string, HelloData](cfg3, func(ctx context.Context, in string) (HelloData, error) {
		return HelloData{
			Message: fmt.Sprintf(`Current date is %s.`, time.Now().Format(time.RFC3339)),
		}, nil
	})

	plugins.Extension[string, int](pluginstypes.ExtensionConfig{
		ID:               "plugina.getRandomNumber.default",
		ExtensionPointID: "plugina.getRandomNumber",
	}, func(ctx context.Context, in string) (int, error) {
		return 4, nil
	})

	ctx := context.Background()
	if err := plugins.Start(ctx, pluginID); err != nil {
		log.Fatal(err)
	}
}

// getRandomNumber returns some "random number"
// it could be extended by Extensions for the "plugina.getRandomNumber" ExtensionPoint
// For demoe we declared two Extensions (plugina.getRandomNumber.default and app.getRandomNumber) which returns different numbers and joined using bitwise XOR
func getRandomNumber(ctx context.Context) (int, error) {
	extensionID := "plugina.getRandomNumber"
	ch := plugins.ExecuteExtensions[string, int](ctx, extensionID, "")

	// iterate over channel to retrieve all results provided by extensions
	randomNumber := 0
	for e := range ch {
		if e.Err != nil {
			return 0, fmt.Errorf("getRandomNumber failed: %w", e.Err)
		}
		randomNumber ^= e.Out
	}
	return randomNumber, nil
}

type HelloData struct {
	Message string `json:"message"`
}
