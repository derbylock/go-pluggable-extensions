package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-host/pkg/extensionmanager"
	pluginstypes "github.com/derbylock/go-pluggable-extensions/plugins-lib/pkg/plugins/types"
	"log"
)

const getRandomNumberExtensionPointID = "plugina.getRandomNumber"

func main() {
	ctx := context.Background()

	// init plugins manager
	pluginsManager, err := extensionmanager.NewWSManager().Init()
	if err != nil {
		panic(err)
	}

	// declare host extensions before loading plugins
	extensionmanager.Extension[string, int](pluginsManager, pluginstypes.ExtensionConfig{
		ID:               "app.getRandomNumber.default",
		ExtensionPointID: getRandomNumberExtensionPointID,
	}, func(ctx context.Context, in string) (int, error) {
		return 6, nil
	})

	// load required plugins
	pluginsNames := []string{
		"../plugina/plugina",
	}
	if err := pluginsManager.LoadPlugins(ctx, pluginsNames...); err != nil {
		log.Fatal(fmt.Errorf("plugins loading failed: %w", err))
	}

	// execute extension hello
	// it receives string as an input and returns the PrintHelloResponse struct as a result
	extensionID := "hello"
	ch := extensionmanager.ExecuteExtensions[string, PrintHelloResponse](ctx, pluginsManager, extensionID, "Anton")

	// iterate over channel to retrieve all results provided by extensions
	for e := range ch {
		if e.Err != nil {
			panic(e.Err)
		}
		fmt.Println(e.Out.Message)
	}

	n, err := getRandomNumber(ctx, pluginsManager)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Host executed random number is: %d\n", n)
}

func getRandomNumber(ctx context.Context, pluginsManager *extensionmanager.WSManager) (int, error) {
	extensionID := "plugina.getRandomNumber"
	ch := extensionmanager.ExecuteExtensions[string, int](ctx, pluginsManager, extensionID, "")

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

type PrintHelloResponse struct {
	Message string `json:"message"`
}
