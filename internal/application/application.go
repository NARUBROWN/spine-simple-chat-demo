package application

import (
	"github.com/NARUBROWN/spine"
	echoplatform "github.com/NARUBROWN/spine-echo-platform"
	gorillaplatform "github.com/NARUBROWN/spine-gorilla-platform"
	"github.com/NARUBROWN/spine-simple-chat-demo/internal/config"
	"github.com/NARUBROWN/spine-simple-chat-demo/internal/interceptor"
	service "github.com/NARUBROWN/spine-simple-chat-demo/src/service"
	"github.com/NARUBROWN/spine-simple-chat-demo/src/websocket/chat"
	"github.com/NARUBROWN/spine/pkg/boot"
)

// BuildApplication은 서버를 시작하지 않고 실행과 CLI 검증에 필요한 앱을 구성합니다.
func BuildApplication() (spine.App, boot.Options, error) {
	// HTTP 업그레이드와 WebSocket 프레임 구현을 각각 선택합니다.
	app := spine.New(echoplatform.Option(), gorillaplatform.Option())
	app.Constructor(service.NewRoomService)
	cfg, err := config.Load()
	if err != nil {
		return nil, boot.Options{}, err
	}
	app.Constructor(interceptor.NewConfiguredAuthInterceptor)
	app.InterceptorFor(boot.InterceptorWebSocket, (*interceptor.AuthInterceptor)(nil))
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
