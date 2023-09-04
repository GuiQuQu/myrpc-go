package myrpc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"myrpc/codec"
	myrpcio "myrpc/io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 异步调用客户端

type Call struct {
	Seq           uint64      // sequence number of this call
	ServiceMethod string      // format "Service.Method"
	Args          interface{} // arguments of the method
	Reply         interface{} // reply of the method
	Error         error       // error of the method
	Done          chan *Call  // Strobes when call is complete
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc  codec.ClientCodec
	bufConn *BufConn
	opt *Option

	reqMutex  sync.Mutex          // protect following
	reqHeader codec.RequestHeader // header of request will be reused

	mu       sync.Mutex       // protect following
	seq      uint64           // global sequence number counter
	pending  map[uint64]*Call // map of pending calls
	closing  bool             // user has called Close
	shutdown bool             // server has told us to stop
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

// remove call from Client.pending and return it
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.shutdown || client.closing {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client) terminateCalls(err error) {
	client.reqMutex.Lock()
	defer client.reqMutex.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	// get codec func
	f := codec.NewClientCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("rpc client:invalid codec type %s", opt.CodecType)
		return nil, err
	}
	// send options to server
	bufConn := &BufConn{
		ReadWriter: bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		conn:       conn,
	}
	
	if err := myrpcio.SendFrame(bufConn, opt.Marshal()); err != nil {
		err = fmt.Errorf("rpc client: send options error: %v", err)
		_ = bufConn.Close()
		return nil, err
	}
	// send to buf, then to conn
	bufConn.Flush()
	return newClientCodec(bufConn,f(bufConn), opt), nil
}

func newClientCodec(buf *BufConn, cc codec.ClientCodec, opt *Option) *Client {
	client := &Client{
		bufConn: buf,
		cc:      cc,
		opt:     opt,
		seq:     1,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

// 'conn receiver' should in loop to receive response
// stop loop when error occurs,and the 'error' it not response error
func (client *Client) receive() {
	var err error
	var resph codec.ResponseHeader
	for err == nil {
		resph = codec.ResponseHeader{}
		if err = client.cc.ReadResponseHeader(&resph); err != nil {
			break
		}
		// debug
		// log.Println("rpc client: receive response header success")
		call := client.removeCall(resph.Seq)
		switch {
		// call not exists
		case call == nil:
			err = client.cc.ReadResponseBody(nil)
		// response return error
		case resph.Error != "":
			call.Error = errors.New(resph.Error)
			err = client.cc.ReadResponseBody(nil)
			call.done()
		// response return no error
		default:
			err = client.cc.ReadResponseBody(call.Reply)
			if err != nil {
				call.Error = errors.New("rpc client: reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err)
}

type newClientFunc func(net.Conn, *Option) (*Client, error)
type newClientResult struct {
	client *Client
	err    error
}

// dial with timeout(connect timeout)
// check timeout for func 'f',f will send Option to server
func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (*Client, error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	// get conn
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	// timeout check for func 'f',f will send Option to server
	done := make(chan newClientResult, 1)
	go func() {
		client, err := f(conn, opt)
		done <- newClientResult{client: client, err: err}
	}()
	// timeout check
	if opt.ConnectTimeout == 0 {
		res := <-done
		return res.client, res.err
	} else {
		select {
		case <-time.After(opt.ConnectTimeout):
			err = fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
			_ = conn.Close()
			return nil, err
		case res := <-done:
			return res.client, res.err
		}
	}
}

// get option, if no option, use default option
func parseOptions(opts ...*Option) (*Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	opt := opts[0]
	opt.MagicNumber = MagicNumber
	if opt.CodecType == codec.UnsupportType {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

func Dial(network, addr string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewClient, network, addr, opts...)
}

// send call to server
func (client *Client) send(call *Call) {
	client.reqMutex.Lock()
	defer client.reqMutex.Unlock()

	// register call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	client.reqHeader.ServiceMethod = call.ServiceMethod
	client.reqHeader.Seq = seq

	// send request to buf
	if err := client.cc.WriteRequest(&client.reqHeader, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	} 
	// buf to conn
	if err := client.bufConn.Flush(); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	} else {
		// debug
		// log.Println("rpc client: send request success")
	}
}

// build call to send to server, asynchronous
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

// build call to send to server, synchronous
// ctx can add timeout setting
func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	done := make(chan *Call, 1)
	call := client.Go(serviceMethod, args, reply, done)

	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return fmt.Errorf("rpc client: call failed: %v", ctx.Err())
	case call := <-done:
		return call.Error

	}
}

// http support
// send http request to server,and build connection used in rpc
func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	// send http request to 'defaultRPCPath'
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))

	// get response
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

// support multiple protocol(simple version) 
// rpcAddr example http@localhost:1234,tcp@localhost:1234
func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client:invalid rpc address %s", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}
