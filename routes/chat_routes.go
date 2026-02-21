package routes

import (
	"github.com/NARUBROWN/spine"
	"github.com/NARUBROWN/spine-simple-chat-demo/controller"
)

func RegisterChatRoutes(app spine.App) {
	app.WebSocket().Register("/ws/chat", (*controller.ChatController).OnMessage)
}
