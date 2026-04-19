package store_test

import (
	"io"
	"log"
	"os"
	"testing"
	"tic-tac-chec/cmd/web/store"

	"github.com/pressly/goose/v3"
)

func TestMain(m *testing.M) {
	goose.SetLogger(log.New(io.Discard, "", 0))
	os.Exit(m.Run())
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
