package myrpc

import (
	"errors"
	"fmt"
	"io"
	"log"
	"myrpc/codec"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Server struct {
	// save service
	serviceMap sync.Map
	// save server addr
	Addr string
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// register a service to server
func (s *Server) Register(rcvr interface{}) error {
	service := newService(rcvr)

	if _, dup := s.serviceMap.LoadOrStore(service.name, service); dup {
		// service already registered
		return errors.New("rpc server: service already defined: " + service.name)
	}
	return nil
}

// register a service to default server
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// find service by serviceMethod, example is 'Foo.Sum'
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		// service not found
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		// method not found
		err = errors.New("rpc server: can't find method " + methodName)
		return
	}
	return
}

func (s *Server) Accept(lis net.Listener) {
	// save addr
	s.Addr = lis.Addr().String()
	// start listen
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("rpc server(%s): accept error: %v", s.Addr, err)
			return
		}
		go s.ServeConn(conn)
	}
}

func (s *Server) ServeConn(conn net.Conn) {
	serverContext,err := NewServerContext(conn)
	if err != nil {
		log.Printf("rpc server: create server context error: %v", err)
		return
	}
	s.serveCodec(serverContext)
}

var invalidReply = struct{}{}

// serve a conn in 'sc'
func (s *Server) serveCodec(sc *ServerContext) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		// read and handle data
		req, err := s.readRequest(sc)

		if err != nil {
			if req == nil {
				break
			}
			s.sendResponse(sc, req.header, invalidReply, err.Error(), sending)
			continue
		}
		// debug
		// log.Printf("rpc server: read request success,request header is %v\n", req.header)
		wg.Add(1)
		// handle request
		go s.handleRequest(sc, req, sending, wg)
	}
	wg.Wait()
	_ = sc.Close()
}

type request struct {
	header *codec.RequestHeader // request header

	argv   reflect.Value // arg
	replyv reflect.Value // reply

	mtype   *methodType // method
	svc     *service    // service
	callErr error       // service method's error
}

func (s *Server) readRequest(sc *ServerContext) (*request, error) {
	// read request header
	header, err := sc.ReadRequestHeadr()
	if err != nil {
		return nil, err
	}
	// debug
	// log.Printf("rpc server: read request header success,header is %v\n", header)
	// init req
	req := &request{header: header}
	// get service and method
	req.svc, req.mtype, err = s.findService(header.ServiceMethod)
	if err != nil {
		return req, err
	}
	// init argv and replyv
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()
	argvi := req.argv.Interface()
	// argvi to it's pointer for assign
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	// read request body
	if err = sc.ReadRequestBody(argvi); err != nil {
		return req, errors.New("read request body error: " + err.Error())
	}
	// debug
	// log.Printf("rpc server: read request body success,argv is %v\n", req.argv)
	return req, nil
}

func (s *Server) sendResponse(sc *ServerContext, header *codec.RequestHeader, reply interface{}, errmsg string, sending *sync.Mutex) {
	resph := &codec.ResponseHeader{
		ServiceMethod: header.ServiceMethod,
		Seq:           header.Seq,
	}
	if errmsg != "" {
		resph.Error = errmsg
		reply = invalidReply
	}
	sending.Lock()
	defer sending.Unlock()
	if err := sc.WriteResponse(resph, reply); err != nil {
		log.Printf("rpc server: write response error: %v", err)
	}
}

// handle request should called in a goroutine
func (s *Server) handleRequest(sc *ServerContext, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	// timeout by channcel and time.After()
	done := make(chan struct{}, 1)
	// call service method
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		req.callErr = err
		done <- struct{}{}
	}()
	timeout := sc.opt.HandleTimeout
	// no timeout
	getErr := func(err error) string {
		if err == nil {
			return ""
		} else {
			return err.Error()
		}
	}
	if timeout == 0 {
		<-done
		if req.callErr != nil {
			s.sendResponse(sc, req.header, invalidReply, req.callErr.Error(), sending)
		}
		s.sendResponse(sc, req.header, req.replyv.Interface(), getErr(req.callErr), sending)
		return
	}
	select {
	case <-time.After(timeout):
		errmsg := fmt.Sprintf("rpc server(%s): request handle timeout: expect within %s", s.Addr, timeout)
		s.sendResponse(sc, req.header, invalidReply, errmsg, sending)
	case <-done:
		if req.callErr != nil {
			s.sendResponse(sc, req.header, invalidReply, req.callErr.Error(), sending)
		}
		s.sendResponse(sc, req.header, req.replyv.Interface(), req.callErr.Error(), sending)
	}
}

// http connect to rpc server
const (
	connected        = "200 Connected to myrpc"
	defaultRPCPath   = "/_myrpc_"
	dafaultDebugPath = "/debug/myrpc"
)

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println("rpc hijacking", req.RemoteAddr, ":", err.Error())
		_ = conn.Close()
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	s.ServeConn(conn)
}

func (s *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, s)
	http.Handle(dafaultDebugPath, debugHTTP{s})
	log.Println("rpc server debug path:", dafaultDebugPath)
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
