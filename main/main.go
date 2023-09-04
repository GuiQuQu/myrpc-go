package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"myrpc"
	myrpcio "myrpc/io"
	"myrpc/registry"
	"myrpc/xclient"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"sync"
	"time"
)

// Service
type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args *Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

type Foo2 int

func (f Foo2) Multi(args *Args, reply *int) error {
	*reply = args.Num1 * args.Num2
	return nil
}

// === Service end ===
func startRegistry(done chan struct{}) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	done <- struct{}{}
	_ = http.Serve(l, nil)
}

func startServer(addrCh chan string, registryAddr string) {
	var foo Foo
	var foo2 Foo2
	l, _ := net.Listen("tcp", ":0")
	server := myrpc.NewServer()
	_ = server.Register(&foo)
	_ = server.Register(&foo2)
	// addr是服务地址,按照XDail的格式
	registry.HeartBeat(registryAddr, "tcp@"+l.Addr().String(), 0)

	addrCh <- l.Addr().String()
	server.Accept(l)
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(xc *xclient.XClient) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(xc *xclient.XClient) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func XClientTest() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_myrpc_/registry"

	done1 := make(chan struct{}, 1)

	go startRegistry(done1)
	<-done1

	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	go startServer(ch1, registryAddr)
	go startServer(ch2, registryAddr)
	<-ch1
	<-ch2
	time.Sleep(time.Second)
	d := xclient.NewMyRegistryDiscovery(registryAddr, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer xc.Close()
	call(xc)
	broadcast(xc)
}

// === RxClientTest start ====
func RxClientTest() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_myrpc_/registry"
	// start registry
	done1 := make(chan struct{}, 1)
	go startRegistry(done1)
	<-done1
	// start server
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	go startRpcServer(ch1, registryAddr)
	go startRpcServer(ch2, registryAddr)
	<-ch1
	<-ch2
	time.Sleep(time.Second)
	d := xclient.NewMyRegistryDiscovery(registryAddr, 0)
	rxc := xclient.NewRxClient(d, xclient.RandomSelect)
	callRx(rxc)
	broadcastRx(rxc)
	defer rxc.Close()
}

func startRpcServer(addr chan string, registryAddr string) {
	var foo Foo
	var foo2 Foo2
	l, _ := net.Listen("tcp", ":0")
	s := rpc.NewServer()
	_ = s.Register(&foo)
	_ = s.Register(&foo2)
	registry.HeartBeat(registryAddr, l.Addr().String(), 0)
	addr <- l.Addr().String()
	s.Accept(l)
}

func callRx(rxc *xclient.RxClient) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rxfoo(rxc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcastRx(rxc *xclient.RxClient) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rxfoo(rxc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func rxfoo(rxc *xclient.RxClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = rxc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = rxc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func startTCPServer(addr chan string) {
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	for {
		conn, _ := l.Accept()

		// serveConn
		go func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			buf := bufio.NewReader(conn)
			recvCnt := 0
			for {
				data, err := myrpcio.RecvFrame(buf)
				if err != nil && err != io.EOF {
					fmt.Println("recv error:", err)
					continue
				}
				if len(data) > 0 {
					recvCnt++
					fmt.Printf("recv(%d):%s \n", recvCnt, string(data))
				}
			}
		}(conn)
	}
}

func TestTCPIO() {
	addr := make(chan string)
	go startTCPServer(addr)

	conn, _ := net.Dial("tcp", <-addr)
	defer conn.Close()

	myrpcio.SendFrame(conn, []byte("hello"))
	// time.Sleep(time.Second * 1)
	myrpcio.SendFrame(conn, []byte("world"))
	// time.Sleep(time.Second * 1)
	myrpcio.SendFrame(conn, []byte("hello-sdasd"))
	time.Sleep(time.Second * 1)
}

func main() {
	XClientTest()
	// RxClientTest()
	// TestTCPIO()
}
