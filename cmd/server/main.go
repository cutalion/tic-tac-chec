package main

import (
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}

	startServer(listener)
}
