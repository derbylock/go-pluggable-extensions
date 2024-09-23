package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/plugins-host/pkg/extensionmanager"
	"log"
)

func main() {
	ctx := context.Background()

	// init plugins manager
	pluginsManager := extensionmanager.NewWSManager().WithDebug().Init()

	// load required plugins
	pluginsNames := []string{"../plugina/plugina"}
	if err := pluginsManager.LoadPlugins(ctx, pluginsNames...); err != nil {
		log.Fatal(err)
	}

	// execute extension hello
	// it receives strings as input and returns the PrintHelloResponse struct as a result
	extensionID := "hello"
	ch := extensionmanager.ExecuteExtension[string, PrintHelloResponse](pluginsManager, extensionID, "Anton")

	// iterate over channel to retrieve results of all extensions
	for e := range ch {
		if e.Err != nil {
			panic(e.Err)
		}
		fmt.Println(e.Out.Message)
	}
}

type PrintHelloResponse struct {
	Message string `json:"message"`
}
