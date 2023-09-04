package xclient

import (
	"context"
	"io"
	"log"
	"net/rpc"
	"reflect"
	"sync"
)

type RxClient struct {
	d    Discovery // get server
	mode SelectMode

	mu      sync.Mutex             // protect 'clients'
	clients map[string]*rpc.Client // save all clients, one client connect one server
}

var _ io.Closer = (*RxClient)(nil)

func (rxc *RxClient) Close() error {
	rxc.mu.Lock()
	defer rxc.mu.Unlock()
	for key, client := range rxc.clients {
		_ = client.Close()
		delete(rxc.clients, key)
	}
	return nil
}

func NewRxClient(d Discovery, mode SelectMode) *RxClient {
	return &RxClient{
		d:       d,
		mode:    mode,
		clients: make(map[string]*rpc.Client),
	}
}

func (rxc *RxClient) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	rpcAddr, err := rxc.d.Get(rxc.mode)
	if err != nil {
		return err
	}
	err = rxc.call(rpcAddr, ctx, serviceMethod, args, reply)
	return err
}

func (rxc *RxClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	client, err := rxc.dial(rpcAddr)
	if err != nil {
		return err
	}
	// sync call rpc method
	return client.Call(serviceMethod, args, reply)
}

// get client by rpcAddr
func (rxc *RxClient) dial(rpcAddr string) (client *rpc.Client, err error) {
	rxc.mu.Lock()
	defer rxc.mu.Unlock()
	client, ok := rxc.clients[rpcAddr]
	// client down => delete client

	if !ok || client == nil {
		client, err = rpc.Dial("tcp", rpcAddr)
		if err != nil {
			log.Printf("rpc.Dial(\"tcp\", rpcAddr) failed: %v", err)
			return
		}
		rxc.clients[rpcAddr] = client
	}
	return
}

func (rxc *RxClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {

	servers, err := rxc.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			err := rxc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}
			// assgin reply
			if err == nil && !replyDone {
				replyDone = true
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return err
}
