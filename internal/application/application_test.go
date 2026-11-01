package application

import (
	"reflect"
	"testing"

	"github.com/NARUBROWN/spine-simple-chat-demo/src/websocket/chat"
)

func TestBuildApplicationValidatesAndRegistersChat(t *testing.T) {
	app, options, err := BuildApplication()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Validate(options); err != nil {
		t.Fatal(err)
	}
	registrations := app.WebSocket().Registrations()
	for _, registration := range registrations {
		if registration.Path == "/ws/chat" {
			if registration.Meta.ControllerType != reflect.TypeFor[*chat.ChatSocket]() || registration.Meta.Method.Name != "Handle" {
				t.Fatalf("chat handler: %#v", registration.Meta)
			}
			return
		}
	}
	t.Fatalf("/ws/chat registration is missing: %#v", registrations)
}
