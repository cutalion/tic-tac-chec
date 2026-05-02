package room

import "tic-tac-chec/internal/web/clients"

type Pairing struct {
	Players [2]clients.Client
}
