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

// 性能测试用例
var benchmarkCases = []struct {
	name    string
	value   string
	pattern string
	desc    string
}{
	{
		name:    "SimpleMatch",
		value:   "XAUUSD",
		pattern: "XAUUSD",
		desc:    "简单精确匹配",
	},
	{
		name:    "WildcardPrefix",
		value:   "XAUUSD",
		pattern: "XAU*",
		desc:    "前缀通配符匹配",
	},
	{
		name:    "WildcardSuffix",
		value:   "XAUUSD",
		pattern: "*USD",
		desc:    "后缀通配符匹配",
	},
	{
		name:    "WildcardMiddle",
		value:   "XAUUSD",
		pattern: "X*U*D",
		desc:    "中间通配符匹配",
	},
	{
		name:    "ExclusionPattern",
		value:   "XAUUSD",
		pattern: "!XAUUSD*,*XAU*",
		desc:    "排除模式匹配",
	},
	{
		name:    "MultiplePatterns",
		value:   "XAUUSD",
		pattern: "!XAUUSD*,!XAU*,*XAU*",
		desc:    "多模式匹配",
	},
	{
		name:    "ComplexPattern",
		value:   "real\\TEST-01",
		pattern: "demo\\*,real\\TEST*,!real\\*01",
		desc:    "复杂模式匹配",
	},
	{
		name:    "LongString",
		value:   "very_long_symbol_name_with_many_characters",
		pattern: "*long*name*",
		desc:    "长字符串匹配",
	},
	{
		name:    "ManyWildcards",
		value:   "abcdefghijklmnopqrstuvwxyz",
		pattern: "a*b*c*d*e*f*g*h*i*j*k*l*m*n*o*p*q*r*s*t*u*v*w*x*y*z",
		desc:    "多通配符匹配",
	},
	{
		name:    "NoMatch",
		value:   "NOMATCH",
		pattern: "!NOMATCH*,MATCH*",
		desc:    "无匹配情况",
	},
	{
		name:    "RealSymbolMatch",
		value:   "Forex.e\\USDJPY.e",
		pattern: "Forex.e\\*SGD.e,Forex.e\\*USD.e,Forex.e\\*CAD.e,Forex.e\\*GBP.e,Forex.e\\*EUR.e,Forex.e\\*CHF.e,Forex.e\\*JPY.e,Forex.e\\*JPY.e,Forex.e\\*JPY.e,Forex.e\\*JPY.e",
		desc:    "真实情况",
	},
}

// Match 性能测试
func BenchmarkMatch(b *testing.B) {
	for _, tc := range benchmarkCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Match(tc.value, tc.pattern)
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

func TestRound(t *testing.T) {
	t.Log(Round(1.10268+0.0001, 5))
}

func TestMatchV3(t *testing.T) {
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
		{"19", args{"hello", "!x*"}, false},
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
		{"32", args{"", "*"}, false},
		{"33", args{"", "test"}, false},
		{"34", args{"test", ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("MatchV3() = %v, want %v", got, tt.want)
			}
		})
	}
}
