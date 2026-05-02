package main

import (
	"path/filepath"
	"testing"

	store "tic-tac-chec/internal/web/persistence/sqlite"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}
