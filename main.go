package main

import (
	"log"

	"github.com/NARUBROWN/spine"
	"github.com/NARUBROWN/spine-simple-chat-demo/config"
	"github.com/NARUBROWN/spine-simple-chat-demo/controller"
	"github.com/NARUBROWN/spine-simple-chat-demo/interceptor"
	"github.com/NARUBROWN/spine-simple-chat-demo/routes"
	"github.com/NARUBROWN/spine/pkg/boot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration failed: %v", err)
	}

	app := spine.New()

	app.Constructor(
		controller.NewChatController,
	)
	app.InterceptorFor(boot.InterceptorWebSocket, interceptor.NewAuthInterceptor(cfg.AuthToken))

	if err := routes.RegisterChatRoutes(app); err != nil {
		log.Fatalf("WebSocket route registration failed: %v", err)
	}

	options := boot.Options{
		Address:                cfg.Address,
		EnableGracefulShutdown: true,
		ShutdownTimeout:        cfg.ShutdownTimeout,
		HTTP: &boot.HTTPOptions{
			WebSocket: cfg.WebSocket,
		},
	}
	if err := app.Validate(options); err != nil {
		log.Fatalf("Application validation failed: %v", err)
	}
	if err := app.Run(options); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
