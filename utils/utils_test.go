package utils

import (
	"bufio"
	"os"
	"testing"
)

func BenchmarkLineCounter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		f, _ := os.OpenFile("color.go", os.O_CREATE|os.O_APPEND, 0666)
		bufReader := bufio.NewReader(f)
		_, err := LineCounter(bufReader)
		if err != nil {
			b.Error(err)
		}
		f.Close()
	}
}

func TestMatch(t *testing.T) {
	type args struct {
		value   string
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"1", args{"XAUUSD", "XAUUSD*"}, true},
		{"2", args{"XAUUSD", "!XAUUSD*"}, false},
		{"3", args{"XAUUSD", "XAU*"}, true},
		{"4", args{"XAUUSD", "!*XAU*"}, false},
		{"5", args{"XAUUSD", "!XAUUSD*,!XAU*,*XAU*"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAny(t *testing.T) {
	type args struct {
		value   string
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"1", args{"XAUUSD", "XAUUSD*"}, true},
		{"2", args{"XAUUSD", "!XAUUSD*"}, false},
		{"3", args{"XAUUSD", "XAU*"}, true},
		{"4", args{"XAUUSD", "!*XAU*"}, false},
		{"5", args{"XAUUSD", "!XAUUSD*,!XAU*,*XAU*"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Any(tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("Any() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloatDigits(t *testing.T) {
	tests := []struct {
		name string
		arg  float64
		want int
	}{
		{"case 1", 1.0, 0},
		{"case 2", 1, 0},
		{"case 3", 0.1, 1},
		{"case 4", 1.120, 2},
		{"case 5", 1.0000000001, 10},
		{"case 6", 2.220000000000000003, 2},
		{"case 7", 2.220000000000003, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FloatDigits(tt.arg); got != tt.want {
				t.Errorf("FloatDigits() = %v, want %v", got, tt.want)
			}
		})
	}
}
