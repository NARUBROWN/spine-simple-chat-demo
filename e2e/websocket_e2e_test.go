//go:build e2e

package e2e

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type chatEvent struct {
	Type    string `json:"type"`
	Room    string `json:"room"`
	From    string `json:"from"`
	Message string `json:"message"`
	Code    string `json:"code"`
	At      string `json:"at"`
}

func TestWebSocketAuthenticationAndOrigin(t *testing.T) {
	settings := loadSettings(t)

	t.Run("requires credentials before upgrade", func(t *testing.T) {
		conn, resp, err := websocket.DefaultDialer.Dial(settings.webSocketURL, http.Header{"Origin": {settings.origin}})
		closeResponse(conn, resp)
		if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("인증 없는 handshake가 401로 거부되지 않았습니다: response=%#v error=%v", resp, err)
		}
	})

	t.Run("rejects a disallowed origin", func(t *testing.T) {
		headers := authorizedHeaders(settings, "https://disallowed.example")
		conn, resp, err := websocket.DefaultDialer.Dial(settings.webSocketURL, headers)
		closeResponse(conn, resp)
		if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("허용되지 않은 출처가 403으로 거부되지 않았습니다: response=%#v error=%v", resp, err)
		}
	})

	t.Run("accepts a query token for browser clients", func(t *testing.T) {
		parsed, err := url.Parse(settings.webSocketURL)
		if err != nil {
			t.Fatalf("WebSocket URL 파싱 실패: %v", err)
		}
		query := parsed.Query()
		query.Set("token", settings.token)
		parsed.RawQuery = query.Encode()
		conn, resp, err := websocket.DefaultDialer.Dial(parsed.String(), http.Header{"Origin": {settings.origin}})
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			t.Fatalf("query token handshake 실패: %v", err)
		}
		conn.Close()
	})
}

func TestWebSocketRoomProtocol(t *testing.T) {
	settings := loadSettings(t)
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 10)
	generalRoom := "general-" + suffix
	otherRoom := "other-" + suffix
	aliceName := "alice-" + suffix
	bobName := "bob-" + suffix
	carolName := "carol-" + suffix
	alice := dial(t, settings)
	defer alice.Close()
	bob := dial(t, settings)
	defer bob.Close()
	carol := dial(t, settings)
	defer carol.Close()
	unjoined := dial(t, settings)
	defer unjoined.Close()

	writeCommand(t, unjoined, map[string]string{"type": "message", "message": "too early"})
	assertEvent(t, readEvent(t, unjoined), "error", "", "", "NOT_JOINED")

	writeCommand(t, alice, map[string]string{"type": "join", "room": generalRoom, "user": aliceName})
	assertEvent(t, readEvent(t, alice), "joined", generalRoom, aliceName, "")

	writeCommand(t, bob, map[string]string{"type": "join", "room": generalRoom, "user": bobName})
	assertEvent(t, readEvent(t, alice), "joined", generalRoom, bobName, "")
	assertEvent(t, readEvent(t, bob), "joined", generalRoom, bobName, "")

	writeCommand(t, carol, map[string]string{"type": "join", "room": otherRoom, "user": carolName})
	assertEvent(t, readEvent(t, carol), "joined", otherRoom, carolName, "")

	writeCommand(t, alice, map[string]string{"type": "message", "message": " hello e2e "})
	for index, conn := range []*websocket.Conn{alice, bob} {
		event := readEvent(t, conn)
		if event.Type != "message" || event.Room != generalRoom || event.From != aliceName || event.Message != "hello e2e" {
			t.Fatalf("클라이언트 %d의 메시지 이벤트가 올바르지 않습니다: %#v", index+1, event)
		}
	}

	duplicate := dial(t, settings)
	defer duplicate.Close()
	writeCommand(t, duplicate, map[string]string{"type": "join", "room": generalRoom, "user": aliceName})
	assertEvent(t, readEvent(t, duplicate), "error", "", "", "USER_ALREADY_EXISTS")

	writeCommand(t, bob, map[string]string{"type": "leave"})
	assertEvent(t, readEvent(t, bob), "left", generalRoom, bobName, "")
	assertEvent(t, readEvent(t, alice), "left", generalRoom, bobName, "")
}

func TestWebSocketCapacityAndMessageLimit(t *testing.T) {
	settings := loadSettings(t)

	t.Run("rejects a connection beyond capacity", func(t *testing.T) {
		connections := make([]*websocket.Conn, 0, settings.maxConnections)
		defer func() {
			for _, conn := range connections {
				conn.Close()
			}
		}()
		for index := 0; index < settings.maxConnections; index++ {
			connections = append(connections, dial(t, settings))
		}

		conn, resp, err := websocket.DefaultDialer.Dial(settings.webSocketURL, authorizedHeaders(settings, settings.origin))
		if conn != nil {
			conn.Close()
		}
		if err == nil || resp == nil {
			t.Fatalf("용량 초과 연결이 거부되지 않았습니다: response=%#v error=%v", resp, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
			t.Fatalf("용량 초과 응답이 올바르지 않습니다: status=%d headers=%v", resp.StatusCode, resp.Header)
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil || !strings.Contains(string(body), "WEBSOCKET_CAPACITY_EXCEEDED") {
			t.Fatalf("용량 초과 응답 본문이 올바르지 않습니다: body=%s error=%v", body, readErr)
		}
	})

	t.Run("closes an oversized message", func(t *testing.T) {
		conn := dial(t, settings)
		defer conn.Close()
		payload := map[string]string{"type": "message", "message": strings.Repeat("x", settings.maxMessageBytes)}
		if err := conn.WriteJSON(payload); err != nil {
			t.Fatalf("크기 초과 메시지 쓰기 실패: %v", err)
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, err := conn.ReadMessage()
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseMessageTooBig {
			t.Fatalf("크기 초과 연결이 1009로 종료되지 않았습니다: %v", err)
		}
	})
}

type testSettings struct {
	webSocketURL    string
	origin          string
	token           string
	maxConnections  int
	maxMessageBytes int
}

func loadSettings(t *testing.T) testSettings {
	t.Helper()
	settings := testSettings{
		webSocketURL:    envOrDefault("CHAT_E2E_WS_URL", "ws://127.0.0.1:8080/ws/chat"),
		origin:          envOrDefault("CHAT_E2E_ORIGIN", "http://127.0.0.1:8080"),
		token:           envOrDefault("CHAT_E2E_AUTH_TOKEN", "demo-token"),
		maxConnections:  envInt(t, "CHAT_E2E_MAX_CONNECTIONS", 256),
		maxMessageBytes: envInt(t, "CHAT_E2E_MAX_MESSAGE_BYTES", 4096),
	}
	return settings
}

func dial(t *testing.T, settings testSettings) *websocket.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, resp, err := websocket.DefaultDialer.Dial(settings.webSocketURL, authorizedHeaders(settings, settings.origin))
		status := 0
		if resp != nil {
			status = resp.StatusCode
			resp.Body.Close()
		}
		if err == nil {
			return conn
		}
		if status != http.StatusServiceUnavailable || time.Now().After(deadline) {
			t.Fatalf("WebSocket 연결 실패: status=%d error=%v", status, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func authorizedHeaders(settings testSettings, origin string) http.Header {
	return http.Header{
		"Authorization": {"Bearer " + settings.token},
		"Origin":        {origin},
	}
}

func writeCommand(t *testing.T, conn *websocket.Conn, command map[string]string) {
	t.Helper()
	if err := conn.WriteJSON(command); err != nil {
		t.Fatalf("WebSocket 명령 전송 실패: %v", err)
	}
}

func readEvent(t *testing.T, conn *websocket.Conn) chatEvent {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var event chatEvent
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("WebSocket 이벤트 수신 실패: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, event.At); err != nil {
		t.Fatalf("이벤트 타임스탬프가 RFC3339가 아닙니다: %q", event.At)
	}
	return event
}

func assertEvent(t *testing.T, event chatEvent, eventType, room, from, code string) {
	t.Helper()
	if event.Type != eventType || event.Room != room || event.From != from || event.Code != code {
		t.Fatalf("이벤트가 올바르지 않습니다: %#v", event)
	}
}

func closeResponse(conn *websocket.Conn, response *http.Response) {
	if conn != nil {
		conn.Close()
	}
	if response != nil {
		response.Body.Close()
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		t.Fatalf("%s must be a positive integer", name)
	}
	return parsed
}
