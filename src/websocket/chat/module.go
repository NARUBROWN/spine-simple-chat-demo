package chat

import (
	"github.com/NARUBROWN/spine"
)

// Register는 WebSocket 경로와 채팅 수신 메서드를 연결합니다.
func Register(app spine.App) error {
	return app.WebSocket().Register("/ws/chat", (*ChatSocket).Handle)
}
