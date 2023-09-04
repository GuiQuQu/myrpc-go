package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	rwc    io.ReadWriteCloser
	encBuf *bufio.Writer
	decBuf *bufio.Reader
	dec    *gob.Decoder
	enc    *gob.Encoder
	closed bool
}

var _ ServerCodec = (*GobCodec)(nil)
var _ ClientCodec = (*GobCodec)(nil)

func NewServerGobCodec(rwc io.ReadWriteCloser) ServerCodec {
	bufw := bufio.NewWriter(rwc)
	bufr := bufio.NewReader(rwc)
	return &GobCodec{
		rwc:    rwc,
		encBuf: bufw,
		decBuf: bufr,
		dec:    gob.NewDecoder(bufr),
		enc:    gob.NewEncoder(bufw),
	}
}

func NewClientGobCodec(rwc io.ReadWriteCloser) ClientCodec {
	bufw := bufio.NewWriter(rwc)
	bufr := bufio.NewReader(rwc)
	return &GobCodec{
		rwc:    rwc,
		encBuf: bufw,
		decBuf: bufr,
		dec:    gob.NewDecoder(bufr),
		enc:    gob.NewEncoder(bufw),
	}
}

// buf缓冲问题导致gob读空,从而导致报错
func (c *GobCodec) ReadRequestHeadr(header *RequestHeader) error {
	return c.dec.Decode(header)
}

func (c *GobCodec) ReadRequestBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) ReadResponseHeader(header *ResponseHeader) error {
	return c.dec.Decode(header)
}

func (c *GobCodec) ReadResponseBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) WriteResponse(header *ResponseHeader, body interface{}) (err error) {
	if err = c.enc.Encode(header); err != nil {
		if c.encBuf.Flush() == nil {
			log.Println("rpc codec(write response): gob error encoding header:", err)
			c.Close()
		}
		return
	}
	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			log.Println("rpc codec(write response): gob error encoding body:", err)
			c.Close()
		}
		return
	}
	return c.encBuf.Flush()
}

func (c *GobCodec) WriteRequest(header *RequestHeader, body interface{}) (err error) {
	if err = c.enc.Encode(header); err != nil {
		if c.encBuf.Flush() == nil {
			log.Println("rpc codec(write request): gob error encoding header:", err)
			c.Close()
		}
		return
	}
	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			log.Println("rpc codec(write request): gob error encoding body:", err)
			c.Close()
		}
		return
	}
	return c.encBuf.Flush()
}

func (c *GobCodec) Close() error {
	// 保证只关闭一次
	if c.closed {
		return nil
	}
	c.closed = true
	c.decBuf.Reset(nil)
	c.encBuf.Reset(nil)
	return c.rwc.Close()
}
