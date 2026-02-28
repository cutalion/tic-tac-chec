package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
func connectToServer(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	return conn
}

func expectMessages(t *testing.T, conn net.Conn, messages []string) {
	t.Helper()
	scanner := bufio.NewScanner(conn)
	for _, expected := range messages {
		if msg := readFromScanner(t, scanner); msg != expected {
			t.Fatalf("Expected: %s, but got: %s", expected, msg)
		}
	}
}

func readFromScanner(t *testing.T, scanner *bufio.Scanner) string {
	t.Helper()
	if !scanner.Scan() {
		t.Fatal(scanner.Err())
	}
	return scanner.Text()
}
