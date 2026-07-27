package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NARUBROWN/spine/pkg/ws"
)

const (
	maxRoomLength    = 32
	maxUserLength    = 32
	maxMessageLength = 1000
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type ChatController struct {
	mu      sync.RWMutex
	clients map[string]client
}

type client struct {
	room   string
	user   string
	sender ws.Sender
}

func NewChatController() *ChatController {
	return &ChatController{clients: make(map[string]client)}
}

type ChatCommand struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	User    string `json:"user,omitempty"`
	Message string `json:"message,omitempty"`
}

type ChatEvent struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	From    string `json:"from,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	At      string `json:"at"`
}

func (c *ChatController) OnMessage(ctx context.Context, connID ws.ConnectionID, command ChatCommand) error {
	sender, ok := ctx.Value(ws.SenderKey).(ws.Sender)
	if !ok || sender == nil {
		return errors.New("websocket sender is unavailable")
	}

	switch strings.ToLower(strings.TrimSpace(command.Type)) {
	case "join":
		return c.join(connID.Value, sender, command)
	case "message":
		return c.message(connID.Value, sender, command.Message)
	case "leave":
		return c.leave(connID.Value, sender)
	default:
		return sendError(sender, "INVALID_COMMAND", "type must be join, message, or leave")
	}
}

func (c *ChatController) join(connectionID string, sender ws.Sender, command ChatCommand) error {
	room := strings.TrimSpace(command.Room)
	user := strings.TrimSpace(command.User)
	if !validIdentifier(room, maxRoomLength) {
		return sendError(sender, "INVALID_ROOM", "room must contain 1-32 letters, numbers, underscores, or hyphens")
	}
	if !validIdentifier(user, maxUserLength) {
		return sendError(sender, "INVALID_USER", "user must contain 1-32 letters, numbers, underscores, or hyphens")
	}

	c.mu.Lock()
	if _, exists := c.clients[connectionID]; exists {
		c.mu.Unlock()
		return sendError(sender, "ALREADY_JOINED", "leave the current room before joining another room")
	}
	for _, existing := range c.clients {
		if existing.room == room && existing.user == user {
			c.mu.Unlock()
			return sendError(sender, "USER_ALREADY_EXISTS", "the user name is already in use in this room")
		}
	}
	c.clients[connectionID] = client{room: room, user: user, sender: sender}
	recipients := c.roomSnapshotLocked(room)
	c.mu.Unlock()

	return c.broadcast(recipients, ChatEvent{Type: "joined", Room: room, From: user, At: now()})
}

func (c *ChatController) message(connectionID string, sender ws.Sender, message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" || len([]rune(trimmed)) > maxMessageLength {
		return sendError(sender, "INVALID_MESSAGE", "message must contain 1-1000 characters")
	}

	c.mu.RLock()
	joined, exists := c.clients[connectionID]
	if !exists {
		c.mu.RUnlock()
		return sendError(sender, "NOT_JOINED", "join a room before sending messages")
	}
	recipients := c.roomSnapshotLocked(joined.room)
	c.mu.RUnlock()

	return c.broadcast(recipients, ChatEvent{
		Type: "message", Room: joined.room, From: joined.user, Message: trimmed, At: now(),
	})
}

func (c *ChatController) leave(connectionID string, sender ws.Sender) error {
	c.mu.Lock()
	joined, exists := c.clients[connectionID]
	if !exists {
		c.mu.Unlock()
		return sendError(sender, "NOT_JOINED", "join a room before leaving")
	}
	delete(c.clients, connectionID)
	recipients := c.roomSnapshotLocked(joined.room)
	c.mu.Unlock()

	// 퇴장한 클라이언트에도 명시적인 완료 이벤트를 보낸 뒤 남은 방 구성원에게 알립니다.
	event := ChatEvent{Type: "left", Room: joined.room, From: joined.user, At: now()}
	if err := sendEvent(sender, event); err != nil {
		return err
	}
	return c.broadcast(recipients, event)
}

func (c *ChatController) roomSnapshotLocked(room string) map[string]client {
	recipients := make(map[string]client)
	for id, joined := range c.clients {
		if joined.room == room {
			recipients[id] = joined
		}
	}
	return recipients
}

func (c *ChatController) broadcast(recipients map[string]client, event ChatEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	for id, recipient := range recipients {
		if err := recipient.sender.Send(ws.TextMessage, data); err != nil {
			log.Printf("Chat delivery failed (connection=%s): %v", id, err)
			c.mu.Lock()
			delete(c.clients, id)
			c.mu.Unlock()
		}
	}
	return nil
}

func sendError(sender ws.Sender, code, message string) error {
	return sendEvent(sender, ChatEvent{Type: "error", Code: code, Message: message, At: now()})
}

func sendEvent(sender ws.Sender, event ChatEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return sender.Send(ws.TextMessage, data)
}

func validIdentifier(value string, maximum int) bool {
	length := len([]rune(value))
	return length > 0 && length <= maximum && identifierPattern.MatchString(value)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
