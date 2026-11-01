package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{
		"CHAT_ADDRESS", "SPINE_PORT", "CHAT_AUTH_TOKEN", "CHAT_ALLOWED_ORIGINS", "CHAT_MAX_CONNECTIONS",
		"CHAT_MAX_MESSAGE_BYTES", "CHAT_HANDSHAKE_TIMEOUT", "CHAT_READ_TIMEOUT",
		"CHAT_WRITE_TIMEOUT", "CHAT_PING_INTERVAL", "CHAT_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(name, "")
	}
	config, err := Load()
	if err != nil {
		t.Fatalf("기본 설정 로드 실패: %v", err)
	}
	if config.Address != ":8080" || config.AuthToken != defaultAuthToken {
		t.Fatalf("기본 주소 또는 토큰이 올바르지 않습니다: %#v", config)
	}
	if config.WebSocket.MaxConnections != 256 || config.WebSocket.MaxMessageBytes != 4096 {
		t.Fatalf("기본 용량 설정이 올바르지 않습니다: %#v", config.WebSocket)
	}
	if config.WebSocket.ReadTimeout != 60*time.Second || config.WebSocket.PingInterval != 25*time.Second {
		t.Fatalf("기본 timeout 설정이 올바르지 않습니다: %#v", config.WebSocket)
	}
}

func TestLoadAddressUsesCLIThenExplicitOverride(t *testing.T) {
	t.Setenv("CHAT_ADDRESS", "")
	t.Setenv("SPINE_PORT", "18086")
	cfg, err := Load()
	if err != nil || cfg.Address != ":18086" {
		t.Fatalf("CLI 포트 설정이 반영되지 않았습니다: address=%q error=%v", cfg.Address, err)
	}
	t.Setenv("CHAT_ADDRESS", "127.0.0.1:19086")
	cfg, err = Load()
	if err != nil || cfg.Address != "127.0.0.1:19086" {
		t.Fatalf("명시한 주소가 우선하지 않습니다: address=%q error=%v", cfg.Address, err)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("CHAT_AUTH_TOKEN", "secret")
	t.Setenv("CHAT_ALLOWED_ORIGINS", "https://one.example, https://two.example")
	t.Setenv("CHAT_MAX_CONNECTIONS", "12")
	t.Setenv("CHAT_MAX_MESSAGE_BYTES", "2048")
	t.Setenv("CHAT_READ_TIMEOUT", "30s")

	config, err := Load()
	if err != nil {
		t.Fatalf("사용자 설정 로드 실패: %v", err)
	}
	if config.AuthToken != "secret" || config.WebSocket.MaxConnections != 12 || config.WebSocket.MaxMessageBytes != 2048 {
		t.Fatalf("사용자 설정이 반영되지 않았습니다: %#v", config)
	}
	if len(config.WebSocket.AllowedOrigins) != 2 || config.WebSocket.ReadTimeout != 30*time.Second {
		t.Fatalf("출처 또는 timeout 설정이 반영되지 않았습니다: %#v", config.WebSocket)
	}
}

func TestLoadRejectsInvalidLimits(t *testing.T) {
	t.Setenv("CHAT_MAX_CONNECTIONS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("잘못된 연결 제한이 거부되지 않았습니다")
	}
}
