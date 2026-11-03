//go:build e2e

package e2e

import (
	"errors"
	"io"
	"net"
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

	t.Run("rejects invalid bearer even with a valid query token", func(t *testing.T) {
		parsed, err := url.Parse(settings.webSocketURL)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("token", settings.token)
		parsed.RawQuery = query.Encode()
		headers := http.Header{"Origin": {settings.origin}, "Authorization": {"Bearer " + settings.token + "-invalid"}}
		conn, resp, err := websocket.DefaultDialer.Dial(parsed.String(), headers)
		closeResponse(conn, resp)
		if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("잘못된 bearer 인증이 거부되지 않았습니다: response=%#v error=%v", resp, err)
		}
	})

	t.Run("rejected handshakes release their connection slots", func(t *testing.T) {
		for index := 0; index < settings.maxConnections+1; index++ {
			conn, resp, err := websocket.DefaultDialer.Dial(settings.webSocketURL, http.Header{"Origin": {settings.origin}})
			closeResponse(conn, resp)
			if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("인증 실패 %d회 후 슬롯이 소진되었거나 요청이 허용되었습니다: response=%#v error=%v", index+1, resp, err)
			}
		}
		conn := dial(t, settings)
		conn.Close()
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

	// 같은 연결의 후속 명령이 처리되면 앞선 브로드캐스트가 끝났음을 알 수 있습니다.
	writeCommand(t, alice, map[string]string{"type": "message", "message": "barrier"})
	for _, conn := range []*websocket.Conn{alice, bob} {
		event := readEvent(t, conn)
		if event.Type != "message" || event.Room != generalRoom || event.From != aliceName || event.Message != "barrier" {
			t.Fatalf("방 격리 검사 동기화가 실패했습니다: %#v", event)
		}
	}

	// 동기화된 후에도 다른 방의 메시지가 Carol에게 먼저 도착하면 격리가 깨진 것입니다.
	writeCommand(t, carol, map[string]string{"type": "message", "message": "other room only"})
	otherEvent := readEvent(t, carol)
	if otherEvent.Type != "message" || otherEvent.Room != otherRoom || otherEvent.From != carolName || otherEvent.Message != "other room only" {
		t.Fatalf("다른 방으로 메시지가 전달되었습니다: %#v", otherEvent)
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
		// 연결 종료가 실제 수용 용량을 반환하는지 확인합니다.
		if err := connections[0].Close(); err != nil {
			t.Fatalf("연결 종료 실패: %v", err)
		}
		replacement := dial(t, settings)
		replacement.Close()
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

func TestMalformedMessageDoesNotAffectOtherConnections(t *testing.T) {
	settings := loadSettings(t)
	victim := dial(t, settings)
	defer victim.Close()
	room := "malformed-" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 10)
	writeCommand(t, victim, map[string]string{"type": "join", "room": room, "user": "victim"})
	assertEvent(t, readEvent(t, victim), "joined", room, "victim", "")

	attacker := dial(t, settings)
	defer attacker.Close()
	if err := attacker.WriteMessage(websocket.TextMessage, []byte(`{"type":`)); err != nil {
		t.Fatalf("잘못된 JSON 전송 실패: %v", err)
	}
	attacker.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := attacker.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseInternalServerErr {
		t.Fatalf("잘못된 JSON 연결이 종료되지 않았습니다: %v", err)
	}

	writeCommand(t, victim, map[string]string{"type": "message", "message": "still available"})
	event := readEvent(t, victim)
	if event.Type != "message" || event.Room != room || event.From != "victim" || event.Message != "still available" {
		t.Fatalf("정상 연결이 영향을 받았습니다: %#v", event)
	}
}

func TestDisconnectAllowsNameReuse(t *testing.T) {
	settings := loadSettings(t)
	room := "reconnect-" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 10)
	first := dial(t, settings)
	writeCommand(t, first, map[string]string{"type": "join", "room": room, "user": "alice"})
	assertEvent(t, readEvent(t, first), "joined", room, "alice", "")
	first.Close()

	// 서버가 소켓 종료를 관찰하고 연결 context를 취소할 시간을 제한해 기다립니다.
	deadline := time.Now().Add(3 * time.Second)
	for {
		replacement := dial(t, settings)
		writeCommand(t, replacement, map[string]string{"type": "join", "room": room, "user": "alice"})
		event := readEvent(t, replacement)
		if event.Type == "joined" && event.Room == room && event.From == "alice" {
			replacement.Close()
			return
		}
		replacement.Close()
		if event.Type != "error" || event.Code != "USER_ALREADY_EXISTS" || time.Now().After(deadline) {
			t.Fatalf("연결 종료 후 같은 이름으로 재참여할 수 없습니다: %#v", event)
		}
		time.Sleep(20 * time.Millisecond)
	}
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
	parsed, err := url.Parse(settings.webSocketURL)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || !isLoopbackHost(parsed.Hostname()) {
		t.Fatalf("CHAT_E2E_WS_URL은 전용 로컬 서버의 loopback WebSocket URL이어야 합니다")
	}
	return settings
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func TestLoopbackHostOnly(t *testing.T) {
	for _, test := range []struct {
		host    string
		allowed bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"192.0.2.1", false},
		{"example.com", false},
		{"", false},
	} {
		if got := isLoopbackHost(test.host); got != test.allowed {
			t.Errorf("host %q: allowed=%v, want %v", test.host, got, test.allowed)
		}
	}
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
