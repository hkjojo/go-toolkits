package kafka

import "encoding/json"

var DefaultCodec Codec = jsonCodec{}

type Codec interface {
	Marshal(interface{}) ([]byte, error)
}

type jsonCodec struct{}

// Marshal ...
func (c jsonCodec) Marshal(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}
