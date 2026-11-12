package interceptor

import (
	"crypto/subtle"
	"github.com/NARUBROWN/spine-simple-chat-demo/internal/config"
	"strings"

	"github.com/NARUBROWN/spine/core"
	"github.com/NARUBROWN/spine/pkg/httperr"
	spinectx "github.com/NARUBROWN/spine/pkg/spine"
)

type AuthInterceptor struct {
	token string
}

func NewAuthInterceptor(token string) *AuthInterceptor {
	return &AuthInterceptor{token: token}
}

func (i *AuthInterceptor) PreHandshake(ctx spinectx.Ctx, _ core.HandlerMeta) error {
	request, ok := ctx.WebSocket()
	if !ok {
		return httperr.Unauthorized("WebSocket request context is required")
	}
	token := strings.TrimSpace(request.Query("token"))
	if authorization := strings.TrimSpace(request.Header("Authorization")); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(i.token)) != 1 {
		return httperr.Unauthorized("valid WebSocket credentials are required")
	}
	return nil
}

func (*AuthInterceptor) PreHandle(spinectx.Ctx, core.HandlerMeta) error { return nil }
func (*AuthInterceptor) PostHandle(spinectx.Ctx, core.HandlerMeta)      {}
func (*AuthInterceptor) BeforeResponse(spinectx.Ctx, core.HandlerMeta, error) error {
	return nil
}
func (*AuthInterceptor) AfterCompletion(spinectx.Ctx, core.HandlerMeta, error) {}

func NewConfiguredAuthInterceptor() *AuthInterceptor {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	return NewAuthInterceptor(cfg.AuthToken)
}
