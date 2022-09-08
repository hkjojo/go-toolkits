package kafka

import (
	"bytes"
	"encoding/gob"

	"github.com/golang/protobuf/proto"
)

type Codec interface {
	Encode(any) ([]byte, error)
	Decode([]byte, any) error
}

type codec struct{}

func (c codec) Encode(e any) (out []byte, err error) {
	vv, ok := e.(proto.Message)
	if ok {
		return proto.Marshal(vv)
	}

	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err = encoder.Encode(e); err != nil {
		return
	}
	return buffer.Bytes(), nil
}

func (c codec) Decode(d []byte, e any) error {
	vv, ok := e.(proto.Message)
	if ok {
		return proto.Unmarshal(d, vv)
	}

	data := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(data)
	return decoder.Decode(e)
}
