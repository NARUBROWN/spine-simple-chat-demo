package interceptor

import (
	"crypto/subtle"
	"strings"

	"github.com/NARUBROWN/spine/core"
	"github.com/NARUBROWN/spine/pkg/httperr"
)

type AuthInterceptor struct {
	token string
}

func NewAuthInterceptor(token string) *AuthInterceptor {
	return &AuthInterceptor{token: token}
}

func (i *AuthInterceptor) PreHandshake(ctx core.WebSocketHandshakeContext, _ core.HandlerMeta) error {
	token := strings.TrimSpace(ctx.Query("token"))
	if authorization := strings.TrimSpace(ctx.Header("Authorization")); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(i.token)) != 1 {
		return httperr.Unauthorized("valid WebSocket credentials are required")
	}
	return nil
}

func (*AuthInterceptor) PreHandle(core.ExecutionContext, core.HandlerMeta) error { return nil }
func (*AuthInterceptor) PostHandle(core.ExecutionContext, core.HandlerMeta)      {}
func (*AuthInterceptor) BeforeResponse(core.ExecutionContext, core.HandlerMeta, error) error {
	return nil
}
func (*AuthInterceptor) AfterCompletion(core.ExecutionContext, core.HandlerMeta, error) {}
