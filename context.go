package myrpc

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"myrpc/codec"
	myrpcio "myrpc/io"
	"sync"
)

// == ServerContext ==
type ServerContext struct {
	bufc        *BufConn // bufc is used in serverCodec
	conn        io.ReadWriteCloser
	serverCodec codec.ServerCodec
	opt         *Option
}

func NewServerContext(conn io.ReadWriteCloser) (*ServerContext, error) {
	// read option
	sc := &ServerContext{
		bufc: NewBufConn(conn),
		conn: conn,
	}
	opt, err := sc.ReadOption()
	log.Println(opt)
	if err != nil {
		err := fmt.Errorf("NewServerContext: read option error: %v", err)
		return nil, err
	}
	sc.opt = opt
	newServerCodecFunc := codec.NewServerCodecFuncMap[opt.CodecType]
	if newServerCodecFunc == nil {
		err := fmt.Errorf("NewServerContext: invalid codec type %s", opt.CodecType)
		return nil, err
	}
	sc.serverCodec = newServerCodecFunc(sc.bufc)
	return sc, nil
}

func (sc *ServerContext) ReadOption() (*Option, error) {
	data, err := myrpcio.RecvFrame(sc.bufc)
	if err != nil {
		err = fmt.Errorf("read option error(recvFrame): %v", err)
		return nil, err
	}
	opt := &Option{RWMutex: new(sync.RWMutex)}
	if err = opt.Unmarshal(data); err != nil {
		err = fmt.Errorf("read option error(unmarshal): %v", err)
		return nil, err
	}
	// compare magic number
	if opt.MagicNumber != MagicNumber {
		err = fmt.Errorf("invalid magic number %x", opt.MagicNumber)
		return nil, err
	}
	return opt, nil
}

func (sc *ServerContext) ReadRequestHeadr() (*codec.RequestHeader, error) {
	var header codec.RequestHeader
	if err := sc.serverCodec.ReadRequestHeadr(&header); err != nil {
		return nil, err
	}
	return &header, nil
}

func (sc *ServerContext) ReadRequestBody(body interface{}) error {
	if err := sc.serverCodec.ReadRequestBody(body); err != nil {
		return err
	}
	return nil
}

func (sc *ServerContext) WriteResponse(header *codec.ResponseHeader, body interface{}) error {
	if err := sc.serverCodec.WriteResponse(header, body); err != nil {
		return fmt.Errorf("ServerContext: %v", err)
	}
	if err := sc.bufc.Flush(); err != nil {
		return fmt.Errorf("ServerContext: buf flush error:%v", err)
	}
	return nil
}

func (sc *ServerContext) Close() error {
	return sc.serverCodec.Close()
}

// == ServerContext End ==

// == ClientContext ==
type ClientContext struct {
	bufc        *BufConn
	conn        io.ReadWriteCloser
	clientCodec codec.ClientCodec
	opt         *Option
}

func NewClientContext(conn io.ReadWriteCloser, opt *Option) (*ClientContext, error) {
	newClientCodecFunc := codec.NewClientCodecFuncMap[opt.CodecType]
	if newClientCodecFunc == nil {
		err := fmt.Errorf("NewClientContext: invalid codec type %s", opt.CodecType)
		return nil, err
	}
	bufc := NewBufConn(conn)
	cc := &ClientContext{
		bufc:        bufc,
		conn:        conn,
		opt:         opt,
		clientCodec: newClientCodecFunc(bufc),
	}
	// send option to server
	if err := cc.WriteOption(); err != nil {
		err = fmt.Errorf("NewClientContext: write option error: %v", err)
		return nil, err
	}
	return cc, nil
}

func (cc *ClientContext) WriteOption() (err error) {
	if err = myrpcio.SendFrame(cc.bufc, cc.opt.Marshal()); err != nil {
		err = fmt.Errorf("ClientContext: write option error: %v", err)
		return
	}
	cc.bufc.Flush()
	return
}

func (cc *ClientContext) ReadResponseHeader(resph *codec.ResponseHeader) error {
	if err := cc.clientCodec.ReadResponseHeader(resph); err != nil {
		err = fmt.Errorf("ClientContext: %v", err)
		return err
	}
	return nil
}

func (cc *ClientContext) ReadResponseBody(body interface{}) error {
	if err := cc.clientCodec.ReadResponseBody(body); err != nil {
		err = fmt.Errorf("ClientContext: %v", err)
		return err
	}
	return nil
}

func (cc *ClientContext) WriteRequest(header *codec.RequestHeader, body interface{}) error {
	if err := cc.clientCodec.WriteRequest(header, body); err != nil {
		err = fmt.Errorf("ClientContext: %v", err)
		return err
	}
	// flush
	if err := cc.bufc.Flush(); err != nil {
		err = fmt.Errorf("ClientContext: buf flush error:%v", err)
		return err
	}
	return nil
}

func (cc *ClientContext) Close() error {
	return cc.clientCodec.Close()
}

// == ClientContext End ==

// == BufConn ==
type BufConn struct {
	*bufio.ReadWriter // brw is a buffered reader and writer of conn
	conn              io.ReadWriteCloser
}

func NewBufConn(conn io.ReadWriteCloser) *BufConn {
	return &BufConn{
		ReadWriter: bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		conn:       conn,
	}
}

func (c *BufConn) Close() error {
	_ = c.Flush()
	c.Writer.Reset(nil)
	c.Reader.Reset(nil)
	return c.conn.Close()
}

// == BufConn End ==
