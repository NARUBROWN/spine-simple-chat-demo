package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NARUBROWN/spine/pkg/boot"
)

const defaultAuthToken = "demo-token"

type Config struct {
	Address         string
	AuthToken       string
	ShutdownTimeout time.Duration
	WebSocket       boot.WebSocketOptions
}

func Load() (Config, error) {
	config := Config{
		Address:         envOrDefault("CHAT_ADDRESS", ":"+envOrDefault("SPINE_PORT", "8080")),
		AuthToken:       envOrDefault("CHAT_AUTH_TOKEN", defaultAuthToken),
		ShutdownTimeout: 10 * time.Second,
		WebSocket: boot.WebSocketOptions{
			MaxConnections:   256,
			MaxMessageBytes:  4096,
			HandshakeTimeout: 5 * time.Second,
			ReadTimeout:      60 * time.Second,
			WriteTimeout:     5 * time.Second,
			PingInterval:     25 * time.Second,
		},
	}
	config.WebSocket.AllowedOrigins = splitCSV(os.Getenv("CHAT_ALLOWED_ORIGINS"))

	var err error
	if config.WebSocket.MaxConnections, err = positiveInt("CHAT_MAX_CONNECTIONS", config.WebSocket.MaxConnections); err != nil {
		return Config{}, err
	}
	if config.WebSocket.MaxMessageBytes, err = positiveInt64("CHAT_MAX_MESSAGE_BYTES", config.WebSocket.MaxMessageBytes); err != nil {
		return Config{}, err
	}
	if config.WebSocket.HandshakeTimeout, err = duration("CHAT_HANDSHAKE_TIMEOUT", config.WebSocket.HandshakeTimeout); err != nil {
		return Config{}, err
	}
	if config.WebSocket.ReadTimeout, err = duration("CHAT_READ_TIMEOUT", config.WebSocket.ReadTimeout); err != nil {
		return Config{}, err
	}
	if config.WebSocket.WriteTimeout, err = duration("CHAT_WRITE_TIMEOUT", config.WebSocket.WriteTimeout); err != nil {
		return Config{}, err
	}
	if config.WebSocket.PingInterval, err = duration("CHAT_PING_INTERVAL", config.WebSocket.PingInterval); err != nil {
		return Config{}, err
	}
	if config.ShutdownTimeout, err = duration("CHAT_SHUTDOWN_TIMEOUT", config.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(config.AuthToken) == "" {
		return Config{}, fmt.Errorf("CHAT_AUTH_TOKEN must not be empty")
	}
	return config, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func positiveInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func positiveInt64(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}
