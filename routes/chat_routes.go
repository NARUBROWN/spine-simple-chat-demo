package routes

import (
	"github.com/NARUBROWN/spine"
	"github.com/NARUBROWN/spine-simple-chat-demo/controller"
)

func RegisterChatRoutes(app spine.App) error {
	return app.WebSocket().Register("/ws/chat", (*controller.ChatController).OnMessage)
}
