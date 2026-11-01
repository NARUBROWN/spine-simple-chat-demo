package application

import "testing"

func TestBuildApplicationValidatesAndRegistersChat(t *testing.T) {
	app, options, err := BuildApplication()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Validate(options); err != nil {
		t.Fatal(err)
	}
	registrations := app.WebSocket().Registrations()
	if len(registrations) != 1 || registrations[0].Path != "/ws/chat" {
		t.Fatalf("chat registrations: %#v", registrations)
	}
}
