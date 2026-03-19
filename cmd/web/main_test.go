package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestJoinFlow(t *testing.T) {
	server := setupServer()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws1, _, _ := websocket.Dial(ctx, server.URL+"/ws", nil)
	ws2, _, _ := websocket.Dial(ctx, server.URL+"/ws", nil)
	defer ws1.Close(200, "closing")
	defer ws2.Close(200, "closing")

	ws1.Write(ctx, websocket.MessageText, []byte(`{"type": "join"}`))
	ws2.Write(ctx, websocket.MessageText, []byte(`{"type": "join"}`))

	_, msg1, _ := ws1.Read(ctx)
	_, msg2, _ := ws2.Read(ctx)

	var m1, m2 map[string]any
	json.Unmarshal(msg1, &m1)
	json.Unmarshal(msg2, &m2)

	if m1["color"] == m2["color"] {
		t.Errorf("colors should be opposite")
	}
	if m1["token"] == m2["token"] {
		t.Errorf("tokens should be different")
	}
	if m1["token"] == "" || m2["token"] == "" {
		t.Errorf("tokens should be non-empty")
	}

}

func setupServer() *httptest.Server {
	go lobbyLoop(lobby)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", handleWebSocket)

	return httptest.NewServer(mux)
}
