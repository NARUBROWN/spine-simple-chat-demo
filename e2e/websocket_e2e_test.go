//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	webSocketURL  = "ws://127.0.0.1:8080/ws/chat"
	allowedOrigin = "http://127.0.0.1:8080"
)

type chatEvent struct {
	Type    string `json:"type"`
	From    string `json:"from"`
	Message string `json:"message"`
	At      string `json:"at"`
}

func TestWebSocketSameOriginBroadcastAndCapacity(t *testing.T) {
	t.Run("rejects cross origin", func(t *testing.T) {
		headers := http.Header{"Origin": []string{"https://127.0.0.1:8080"}}
		conn, resp, err := websocket.DefaultDialer.Dial(webSocketURL, headers)
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatal("교차 출처 연결이 거부되지 않았습니다")
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("교차 출처 응답 상태가 403이 아닙니다: %#v", resp)
		}
		resp.Body.Close()
	})

	t.Run("broadcasts to active same-origin clients", func(t *testing.T) {
		first := dialSameOrigin(t)
		defer first.Close()
		second := dialSameOrigin(t)
		defer second.Close()

		activateClient(t, first)
		activateClient(t, second)

		if err := first.WriteJSON(map[string]string{"message": " hello e2e "}); err != nil {
			t.Fatalf("메시지 전송 실패: %v", err)
		}

		for index, conn := range []*websocket.Conn{first, second} {
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			var event chatEvent
			if err := conn.ReadJSON(&event); err != nil {
				t.Fatalf("클라이언트 %d 이벤트 수신 실패: %v", index+1, err)
			}
			if event.Type != "message" || event.Message != "hello e2e" || event.From == "" {
				t.Fatalf("클라이언트 %d 이벤트가 올바르지 않습니다: %#v", index+1, event)
			}
			if _, err := time.Parse(time.RFC3339, event.At); err != nil {
				t.Fatalf("클라이언트 %d 타임스탬프가 RFC3339가 아닙니다: %q", index+1, event.At)
			}
		}
	})

	t.Run("rejects the 257th connection", func(t *testing.T) {
		connections := make([]*websocket.Conn, 0, 256)
		defer func() {
			for _, conn := range connections {
				conn.Close()
			}
		}()

		for index := 0; index < 256; index++ {
			connections = append(connections, dialSameOrigin(t))
		}

		headers := http.Header{"Origin": []string{allowedOrigin}}
		conn, resp, err := websocket.DefaultDialer.Dial(webSocketURL, headers)
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatal("257번째 연결이 거부되지 않았습니다")
		}
		if resp == nil {
			t.Fatal("연결 용량 초과 응답이 없습니다")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("연결 용량 초과 상태가 503이 아닙니다: %d", resp.StatusCode)
		}
		if resp.Header.Get("Retry-After") == "" {
			t.Fatal("연결 용량 초과 응답에 Retry-After가 없습니다")
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			t.Fatalf("연결 용량 초과 응답 읽기 실패: %v", readErr)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("연결 용량 초과 응답이 JSON이 아닙니다: %v", err)
		}
		if !strings.Contains(string(body), "WEBSOCKET_CAPACITY_EXCEEDED") {
			t.Fatalf("연결 용량 초과 코드가 없습니다: %s", body)
		}
	})
}

func activateClient(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.WriteJSON(map[string]string{"message": ""}); err != nil {
		t.Fatalf("클라이언트 활성화 메시지 전송 실패: %v", err)
	}
}

func dialSameOrigin(t *testing.T) *websocket.Conn {
	t.Helper()
	headers := http.Header{"Origin": []string{allowedOrigin}}
	conn, resp, err := websocket.DefaultDialer.Dial(webSocketURL, headers)
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("동일 출처 WebSocket 연결 실패: %v", err)
	}
	return conn
}
