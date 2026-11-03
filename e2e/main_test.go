//go:build e2e

package e2e

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// 주소를 지정하지 않으면 전용 프로세스를 시작해 공유 개발 서버와 격리합니다.
func TestMain(m *testing.M) { os.Exit(runSuite(m)) }

func runSuite(m *testing.M) (code int) {
	if os.Getenv("CHAT_E2E_WS_URL") != "" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "spine-chat-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "chat")
	build := exec.Command("go", "build", "-race", "-o", binary, ".")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, output)
		return 1
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	address := listener.Addr().String()
	listener.Close()
	logfile, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logfile.Close()
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "CHAT_ADDRESS="+address,
		"CHAT_AUTH_TOKEN=e2e-token", "CHAT_ALLOWED_ORIGINS=https://chat.example.com",
		"CHAT_MAX_CONNECTIONS=16", "CHAT_MAX_MESSAGE_BYTES=4096",
		"CHAT_HANDSHAKE_TIMEOUT=5s", "CHAT_READ_TIMEOUT=60s", "CHAT_WRITE_TIMEOUT=5s",
		"CHAT_PING_INTERVAL=25s", "CHAT_SHUTDOWN_TIMEOUT=5s")
	cmd.Stdout, cmd.Stderr = logfile, logfile
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		cmd.Process.Signal(os.Interrupt)
		select {
		case err := <-done:
			if err != nil {
				fmt.Fprintln(os.Stderr, "chat shutdown:", err)
				code = 1
			}
		case <-time.After(8 * time.Second):
			cmd.Process.Kill()
			<-done
			code = 1
		}
		if code != 0 {
			data, _ := os.ReadFile(logfile.Name())
			fmt.Fprintf(os.Stderr, "chat server:\n%s", data)
		}
	}()
	os.Setenv("CHAT_E2E_WS_URL", "ws://"+address+"/ws/chat")
	os.Setenv("CHAT_E2E_ORIGIN", "https://chat.example.com")
	os.Setenv("CHAT_E2E_AUTH_TOKEN", "e2e-token")
	os.Setenv("CHAT_E2E_MAX_CONNECTIONS", "16")
	os.Setenv("CHAT_E2E_MAX_MESSAGE_BYTES", "4096")
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://" + address + "/ws/chat")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				return m.Run()
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "chat server did not become ready")
	return 1
}
