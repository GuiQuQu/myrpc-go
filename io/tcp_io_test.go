package myrpcio

import (
	"bufio"
	"net"
	"testing"
)

func startTCPServer(addr chan string, t *testing.T) {
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	for {
		conn, _ := l.Accept()

		// serveConn
		go func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			buf := bufio.NewReader(conn)
			for {
				data, _ := RecvFrame(buf)
				t.Log("recv:", string(data))
			}
		}(conn)
	}
}

func TestTCPIO(t *testing.T) {
	addr := make(chan string)
	go startTCPServer(addr, t)

	conn, _ := net.Dial("tcp", <-addr)
	defer conn.Close()

	SendFrame(conn, []byte("hello"))
	SendFrame(conn, []byte("world"))
	SendFrame(conn, []byte("hello-sdasd"))
}
