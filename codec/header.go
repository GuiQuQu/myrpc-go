package codec

// request header
type RequestHeader struct {
	ServiceMethod string // format "Service.Method"
	Seq           uint64 // sequence number chosen by client
}

// response header
type ResponseHeader struct {
	ServiceMethod string // echoes that of the Request
	Seq           uint64 // echoes that of the request
	Error         string // error
}

// marshal and unmarshal (request header and response header)
// var UnmarshalError = errors.New("codec: header unmarshal error")

// const (
// 	MaxHeaderLen = binary.MaxVarintLen64 * 3
// )

// func (r *RequestHeader) Marshal() []byte {
// 	r.RLock()
// 	defer r.RUnlock()
// 	idx := 0
// 	headerLen := len(r.ServiceMethod) + MaxHeaderLen
// 	header := make([]byte, headerLen)
// 	// write serviceMethod string
// 	idx += writeString(header[idx:], r.ServiceMethod)
// 	// write seq
// 	idx += binary.PutUvarint(header[idx:], r.Seq)
// 	// body length
// 	idx += binary.PutUvarint(header[idx:], r.BodyLen)
// 	return header[:idx]
// }

// func (r *RequestHeader) Unmarshal(data []byte) (err error) {
// 	r.Lock()
// 	defer r.Unlock()
// 	if len(data) == 0 {
// 		err = UnmarshalError
// 		return
// 	}

// 	// binary wrong will panic
// 	defer func() {
// 		if err := recover(); err != nil {
// 			err = UnmarshalError
// 		}
// 	}()

// 	idx, size := 0, 0
// 	// read serviceMethod string
// 	r.ServiceMethod, size = readString(data[idx:])
// 	idx += size

// 	// read seq
// 	r.Seq, size = binary.Uvarint(data[idx:])
// 	idx += size

// 	// read body length
// 	r.BodyLen, size = binary.Uvarint(data[idx:])
// 	idx += size
// 	return nil
// }

// func (r *ResponseHeader) Marshal() []byte {
// 	r.RLock()
// 	defer r.RUnlock()
// 	idx := 0
// 	headerLen := len(r.ServiceMethod) + MaxHeaderLen
// 	header := make([]byte, headerLen)
// 	// write serviceMethod string
// 	idx += writeString(header[idx:], r.ServiceMethod)
// 	// write seq
// 	idx += binary.PutUvarint(header[idx:], r.Seq)
// 	// body length
// 	idx += binary.PutUvarint(header[idx:], r.BodyLen)
// 	return header[:idx]
// }

// func (r *ResponseHeader) Unmarshal(data []byte) (err error) {
// 	r.Lock()
// 	defer r.Unlock()
// 	if len(data) == 0 {
// 		err = UnmarshalError
// 		return
// 	}

// 	// binary wrong will panic
// 	defer func() {
// 		if err := recover(); err != nil {
// 			err = UnmarshalError
// 		}
// 	}()

// 	idx, size := 0, 0
// 	// read serviceMethod string
// 	r.ServiceMethod, size = readString(data[idx:])
// 	idx += size

// 	// read seq
// 	r.Seq, size = binary.Uvarint(data[idx:])
// 	idx += size

// 	// read body length
// 	r.BodyLen, size = binary.Uvarint(data[idx:])
// 	idx += size
// 	return nil
// }

// func readString(data []byte) (string, int) {
// 	idx := 0
// 	length, size := binary.Uvarint(data)
// 	idx += size
// 	str := string(data[idx : idx+int(length)])
// 	idx += len(str)
// 	return str, idx
// }

// func writeString(data []byte, str string) int {
// 	idx := 0
// 	idx += binary.PutUvarint(data, uint64(len(str)))
// 	copy(data[idx:], str)
// 	idx += len(str)
// 	return idx
// }



