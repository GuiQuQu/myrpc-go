package codec

import "io"

type Header struct {
	ServiceMethod string // format "Service.Method"
	Seq           uint64 // sequence number chosen by client
	Error         string // error info
}

type ServerCodec interface {
	io.Closer
	ReadRequestHeadr(*RequestHeader) error
	ReadRequestBody(interface{}) error
	WriteResponse(*ResponseHeader, interface{}) error
}

type ClientCodec interface {
	io.Closer
	WriteRequest(*RequestHeader, interface{}) error
	ReadResponseHeader(*ResponseHeader) error
	ReadResponseBody(interface{}) error
}

type NewServerCodecFunc func(io.ReadWriteCloser) ServerCodec
type NewClientCodecFunc func(io.ReadWriteCloser) ClientCodec

type Type int
const (
	UnsupportType Type = iota
	GobType 
	JsonType
)


func (t Type) String() string {
	switch t {
	case GobType:
		return "application/gob"
	case JsonType:
		return "application/json"
	default:
		return "unsupport type"
	}
}


var NewServerCodecFuncMap map[Type]NewServerCodecFunc
var NewClientCodecFuncMap map[Type]NewClientCodecFunc

func init() {
	NewServerCodecFuncMap = make(map[Type]NewServerCodecFunc)
	NewClientCodecFuncMap = make(map[Type]NewClientCodecFunc)
	NewServerCodecFuncMap[GobType] = NewServerGobCodec
	NewClientCodecFuncMap[GobType] = NewClientGobCodec
}
