package chat

import (
	"encoding/json"

	"github.com/NARUBROWN/spine"

	"github.com/NARUBROWN/spine-simple-chat-demo/src/dto"
	"github.com/NARUBROWN/spine-simple-chat-demo/src/service"
	spinectx "github.com/NARUBROWN/spine/pkg/spine"
	"github.com/NARUBROWN/spine/pkg/ws"
)

// ChatSocket는 Spine이 해석한 입력과 연결을 방 서비스에 전달합니다.
type ChatSocket struct{ rooms *service.RoomService }

func (*ChatSocket) Constructor(rooms *service.RoomService) *ChatSocket {
	return &ChatSocket{rooms: rooms}
}

func (c *ChatSocket) Handle(ctx spinectx.Ctx, connID ws.ConnectionID, command dto.ChatMessage) error {
	executionCtx := ctx.ExecutionContext().Context()
	return c.rooms.Handle(executionCtx, connID.Value, func(event dto.ChatEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return ws.Send(executionCtx, ws.TextMessage, data)
	}, command)
}

// Register는 WebSocket 경로와 채팅 수신 메서드를 연결합니다.
func Register(app spine.App) error {
	return app.WebSocket().Register("/ws/chat", (*ChatSocket).Handle)
}
