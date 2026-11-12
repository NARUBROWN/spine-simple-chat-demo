package chat

import (
	"context"
	"encoding/json"

	"github.com/NARUBROWN/spine-simple-chat-demo/src/dto"
	"github.com/NARUBROWN/spine-simple-chat-demo/src/service"
	"github.com/NARUBROWN/spine/pkg/ws"
)

// ChatSocket는 Spine이 해석한 입력과 연결을 방 서비스에 전달합니다.
type ChatSocket struct{ rooms *service.RoomService }

func NewChatSocket(rooms *service.RoomService) *ChatSocket { return &ChatSocket{rooms: rooms} }

func (c *ChatSocket) Handle(ctx context.Context, connID ws.ConnectionID, command dto.ChatMessage) error {
	return c.rooms.Handle(ctx, connID.Value, func(event dto.ChatEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return ws.Send(ctx, ws.TextMessage, data)
	}, command)
}
