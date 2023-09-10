package myrpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// 测试NewClient函数超时和不超时情况,并且检测goroutine泄露
func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")

	f := func(conn net.Conn, opt *Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(time.Second)
		return nil, nil
	}
	t.Run("timeout", func(t *testing.T) {
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: time.Millisecond})
		if err == nil || err.Error() != "rpc client: connect timeout: expect within 1ms" {
			t.Errorf("expect a timeout error")
		}
		time.Sleep(time.Second * 2)
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
	})
	t.Run("0", func(t *testing.T) {
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: 0})
		_assert(err == nil, "0 timeout should not timeout")
		time.Sleep(time.Second)
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
	})
}

// 测试Call函数超时和不超时情况
type Bar int

func (b Bar) TimeoutSum(args Args, reply *int) error {
	time.Sleep(time.Second * 2)
	*reply = args.Num1 + args.Num2
	return nil
}

func (b Bar) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func startServer(addr chan string) {
	var b Bar
	_ = Register(&b)
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	Accept(l)
}

func TestClient_Call(t *testing.T) {
	addrCh := make(chan string)
	go startServer(addrCh)
	client, _ := Dial("tcp", <-addrCh)
	defer client.Close()
	// send call
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// defer cancel()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		var reply int
		err := client.Call(ctx, "Bar.Sum", Args{Num1: i, Num2: i}, &reply)
		_assert(err == nil && reply == i+i, fmt.Sprintf("expect %d", i+i), err)
		t.Log("test result:", i, "+", i, "=", reply)
	}
}

func TestClient_CallTimeout(t *testing.T) {
	t.Parallel()

	addrCh := make(chan string)
	go startServer(addrCh)
	addr := <-addrCh
	time.Sleep(time.Second)

	t.Run("client call function timeout", func(t *testing.T) {

		client, _ := Dial("tcp", addr)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 1)
		defer cancel()
		var reply int
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		err := client.Call(ctx, "Bar.TimeoutSum", Args{Num1: 250, Num2: 750}, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "expect a client timeout error", err.Error())
		t.Log(err.Error())
		time.Sleep(time.Second * 2)
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
	})

	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr, &Option{
			RWMutex:       new(sync.RWMutex),
			HandleTimeout: time.Second,
		})
		var reply int
		bg := context.Background()
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
		err := client.Call(bg, "Bar.TimeoutSum", Args{Num1: 250, Num2: 750}, &reply)
		_assert(err != nil && strings.Contains(err.Error(), "handle timeout"), "expect a handle timeout error", err.Error())
		t.Log(err.Error())
		time.Sleep(time.Second * 2)
		t.Log("runtime.NumGoroutine():", runtime.NumGoroutine())
	})
}

// 测试多种连接协议
func TestXDial(t *testing.T) {
	if runtime.GOOS == "linux" {
		ch := make(chan struct{})
		addr := "/tmp/myrpc.sock"
		// 开启unix监听
		go func() {
			Register(new(Bar))
			_ = os.Remove(addr)
			l, _ := net.Listen("unix", addr)
			// if err != nil {
			// 	t.Fatal("failed to listen unix socket")

			// }
			ch <- struct{}{}
			Accept(l)
		}()
		<-ch
		client, err := XDial("unix@" + addr)
		defer func() { _ = client.Close() }()
		_assert(err == nil, "XDial failed", err)
		var reply int
		client.Call(context.Background(), "Bar.Sum", Args{Num1: 2,Num2: 5}, &reply)
		_assert(reply == 7, "expect 7", reply)
	}

}
