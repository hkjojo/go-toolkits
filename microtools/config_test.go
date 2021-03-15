package microtools

import (
	"testing"
)

type kv struct {
	Data string
}

func Test_Put(t *testing.T) {
	InitSource(WithFrom("consul://kvTest"))

	err := ConfigPut(&kv{
		Data: "put test",
	}, "123")
	if err != nil {
		t.Error(err)
	}
}

func Test_ConfigGet(t *testing.T) {
	type args struct {
		x    interface{}
		path []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConfigGet(tt.args.x, tt.args.path...); (err != nil) != tt.wantErr {
				t.Errorf("ConfigGet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
