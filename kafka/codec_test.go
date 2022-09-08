package kafka

import (
	"fmt"
	"testing"
)

func TestCodec_Decode(t *testing.T) {
	c := codec{}

	// p := &Person{Name: "Hale"}
	// data, err := c.Encode(p)
	// p := &proto.HelloReq{Name: "Hello"}
	// data, err := c.Encode(p)
	// data, err := c.Encode("Hello")
	data, err := c.Encode(9)
	if err != nil {
		panic(err)
	}
	fmt.Println(data)
	fmt.Println(string(data))

	// var pp proto.HelloReq
	// var pp Person
	// var pp string
	var pp int32
	if err = c.Decode(data, &pp); err != nil {
		panic(err)
	}
	fmt.Println(pp)
}
