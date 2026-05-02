package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tic-tac-chec/internal/web/app"
	"tic-tac-chec/internal/web/clients"
	"tic-tac-chec/internal/web/config"
	"tic-tac-chec/internal/web/room"
	"tic-tac-chec/internal/web/ws"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
)

func setupAppServer(t *testing.T) (http.Handler, *app.App) {
	t.Helper()

	db := newTestStore(t)
	cfg, err := config.NewConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	app := app.NewApp(context.Background(), db, *cfg)

	return app.Router(), app
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

	client, _ := app.Clients().Create(context.Background())

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

	server := httptest.NewServer(router)
	defer server.Close()

	client, _ := app.Clients().Create(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sock, _, _ := connectWs(t, ctx, server.URL+"/ws/lobby", client)
	defer sock.Close(200, "closing")

	got := readJSON[ws.LobbyWaitMessage](t, ctx, sock)
	if got.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got.Type)
	}

	client2, _ := app.Clients().Create(context.Background())

	if client2.ID == client.ID {
		t.Fatal("client2 should have a different ID")
	}

	sock2, _, _ := connectWs(t, ctx, server.URL+"/ws/lobby", client2)
	defer sock2.Close(200, "closing")

	got2 := readJSON[ws.LobbyWaitMessage](t, ctx, sock2)
	if got2.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got2.Type)
	}

	paired1 := readJSON[ws.LobbyPairedMessage](t, ctx, sock)
	if paired1.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired1.Type)
	}

	paired2 := readJSON[ws.LobbyPairedMessage](t, ctx, sock2)
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

	lobby := app.LobbyRegistry().Create()

	server := httptest.NewServer(router)
	defer server.Close()

	client, _ := app.Clients().Create(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	lobbyURL := server.URL + "/ws/lobby/" + string(lobby.ID)

	sock, _, _ := connectWs(t, ctx, lobbyURL, client)
	defer sock.Close(200, "closing")

	got := readJSON[ws.LobbyWaitMessage](t, ctx, sock)
	if got.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got.Type)
	}

	client2, _ := app.Clients().Create(context.Background())

	if client2.ID == client.ID {
		t.Fatal("client2 should have a different ID")
	}

	sock2, _, _ := connectWs(t, ctx, lobbyURL, client2)
	defer sock2.Close(200, "closing")

	got2 := readJSON[ws.LobbyWaitMessage](t, ctx, sock2)
	if got2.Type != "waiting" {
		t.Errorf("expected type %s, got %s", "waiting", got2.Type)
	}

	paired1 := readJSON[ws.LobbyPairedMessage](t, ctx, sock)
	if paired1.Type != "paired" {
		t.Errorf("expected type %s, got %s", "paired", paired1.Type)
	}

	paired2 := readJSON[ws.LobbyPairedMessage](t, ctx, sock2)
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

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.Clients().Create(context.Background())
	client2, _ := app.Clients().Create(context.Background())
	client3, _ := app.Clients().Create(context.Background())

	roomRegistry := app.RoomRegistry().Create(room.Pairing{Players: [2]clients.Client{*client1, *client2}})
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

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.Clients().Create(context.Background())
	client2, _ := app.Clients().Create(context.Background())

	roomEntry := app.RoomRegistry().Create(room.Pairing{Players: [2]clients.Client{*client1, *client2}})
	go roomEntry.Room.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sock, _, _ := connectWs(t, ctx, server.URL+"/ws/room/"+string(roomEntry.Room.ID), client1)
	defer sock.Close(200, "closing")

	got := readJSON[ws.OutboundReactionMessage](t, ctx, sock)
	if got.RoomID != string(roomEntry.Room.ID) {
		t.Errorf("expected room ID to be %s, got %s", roomEntry.Room.ID, got.RoomID)
	}
}

func TestReactionMessage(t *testing.T) {
	router, app := setupAppServer(t)

	server := httptest.NewServer(router)
	defer server.Close()

	client1, _ := app.Clients().Create(context.Background())
	client2, _ := app.Clients().Create(context.Background())

	roomEntry := app.RoomRegistry().Create(room.Pairing{Players: [2]clients.Client{*client1, *client2}})
	go roomEntry.Room.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sock, _, _ := connectWs(t, ctx, server.URL+"/ws/room/"+string(roomEntry.Room.ID), client1)
	defer sock.Close(200, "closing")

	sock.Write(ctx, websocket.MessageText, []byte(`{"type":"reaction","reaction":"👋"}`))

	readJSON[ws.RoomJoinedMessage](t, ctx, sock)
	readJSON[ws.GameStateMessage](t, ctx, sock)

	got := readJSON[ws.OutboundReactionMessage](t, ctx, sock)
	if got.Type != "reaction" {
		t.Errorf("expected type to be reaction, got %s", got.Type)
	}
	if got.Reaction != "👋" {
		t.Errorf("expected reaction to be 👋, got %s", got.Reaction)
	}
}

func TestStaticRoutesServeIndexForKnownPathsAnd404ForUnknown(t *testing.T) {
	router, _ := setupAppServer(t)

	tests := []struct {
		path   string
		status int
	}{
		{"/", http.StatusOK},
		{"/rules", http.StatusOK},
		{"/lobby", http.StatusOK},
		{"/lobby/abc123", http.StatusOK},
		{"/room/xyz", http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
		{"/foo/bar", http.StatusNotFound},
		{"/random/deep/path", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, tc.status, rr.Code, "path %s", tc.path)
		})
	}
}

func readJSON[T any](t *testing.T, ctx context.Context, ws *websocket.Conn) T {
	t.Helper()

	_, msg, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("msg: %s", msg)

	var got T
	err = json.Unmarshal(msg, &got)
	if err != nil {
		t.Fatal(err)
	}

	return got
}

func connectWs(t *testing.T, ctx context.Context, url string, client *clients.Client) (*websocket.Conn, *http.Response, error) {
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
