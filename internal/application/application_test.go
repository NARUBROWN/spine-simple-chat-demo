package application

import (
	"context"
	"github.com/NARUBROWN/spine"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildApplicationValidatesAndRegistersChat(t *testing.T) {
	app, options, err := BuildApplication()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Validate(options); err != nil {
		t.Fatal(err)
	}
	prepared, err := app.Prepare(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	h, err := spine.NetHTTPHandler(prepared)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ws/chat", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated handshake status = %d", response.Code)
	}
}
