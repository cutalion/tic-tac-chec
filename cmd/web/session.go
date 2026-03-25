package main

import (
	"github.com/google/uuid"
)

type Session struct {
	token string
}

func NewSession() *Session {
	return &Session{token: newSessionToken()}
}

func newSessionToken() (token string) {
	uuid, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	return uuid.String()
}
