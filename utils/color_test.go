package utils

import (
	"testing"
)

func TestToColor64(t *testing.T) {
	for value := range _colorNameMap {
		color64, ok := ToColor64(value)
		if !ok {
			t.Fatalf("to color64 fail:%s, color:%d", value, color64)
		}
		ok = ColorEq(uint(color64), value)
		if !ok {
			t.Fatalf("color eq fail:%s", value)
		}
	}
}
