package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
)

var server *httptest.Server

func TestMain(m *testing.M) {
	lobby = make(chan playerConn)
	go lobbyLoop(lobby)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", handleWebSocket)
	server = httptest.NewServer(mux)

	code := m.Run()
	server.Close()
	os.Exit(code)
}

func TestJoinFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws1, _, _ := websocket.Dial(ctx, server.URL+"/ws", nil)
	ws1.Write(ctx, websocket.MessageText, []byte(`{"type": "join"}`))
	defer ws1.Close(200, "closing")

	ws2, _, _ := websocket.Dial(ctx, server.URL+"/ws", nil)
	ws2.Write(ctx, websocket.MessageText, []byte(`{"type": "join"}`))
	defer ws2.Close(200, "closing")

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

func TestReconnectFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws1 := join(t, server, ctx)
	defer ws1.Close(200, "closing")

	ws2 := join(t, server, ctx)
	token2 := readToken(t, ws2, ctx)
	ws2.Close(200, "closing") // close, emulate disconnect

	ws3 := reconnect(t, server, ctx, token2)
	defer ws3.Close(200, "closing")

	token3 := readToken(t, ws3, ctx)
	if token3 != token2 {
		t.Errorf("reconnected token should match original")
	}
}

func join(t *testing.T, server *httptest.Server, ctx context.Context) *websocket.Conn {
	t.Helper()

	ws, _, err := websocket.Dial(ctx, server.URL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	ws.Write(ctx, websocket.MessageText, []byte(`{"type": "join"}`))

	return ws
}

func readToken(t *testing.T, ws *websocket.Conn, ctx context.Context) (token string) {
	t.Helper()

	for range 3 {
		_, msg, err := ws.Read(ctx)

		if err != nil {
			t.Fatalf("read error: %v", err)
		}

		var m map[string]any
		json.Unmarshal(msg, &m)

		if m["token"] != nil {
			token = m["token"].(string)
			break
		}
	}

	if token == "" {
		t.Fatal("token should be non-empty")
	}

	return token
}

func reconnect(t *testing.T, server *httptest.Server, ctx context.Context, token string) *websocket.Conn {
	t.Helper()

	ws, _, err := websocket.Dial(ctx, server.URL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	ws.Write(ctx, websocket.MessageText, []byte(`{"type": "reconnect", "token": "`+token+`"}`))

	return ws
}
