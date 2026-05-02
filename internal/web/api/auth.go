package api

import (
	"errors"
	"net/http"
	"strings"
	"tic-tac-chec/internal/web/clients"
	store "tic-tac-chec/internal/web/persistence/sqlite"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

func (a *API) authenticate(r *http.Request) (*clients.Client, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		header := r.Header.Get("Authorization")
		if header == "" {
			return nil, ErrUnauthorized
		}

		split := strings.Split(header, " ")
		if len(split) != 2 {
			return nil, ErrUnauthorized
		}

		if split[0] != "Bearer" {
			return nil, ErrUnauthorized
		}

		token = split[1]
	}

	if token == "" {
		return nil, ErrUnauthorized
	}

	c, err := a.clients.Lookup(r.Context(), clients.ClientID(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	return c, nil
}

func (a *API) handleAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnauthorized) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	http.Error(w, "internal server error", http.StatusInternalServerError)
}
