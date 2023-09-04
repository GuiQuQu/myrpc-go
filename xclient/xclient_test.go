package xclient

import (
	"context"
	"fmt"
	. "myrpc"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

// test call function

type Foo int
type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) SleepSum(args Args, reply *int) error {
	time.Sleep(time.Second * 1)
	*reply = args.Num1 + args.Num2
	return nil
}

func startOneServer(addrCh chan string) {
	var foo Foo
	s := NewServer()
	s.Register(&foo)
	l, _ := net.Listen("tcp", ":0")
	addrCh <- l.Addr().String()
	s.Accept(l)
}

func startServer() (addr1, addr2 string) {
	addrCh1 := make(chan string, 1)
	addrCh2 := make(chan string, 1)
	go startOneServer(addrCh1)
	go startOneServer(addrCh2)
	addr1 = <-addrCh1
	addr2 = <-addrCh2
	return
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestXClient_Call_And_Broadcast(t *testing.T) {
	addr1, addr2 := startServer()
	d := NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})

	xc := NewXClient(d, RandomSelect, nil)
	// for 5 times
	t.Run("test call function", func(t *testing.T) {
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		for i := 0; i < 5; i++ {
			var reply int
			err := xc.Call(context.Background(), "Foo.Sum", Args{Num1: i, Num2: i}, &reply)
			_assert(err == nil && reply == i+i, fmt.Sprintf("expect %d", i+i), err)
			t.Logf("test result: %d + %d = %d", i, i, reply)
			t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		}
		time.Sleep(time.Second * 2)
		xc.Close()
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
	})

	t.Run("test broadcast function(no timeout)", func(t *testing.T) {
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		for i := 0; i < 5; i++ {
			var reply int
			err := xc.Broadcast(context.Background(), "Foo.Sum", Args{Num1: i, Num2: i}, &reply)
			_assert(err == nil && reply == i+i, fmt.Sprintf("expect %d", i+i), err)
			t.Logf("test result: %d + %d = %d", i, i, reply)
			t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		}
	})

	t.Run("test broadcast function(timeout)", func(t *testing.T) {
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		for i := 0; i < 5; i++ {
			var reply int
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
			defer cancel()
			err := xc.Broadcast(ctx, "Foo.SleepSum", Args{Num1: i, Num2: i}, &reply)
			_assert(err != nil && strings.Contains(err.Error(), "context deadline exceeded"), "expect timeout error", err)
			t.Log(err)
			t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		}
	})
}
