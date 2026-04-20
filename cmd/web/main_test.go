package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
)

func setupAppServer(t *testing.T) (*http.ServeMux, *App) {
	t.Helper()

	db := newTestStore(t)
	app := NewApp(db, nil, nil) // nil bots and origins for tests

	router := http.NewServeMux()
	router.HandleFunc("/api/clients", app.CreateClient)
	router.HandleFunc("/api/me", app.Me)

	return router, app
}

func TestCreateClientRespondsWithToken(t *testing.T) {
	router, _ := setupAppServer(t)

	req, err := http.NewRequest("POST", "/api/clients", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if http.StatusCreated != rr.Code {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	assert.Regexp(t, `{"token":"[a-f0-9\-]+"}`, rr.Body.String())
}

func TestMeRespondsWithClient(t *testing.T) {
	router, app := setupAppServer(t)

	client, _ := app.clients.Create(context.Background())

	req, err := http.NewRequest("GET", "/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+string(client.ID))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if http.StatusOK != rr.Code {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	expectedBody := "{\"token\":\"" + string(client.ID) + "\"}\n"
	if expectedBody != rr.Body.String() {
		t.Errorf("expected body %s, got '%s'", expectedBody, rr.Body.String())
	}
}

func TestLobbyPairsClients(t *testing.T) {
	router, app := setupAppServer(t)
	router.HandleFunc("/ws/lobby", app.DefaultLobby)

	server := httptest.NewServer(router)
	defer server.Close()

	client, _ := app.clients.Create(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, _, _ := connectWs(t, ctx, server.URL+"/ws/lobby", client)
	defer ws.Close(200, "closing")

	got := readJSON[lobbyWaitMessage](t, ctx, ws)
	if got.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got.Type)
	}

	client2, _ := app.clients.Create(context.Background())

	if client2.ID == client.ID {
		t.Fatal("client2 should have a different ID")
	}

	ws2, _, _ := connectWs(t, ctx, server.URL+"/ws/lobby", client2)
	defer ws2.Close(200, "closing")

	got2 := readJSON[lobbyWaitMessage](t, ctx, ws2)
	if got2.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got2.Type)
	}

	paired1 := readJSON[lobbyPairedMessage](t, ctx, ws)
	if paired1.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired1.Type)
	}

	paired2 := readJSON[lobbyPairedMessage](t, ctx, ws2)
	if paired2.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired2.Type)
	}
	if paired1.RoomID == "" {
		t.Fatal("room ID should not be empty")
	}
	if paired2.RoomID == "" {
		t.Fatal("room ID should not be empty")
	}
	if paired1.RoomID != paired2.RoomID {
		t.Errorf("expected room IDs to be the same, got %s and %s", paired1.RoomID, paired2.RoomID)
	}
}

func TestLobbyWithIDPairsClients(t *testing.T) {
	router, app := setupAppServer(t)
	router.HandleFunc("/ws/lobby/{id}", app.Lobby)

	lobby := app.lobbyRegistry.Create()

	server := httptest.NewServer(router)
	defer server.Close()

	client, _ := app.clients.Create(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	lobbyURL := server.URL + "/ws/lobby/" + string(lobby.ID)

	ws, _, _ := connectWs(t, ctx, lobbyURL, client)
	defer ws.Close(200, "closing")

	got := readJSON[lobbyWaitMessage](t, ctx, ws)
	if got.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got.Type)
	}

	client2, _ := app.clients.Create(context.Background())

	if client2.ID == client.ID {
		t.Fatal("client2 should have a different ID")
	}

	ws2, _, _ := connectWs(t, ctx, lobbyURL, client2)
	defer ws2.Close(200, "closing")

	got2 := readJSON[lobbyWaitMessage](t, ctx, ws2)
	if got2.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got2.Type)
	}

	paired1 := readJSON[lobbyPairedMessage](t, ctx, ws)
	if paired1.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired1.Type)
	}

	paired2 := readJSON[lobbyPairedMessage](t, ctx, ws2)
	if paired2.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired2.Type)
	}
	if paired1.RoomID == "" {
		t.Fatal("room ID should not be empty")
	}
	if paired2.RoomID == "" {
		t.Fatal("room ID should not be empty")
	}
	if paired1.RoomID != paired2.RoomID {
		t.Errorf("expected room IDs to be the same, got %s and %s", paired1.RoomID, paired2.RoomID)
	}
}

func TestRoomJoinClientNotParticipant(t *testing.T) {
	router, app := setupAppServer(t)
	router.HandleFunc("/ws/room/{id}", app.Room)

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.clients.Create(context.Background())
	client2, _ := app.clients.Create(context.Background())
	client3, _ := app.clients.Create(context.Background())

	roomRegistry := app.roomRegistry.Create(Pairing{Players: [2]Client{*client1, *client2}})
	go roomRegistry.Room.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, resp, err := connectWs(t, ctx, server.URL+"/ws/room/"+string(roomRegistry.Room.ID), client3)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRoomJoinClientParticipant(t *testing.T) {
	router, app := setupAppServer(t)
	router.HandleFunc("/ws/room/{id}", app.Room)

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.clients.Create(context.Background())
	client2, _ := app.clients.Create(context.Background())

	roomEntry := app.roomRegistry.Create(Pairing{Players: [2]Client{*client1, *client2}})
	go roomEntry.Room.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, _, _ := connectWs(t, ctx, server.URL+"/ws/room/"+string(roomEntry.Room.ID), client1)
	defer ws.Close(200, "closing")

	got := readJSON[outboundReactionMessage](t, ctx, ws)
	if got.RoomID != string(roomEntry.Room.ID) {
		t.Errorf("expected room ID to be %s, got %s", roomEntry.Room.ID, got.RoomID)
	}
}

func TestReactionMessage(t *testing.T) {
	router, app := setupAppServer(t)
	router.HandleFunc("/ws/room/{id}", app.Room)

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.clients.Create(context.Background())
	client2, _ := app.clients.Create(context.Background())

	roomEntry := app.roomRegistry.Create(Pairing{Players: [2]Client{*client1, *client2}})
	go roomEntry.Room.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, _, _ := connectWs(t, ctx, server.URL+"/ws/room/"+string(roomEntry.Room.ID), client1)
	defer ws.Close(200, "closing")

	ws.Write(ctx, websocket.MessageText, []byte(`{"type":"reaction","reaction":"👋"}`))

	readJSON[roomJoinedMessage](t, ctx, ws)
	readJSON[gameStateMessage](t, ctx, ws)

	got := readJSON[outboundReactionMessage](t, ctx, ws)
	if got.Type != "reaction" {
		t.Errorf("expected type to be reaction, got %s", got.Type)
	}
	if got.Reaction != "👋" {
		t.Errorf("expected reaction to be 👋, got %s", got.Reaction)
	}
}

func readJSON[T any](t *testing.T, ctx context.Context, ws *websocket.Conn) T {
	t.Helper()

	_, msg, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("msg: %s", msg)

	var got T
	err = json.Unmarshal(msg, &got)
	if err != nil {
		t.Fatal(err)
	}

	return got
}

func connectWs(t *testing.T, ctx context.Context, url string, client *Client) (*websocket.Conn, *http.Response, error) {
	t.Helper()

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + string(client.ID)},
		},
	}

	return websocket.Dial(ctx, wsURL(url), opts)
}

func wsURL(httpURL string) string {
	return strings.Replace(httpURL, "http://", "ws://", 1)
}
