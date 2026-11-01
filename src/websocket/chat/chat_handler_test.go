package chat

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/NARUBROWN/spine/pkg/ws"
)

type recordingSender struct {
	mu     sync.Mutex
	events []ChatEvent
}

func (s *recordingSender) Send(_ int, data []byte) error {
	var event ChatEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *recordingSender) last(t *testing.T) ChatEvent {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		t.Fatal("이벤트가 기록되지 않았습니다")
	}
	return s.events[len(s.events)-1]
}

func TestChatSocketRoomProtocol(t *testing.T) {
	controller := NewChatSocket()
	alice := &recordingSender{}
	bob := &recordingSender{}
	other := &recordingSender{}

	sendCommand(t, controller, "alice-connection", alice, ChatMessage{Type: "join", Room: "general", User: "alice"})
	sendCommand(t, controller, "bob-connection", bob, ChatMessage{Type: "join", Room: "general", User: "bob"})
	sendCommand(t, controller, "other-connection", other, ChatMessage{Type: "join", Room: "other", User: "carol"})
	sendCommand(t, controller, "alice-connection", alice, ChatMessage{Type: "message", Message: " hello "})

	if event := alice.last(t); event.Type != "message" || event.Room != "general" || event.From != "alice" || event.Message != "hello" {
		t.Fatalf("발신자 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := bob.last(t); event.Type != "message" || event.Message != "hello" {
		t.Fatalf("같은 방 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := other.last(t); event.Type != "joined" {
		t.Fatalf("다른 방으로 메시지가 전파되었습니다: %#v", event)
	}

	sendCommand(t, controller, "bob-connection", bob, ChatMessage{Type: "leave"})
	if event := bob.last(t); event.Type != "left" || event.From != "bob" {
		t.Fatalf("퇴장 확인 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := alice.last(t); event.Type != "left" || event.From != "bob" {
		t.Fatalf("방 퇴장 이벤트가 올바르지 않습니다: %#v", event)
	}
}

func TestChatSocketRejectsInvalidState(t *testing.T) {
	controller := NewChatSocket()
	first := &recordingSender{}
	second := &recordingSender{}

	sendCommand(t, controller, "first", first, ChatMessage{Type: "message", Message: "early"})
	assertCode(t, first.last(t), "NOT_JOINED")

	sendCommand(t, controller, "first", first, ChatMessage{Type: "join", Room: "general", User: "alice"})
	sendCommand(t, controller, "first", first, ChatMessage{Type: "join", Room: "other", User: "alice"})
	assertCode(t, first.last(t), "ALREADY_JOINED")

	sendCommand(t, controller, "second", second, ChatMessage{Type: "join", Room: "general", User: "alice"})
	assertCode(t, second.last(t), "USER_ALREADY_EXISTS")

	sendCommand(t, controller, "second", second, ChatMessage{Type: "join", Room: "bad room", User: "bob"})
	assertCode(t, second.last(t), "INVALID_ROOM")
}

func sendCommand(t *testing.T, controller *ChatSocket, connectionID string, sender ws.Sender, command ChatMessage) {
	t.Helper()
	ctx := context.WithValue(context.Background(), ws.SenderKey, sender)
	if err := controller.Handle(ctx, ws.ConnectionID{Value: connectionID}, command); err != nil {
		t.Fatalf("명령 처리 실패: %v", err)
	}
}

func assertCode(t *testing.T, event ChatEvent, expected string) {
	t.Helper()
	if event.Type != "error" || event.Code != expected {
		t.Fatalf("오류 코드가 올바르지 않습니다: %#v", event)
	}
}
