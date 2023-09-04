package myrpc

import (
	"myrpc/codec"
	"sync"
	"time"
	"encoding/binary"
	"errors"
)

// ============= Option =================
const MagicNumber = 0x3bef5c

type Option struct {
	//
	*sync.RWMutex

	MagicNumber int        // MagicNumber marks this's a myrpc request
	CodecType   codec.Type // client may choose different Codec to encode body

	// timeout 0 mean no limit
	ConnectTimeout time.Duration // 连接超时时间 int64
	HandleTimeout  time.Duration // call函数调用超时时间
}

var DefaultOption = &Option{
	RWMutex:        new(sync.RWMutex),
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: time.Second * 10,
	HandleTimeout:  time.Second * 0,
}

func (opt *Option) Marshal() []byte {
	opt.RLock()
	defer opt.RUnlock()
	
	dataLen := 4 * binary.MaxVarintLen64
	data := make([]byte, dataLen)
	idx := 0
	// write magic number
	idx += binary.PutVarint(data[idx:], int64(opt.MagicNumber))
	// write codec type
	idx += binary.PutVarint(data[idx:], int64(opt.CodecType))
	// write connect timeout
	idx += binary.PutVarint(data[idx:], int64(opt.ConnectTimeout))
	// write handle timeout
	idx += binary.PutVarint(data[idx:], int64(opt.HandleTimeout))
	return data[:idx]
	
}

func (opt *Option) Unmarshal(data []byte) (err error) {
	opt.Lock()
	defer opt.Unlock()
	if len(data) == 0 {
		err = errors.New("option unmarshal error: data is empty")
		return
	}

	// read magic number
	idx, n := 0, 0
	magicNumber, n := binary.Varint(data[idx:])
	if n < 0 {
		err = errors.New("option unmarshal error: magic number")
		return
	}
	idx += n
	// read codec type
	codecType, n := binary.Varint(data[idx:])
	if n < 0 {
		err = errors.New("option unmarshal error: codec type")
		return
	}
	idx += n
	// read connect timeout
	connectTimeout, n := binary.Varint(data[idx:])
	if n < 0 {
		err = errors.New("option unmarshal error: connect timeout")
		return
	}
	idx += n
	// read handle timeout
	handleTimeout, n := binary.Varint(data[idx:])
	if n < 0 {
		err = errors.New("option unmarshal error: handle timeout")
		return
	}
	idx += n

	opt.MagicNumber = int(magicNumber)
	opt.CodecType = codec.Type(codecType)
	opt.ConnectTimeout = time.Duration(connectTimeout)
	opt.HandleTimeout = time.Duration(handleTimeout)
	return
}

// ============= Option =================