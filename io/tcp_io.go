package myrpcio

import (
	"encoding/binary"
	"io"
	"net"
)

type ReadrAndByteReader interface {
	io.Reader
	io.ByteReader
}

// ==== io method ====
func SendFrame(w io.Writer, data []byte) (err error) {
	var size [binary.MaxVarintLen64]byte

	if len(data) == 0 {
		n := binary.PutUvarint(size[:], uint64(0))
		if err = write(w,size[:n]); err != nil {
			return
		}
	}
	// write data size
	n := binary.PutUvarint(size[:], uint64(len(data)))
	if err = write(w,size[:n]); err != nil {
		return
	}
	if err = write(w, data); err != nil {
		return
	}
	return
}

func write(w io.Writer, data []byte) error {
	for index := 0; index < len(data); {
		n, err := w.Write(data[index:])
		if _, ok := err.(net.Error); !ok {
			return err
		}
		index += n
	}
	return nil
}

func RecvFrame(r ReadrAndByteReader) (data []byte, err error) {
	size, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	// read data
	if size != 0 {
		data = make([]byte, size)
		if err = read(r, data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func read(r io.Reader, data []byte) error {
	for index := 0; index < len(data); {
		n, err := r.Read(data[index:])
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				return err
			}
		}
		index += n
	}
	return nil
}