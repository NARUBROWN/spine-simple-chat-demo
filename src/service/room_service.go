package service

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NARUBROWN/spine-simple-chat-demo/src/dto"
)

const (
	maxRoomLength    = 32
	maxUserLength    = 32
	maxMessageLength = 1000
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type RoomService struct {
	mu      sync.RWMutex
	clients map[string]client
}

type client struct {
	room    string
	user    string
	deliver Delivery
	// 같은 연결이 퇴장 후 다시 참여해도 이전 정리가 새 참여를 지우지 않도록 구분합니다.
	membership  chan struct{}
	stopCleanup func() bool
}

func NewRoomService() *RoomService {
	return &RoomService{clients: make(map[string]client)}
}

// Delivery는 이벤트를 연결에 전달하는 애플리케이션 경계입니다.
// 방 서비스는 WebSocket 프레임이나 Gorilla 연결을 다루지 않습니다.
type Delivery func(dto.ChatEvent) error

func (c *RoomService) Handle(ctx context.Context, connectionID string, deliver Delivery, command dto.ChatMessage) error {
	switch strings.ToLower(strings.TrimSpace(command.Type)) {
	case "join":
		return c.join(ctx, connectionID, deliver, command)
	case "message":
		return c.message(connectionID, deliver, command.Message)
	case "leave":
		return c.leave(connectionID, deliver)
	default:
		return sendError(deliver, "INVALID_COMMAND", "type must be join, message, or leave")
	}
}

func (c *RoomService) join(ctx context.Context, connectionID string, deliver Delivery, command dto.ChatMessage) error {
	room := strings.TrimSpace(command.Room)
	user := strings.TrimSpace(command.User)
	if !validIdentifier(room, maxRoomLength) {
		return sendError(deliver, "INVALID_ROOM", "room must contain 1-32 letters, numbers, underscores, or hyphens")
	}
	if !validIdentifier(user, maxUserLength) {
		return sendError(deliver, "INVALID_USER", "user must contain 1-32 letters, numbers, underscores, or hyphens")
	}

	c.mu.Lock()
	if _, exists := c.clients[connectionID]; exists {
		c.mu.Unlock()
		return sendError(deliver, "ALREADY_JOINED", "leave the current room before joining another room")
	}
	for _, existing := range c.clients {
		if existing.room == room && existing.user == user {
			c.mu.Unlock()
			return sendError(deliver, "USER_ALREADY_EXISTS", "the user name is already in use in this room")
		}
	}
	membership := make(chan struct{})
	stopCleanup := context.AfterFunc(ctx, func() {
		c.removeMembership(connectionID, membership)
	})
	c.clients[connectionID] = client{room: room, user: user, deliver: deliver, membership: membership, stopCleanup: stopCleanup}
	recipients := c.roomSnapshotLocked(room)
	c.mu.Unlock()

	return c.broadcast(recipients, dto.ChatEvent{Type: "joined", Room: room, From: user, At: now()})
}

func (c *RoomService) message(connectionID string, deliver Delivery, message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" || len([]rune(trimmed)) > maxMessageLength {
		return sendError(deliver, "INVALID_MESSAGE", "message must contain 1-1000 characters")
	}

	c.mu.RLock()
	joined, exists := c.clients[connectionID]
	if !exists {
		c.mu.RUnlock()
		return sendError(deliver, "NOT_JOINED", "join a room before sending messages")
	}
	recipients := c.roomSnapshotLocked(joined.room)
	c.mu.RUnlock()

	return c.broadcast(recipients, dto.ChatEvent{
		Type: "message", Room: joined.room, From: joined.user, Message: trimmed, At: now(),
	})
}

func (c *RoomService) leave(connectionID string, deliver Delivery) error {
	c.mu.Lock()
	joined, exists := c.clients[connectionID]
	if !exists {
		c.mu.Unlock()
		return sendError(deliver, "NOT_JOINED", "join a room before leaving")
	}
	delete(c.clients, connectionID)
	joined.stopCleanup()
	recipients := c.roomSnapshotLocked(joined.room)
	c.mu.Unlock()

	// 퇴장한 클라이언트에도 명시적인 완료 이벤트를 보낸 뒤 남은 방 구성원에게 알립니다.
	event := dto.ChatEvent{Type: "left", Room: joined.room, From: joined.user, At: now()}
	if err := sendEvent(deliver, event); err != nil {
		return err
	}
	return c.broadcast(recipients, event)
}

func (c *RoomService) roomSnapshotLocked(room string) map[string]client {
	recipients := make(map[string]client)
	for id, joined := range c.clients {
		if joined.room == room {
			recipients[id] = joined
		}
	}
	return recipients
}

func (c *RoomService) broadcast(recipients map[string]client, event dto.ChatEvent) error {
	for id, recipient := range recipients {
		if err := recipient.deliver(event); err != nil {
			log.Printf("Chat delivery failed (connection=%s): %v", id, err)
			c.removeMembership(id, recipient.membership)
		}
	}
	return nil
}

// removeMembership는 취소나 오래된 전송 실패가 이후 참여 상태를 지우지 않도록 확인합니다.
func (c *RoomService) removeMembership(connectionID string, membership chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	joined, exists := c.clients[connectionID]
	if exists && joined.membership == membership {
		delete(c.clients, connectionID)
		joined.stopCleanup()
	}
}

func sendError(deliver Delivery, code, message string) error {
	return sendEvent(deliver, dto.ChatEvent{Type: "error", Code: code, Message: message, At: now()})
}

func sendEvent(deliver Delivery, event dto.ChatEvent) error {
	return deliver(event)
}

func validIdentifier(value string, maximum int) bool {
	length := len([]rune(value))
	return length > 0 && length <= maximum && identifierPattern.MatchString(value)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
