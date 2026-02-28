package main

import (
	"errors"
	"log"
	"net"
)

func startServer(listener net.Listener) {
	defer listener.Close()

	channel := make(chan net.Conn)
	defer close(channel)
	go lobby(channel)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)

			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		log.Printf("New connection from %s", conn.RemoteAddr())
		channel <- conn
	}
}
