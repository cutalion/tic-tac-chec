package main

import (
	"path/filepath"
	"testing"

	"tic-tac-chec/cmd/web/store"

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
