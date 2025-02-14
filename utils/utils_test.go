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
		{"6", args{"demo", "!demo*,*"}, false},
		{"7", args{"demo1", "*,!demo1"}, true},
		{"8", args{"demo", "real,demo"}, true},
		{"9", args{"real\\TEST-01", "demo\\*,real\\TEST*"}, true},
		{"10", args{"real\\TEST-01", "!real\\*,real\\TEST*"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchV2(t *testing.T) {
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
		{"5", args{"XAUUSD", "!XAU*,*XAU*"}, false},
		{"6", args{"demo", "!demo*,*"}, false},
		{"7", args{"demo1", "!demo1,*"}, false},
		{"8", args{"demo", "real,demo"}, true},
		{"9", args{"real\\TEST-01", "demo\\*,real\\TEST*"}, true},
		{"10", args{"real\\TEST-01", "!real\\*,real\\TEST*"}, false},
		{"11", args{"real\\TEST-01", "real\\*01"}, true},
		{"12", args{"XAUUSD", "!XAUUSD.a,*XAU*,!XAGUSD.a,*XAG*"}, true},
		{"13", args{"XAUUSD.b", "!XAUUSD.a,*XAU*,!XAGUSD.a,*XAG*"}, true},
		{"14", args{"XAGUSD.b", "!XAUUSD.a,*XAU*,!XAGUSD.a,*XAG*"}, true},
		{"15", args{"XAUUSD.a", "!XAUUSD.a,*XAU*,!XAGUSD.a,*XAG*"}, false},
		{"16", args{"XAGUSD.a", "!XAUUSD.a,*XAU*,!XAGUSD.a,*XAG*"}, false},
		{"17", args{"real\\TEST-01", "!real\\*01,!real\\*02,real\\*"}, false},
		{"18", args{"real\\TEST-01", "real\\*01,real\\*02"}, true},
		{"19", args{"hello", "!x*"}, true},
		{"20", args{"abc", "a*c"}, true},
		{"21", args{"abc", "a*b*c"}, true},
		{"22", args{"abc", "*b*"}, true},
		{"23", args{"abc", "a*"}, true},
		{"24", args{"abc", "*c"}, true},
		{"25", args{"abc", "*"}, true},
		{"26", args{"abc", "abc"}, true},
		{"27", args{"abc", "ab**c"}, true},
		{"28", args{"abc", "a**c"}, true},
		{"29", args{"abc", "a***c"}, true},
		{"30", args{"abc", "*a*b*c*"}, true},
		{"31", args{"abc", "a*b*c*d*"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchV2(tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkMatch2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchV2("XAUUSD", "!XAUUSD*,!XAU*,*XAU*")
	}
}

func BenchmarkMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("XAUUSD", "!XAUUSD*,!XAU*,*XAU*")
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

func TestRound(t *testing.T) {
	t.Log(Round(1.10268+0.0001, 5))
}
