package main

import (
	"log"

	"github.com/NARUBROWN/spine-simple-chat-demo/internal/application"
)

func main() {
	app, options, err := application.BuildApplication()
	if err != nil {
		log.Fatalf("Configuration failed: %v", err)
	}
	if err := app.Validate(options); err != nil {
		log.Fatalf("Application validation failed: %v", err)
	}
	if err := app.Run(options); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
