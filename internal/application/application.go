package application

import (
	"github.com/NARUBROWN/spine"
	platform "github.com/NARUBROWN/spine-echo-platform"
	"github.com/NARUBROWN/spine-simple-chat-demo/internal/config"
	"github.com/NARUBROWN/spine-simple-chat-demo/internal/interceptor"
	"github.com/NARUBROWN/spine-simple-chat-demo/src/websocket/chat"
	"github.com/NARUBROWN/spine/pkg/boot"
)

// BuildApplication은 서버를 시작하지 않고 실행과 CLI 검증에 필요한 앱을 구성합니다.
func BuildApplication() (spine.App, boot.Options, error) {
	app := platform.New()
	cfg, err := config.Load()
	if err != nil {
		return nil, boot.Options{}, err
	}
	app.InterceptorFor(boot.InterceptorWebSocket, interceptor.NewAuthInterceptor(cfg.AuthToken))
	if err := chat.Register(app); err != nil {
		return nil, boot.Options{}, err
	}
	return app, boot.Options{
		Address:                cfg.Address,
		EnableGracefulShutdown: true,
		ShutdownTimeout:        cfg.ShutdownTimeout,
		HTTP:                   &boot.HTTPOptions{WebSocket: cfg.WebSocket},
	}, nil
}
