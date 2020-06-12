package microtools

import "testing"

type kv struct {
	Data string
}

func Test_Put(t *testing.T) {
	InitSource(WithFrom("consul://kvTest"))

	err := Put(&kv{
		Data: "put test",
	}, "123")
	if err != nil {
		t.Error(err)
	}
}
