package chat

import "github.com/NARUBROWN/spine"

// Register는 생성자와 메시지 수신 계약을 애플리케이션에 등록합니다.
func Register(app spine.App) error {
	app.Constructor(NewChatSocket)
	if err := app.WebSocket().Register("/ws/chat", (*ChatSocket).Handle); err != nil {
		return err
	}
	return nil
}
