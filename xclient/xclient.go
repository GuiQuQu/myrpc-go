package xclient

// more than one servers should use xclient

import (
	"context"
	"fmt"
	"io"
	// "log"
	. "myrpc"
	"reflect"
	"sync"
)

type XClient struct {
	d       Discovery // get server
	mode    SelectMode
	opt     *Option

	mu      sync.Mutex         // protect following
	clients map[string]*Client // save all clients, one client connect one server
}

var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients {
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}

// connect to 'rpcAddr' by 'XDial' function
// if the connection has already exists, return it
func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client, ok := xc.clients[rpcAddr]
	if ok && !client.IsAvailable() {
		_ = client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		var err error
		client, err = XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMothod string, args, reply interface{}) error {
	client, err := xc.dial(rpcAddr)
	
	// fmt.Println("after dial: NumGoroutine():", runtime.NumGoroutine())
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMothod, args, reply)
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	// log.Printf("XClient.Call: rpcAddr:=>%s\n", rpcAddr)
	if err != nil {
		return err
	}

	err = xc.call(rpcAddr, ctx, serviceMethod, args, reply)
	// xc.PrintMapKey()
	return err
}

func (xc *XClient) PrintMapKey() {
	str := "Client Map Key:=>"
	for k := range xc.clients {
		str = str + k + "||||"
	}
	fmt.Println(str)
}

// concurrency broadcast `call` to all servers, and return the first answer
// if one server get error,cancel the goroutine and set err
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := xc.d.GetAll()

	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	// protect 'e' and 'reply'
	var mu sync.Mutex
	var e error

	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			// 声明reply的副本
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)

			// 设定err
			mu.Lock()
			// if get err, cancel the goroutine
			if err != nil && e == nil {
				e = err
				cancel()
			}
			// if get reply and replyDone is false, set reply and replyDone=true
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
