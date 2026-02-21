package main

import (
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

	routes.RegisterChatRoutes(app)

	app.Run(boot.Options{
		Address:                ":8080",
		EnableGracefulShutdown: true,
		ShutdownTimeout:        10 * time.Second,
		HTTP:                   &boot.HTTPOptions{},
	})
}
