package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NARUBROWN/spine-simple-chat-demo/src/dto"
)

type recordingSender struct {
	mu     sync.Mutex
	events []dto.ChatEvent
}

func (s *recordingSender) deliver(event dto.ChatEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *recordingSender) last(t *testing.T) dto.ChatEvent {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		t.Fatal("이벤트가 기록되지 않았습니다")
	}
	return s.events[len(s.events)-1]
}

func TestRoomServiceRoomProtocol(t *testing.T) {
	controller := (&RoomService{}).Constructor()
	alice := &recordingSender{}
	bob := &recordingSender{}
	other := &recordingSender{}

	sendCommand(t, controller, "alice-connection", alice, dto.ChatMessage{Type: "join", Room: "general", User: "alice"})
	sendCommand(t, controller, "bob-connection", bob, dto.ChatMessage{Type: "join", Room: "general", User: "bob"})
	sendCommand(t, controller, "other-connection", other, dto.ChatMessage{Type: "join", Room: "other", User: "carol"})
	sendCommand(t, controller, "alice-connection", alice, dto.ChatMessage{Type: "message", Message: " hello "})

	if event := alice.last(t); event.Type != "message" || event.Room != "general" || event.From != "alice" || event.Message != "hello" {
		t.Fatalf("발신자 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := bob.last(t); event.Type != "message" || event.Message != "hello" {
		t.Fatalf("같은 방 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := other.last(t); event.Type != "joined" {
		t.Fatalf("다른 방으로 메시지가 전파되었습니다: %#v", event)
	}

	sendCommand(t, controller, "bob-connection", bob, dto.ChatMessage{Type: "leave"})
	if event := bob.last(t); event.Type != "left" || event.From != "bob" {
		t.Fatalf("퇴장 확인 이벤트가 올바르지 않습니다: %#v", event)
	}
	if event := alice.last(t); event.Type != "left" || event.From != "bob" {
		t.Fatalf("방 퇴장 이벤트가 올바르지 않습니다: %#v", event)
	}
}

func TestRoomServiceRejectsInvalidState(t *testing.T) {
	controller := (&RoomService{}).Constructor()
	first := &recordingSender{}
	second := &recordingSender{}

	sendCommand(t, controller, "first", first, dto.ChatMessage{Type: "message", Message: "early"})
	assertCode(t, first.last(t), "NOT_JOINED")

	sendCommand(t, controller, "first", first, dto.ChatMessage{Type: "join", Room: "general", User: "alice"})
	sendCommand(t, controller, "first", first, dto.ChatMessage{Type: "join", Room: "other", User: "alice"})
	assertCode(t, first.last(t), "ALREADY_JOINED")

	sendCommand(t, controller, "second", second, dto.ChatMessage{Type: "join", Room: "general", User: "alice"})
	assertCode(t, second.last(t), "USER_ALREADY_EXISTS")

	sendCommand(t, controller, "second", second, dto.ChatMessage{Type: "join", Room: "bad room", User: "bob"})
	assertCode(t, second.last(t), "INVALID_ROOM")
}

func TestDisconnectRemovesMembershipAndAllowsNameReuse(t *testing.T) {
	controller := (&RoomService{}).Constructor()
	sender := &recordingSender{}
	connectionCtx, cancel := context.WithCancel(context.Background())
	ctx := connectionCtx
	if err := controller.Handle(ctx, "first", sender.deliver, dto.ChatMessage{Type: "join", Room: "general", User: "alice"}); err != nil {
		t.Fatal(err)
	}
	cancel()

	deadline := time.Now().Add(time.Second)
	for {
		controller.mu.RLock()
		_, exists := controller.clients["first"]
		controller.mu.RUnlock()
		if !exists {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("연결 취소 후 방 상태가 정리되지 않았습니다")
		}
		time.Sleep(time.Millisecond)
	}

	replacement := &recordingSender{}
	sendCommand(t, controller, "second", replacement, dto.ChatMessage{Type: "join", Room: "general", User: "alice"})
	if event := replacement.last(t); event.Type != "joined" || event.From != "alice" {
		t.Fatalf("연결 종료 후 이름을 다시 사용할 수 없습니다: %#v", event)
	}
}

func TestStaleCleanupDoesNotRemoveNewMembership(t *testing.T) {
	controller := (&RoomService{}).Constructor()
	sender := &recordingSender{}
	sendCommand(t, controller, "same", sender, dto.ChatMessage{Type: "join", Room: "first", User: "alice"})
	controller.mu.RLock()
	previous := controller.clients["same"].membership
	controller.mu.RUnlock()
	sendCommand(t, controller, "same", sender, dto.ChatMessage{Type: "leave"})
	sendCommand(t, controller, "same", sender, dto.ChatMessage{Type: "join", Room: "second", User: "alice"})
	controller.removeMembership("same", previous)
	sendCommand(t, controller, "same", sender, dto.ChatMessage{Type: "message", Message: "still joined"})
	if event := sender.last(t); event.Type != "message" || event.Room != "second" {
		t.Fatalf("이전 정리가 새 참여를 제거했습니다: %#v", event)
	}
}

func sendCommand(t *testing.T, controller *RoomService, connectionID string, sender *recordingSender, command dto.ChatMessage) {
	t.Helper()
	ctx := context.Background()
	if err := controller.Handle(ctx, connectionID, sender.deliver, command); err != nil {
		t.Fatalf("명령 처리 실패: %v", err)
	}
}

func assertCode(t *testing.T, event dto.ChatEvent, expected string) {
	t.Helper()
	if event.Type != "error" || event.Code != expected {
		t.Fatalf("오류 코드가 올바르지 않습니다: %#v", event)
	}
}
