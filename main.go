package main

import (
	"log"
	"time"

	"github.com/NARUBROWN/spine"
	"github.com/NARUBROWN/spine-simple-chat-demo/controller"
	"github.com/NARUBROWN/spine-simple-chat-demo/routes"
	"github.com/NARUBROWN/spine/pkg/boot"
)

func main() {
	app := spine.New()

	app.Constructor(
		controller.NewChatController,
	)

	if err := routes.RegisterChatRoutes(app); err != nil {
		log.Fatalf("WebSocket route registration failed: %v", err)
	}

	if err := app.Run(boot.Options{
		Address:                ":8080",
		EnableGracefulShutdown: true,
		ShutdownTimeout:        10 * time.Second,
		HTTP: &boot.HTTPOptions{
			WebSocket: boot.WebSocketOptions{
				MaxConnections: 256,
			},
		},
	}); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
