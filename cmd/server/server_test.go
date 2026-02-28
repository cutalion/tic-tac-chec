package main

import (
	"net"
	"testing"
)

func TestStartServer(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go startServer(listener)

	conn1 := connectToServer(t, listener.Addr().String())
	defer conn1.Close()
	conn2 := connectToServer(t, listener.Addr().String())
	defer conn2.Close()
	conn3 := connectToServer(t, listener.Addr().String())
	defer conn3.Close()
	conn4 := connectToServer(t, listener.Addr().String())
	defer conn4.Close()

	expectMessages(t, conn1, []string{msgWelcome, msgWhite})
	expectMessages(t, conn2, []string{msgWelcome, msgBlack})

	expectMessages(t, conn3, []string{msgWelcome, msgWhite})
	expectMessages(t, conn4, []string{msgWelcome, msgBlack})
}
