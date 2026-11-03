package dto

type ChatMessage struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	User    string `json:"user,omitempty"`
	Message string `json:"message,omitempty"`
}

type ChatEvent struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	From    string `json:"from,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	At      string `json:"at"`
}
