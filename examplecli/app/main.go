package main

import (
	"context"
	"fmt"
	"github.com/derbylock/go-pluggable-extensions/examplecli/app/pkg/extensionmanager"
	"log"
	"time"
)

func main() {
	now := time.Now()

	ctx := context.Background()

	pman := extensionmanager.NewWSManager()
	pman.Debug(false)
	pman.Listen()
	go pman.StartServer()

	for i := 0; i < 100; i++ {
		if err := pman.LoadPlugins(ctx, "../plugina/plugina"); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(time.Since(now))

	ch := extensionmanager.ExecuteExtension[string, PrintHelloResponse](pman, "hello", "Anton")
	for e := range ch {
		if e.Err != nil {
			panic(e.Err)
		}
		fmt.Println(e.Out.Message)
	}
	fmt.Println(time.Since(now))
}

type PrintHelloResponse struct {
	Message string `json:"message"`
}
