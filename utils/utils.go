package utils

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"strings"
)

// StringInSlice ...
func StringInSlice(v string, list []string) bool {
	for _, k := range list {
		if k == v {
			return true
		}
	}
	return false
}

// IntInSlice ...
func IntInSlice(v int, list []int) bool {
	for _, k := range list {
		if k == v {
			return true
		}
	}
	return false
}

// Match pattern compatible MT wildcard pattern
// see test cases for usage
func Match(value, pattern string) bool {
	if pattern == "" {
		return false
	}
	for _, exp := range strings.Split(pattern, ",") {
		if len(exp) == 0 {
			continue
		}

		var reverse, ok bool
		if exp[:1] == "!" {
			exp = exp[1:]
			reverse = true
		}

		if exp != "*" {
			// replace regexp
			prefix := strings.HasPrefix(exp, "*")
			suffix := strings.HasSuffix(exp, "*")
			exp = strings.Replace(exp, "*", "", -1)
			switch {
			case prefix && !suffix:
				ok = strings.HasSuffix(value, exp)
			case !prefix && suffix:
				ok = strings.HasPrefix(value, exp)
			case prefix && suffix:
				ok = strings.Contains(value, exp)
			default:
				ok = value == exp
			}
		} else {
			ok = true
		}

		if reverse && ok {
			return false
			// ok = !ok
		}

		if ok {
			return true
		}
	}
	return false
}

// MatchIE match ignore case
func MatchIE(value, pattern string) bool {
	return Match(strings.ToLower(value), strings.ToLower(pattern))
}

// TODO: remove next version, use Match instead
// Any pattern is ||
func Any(value, pattern string) bool {
	if pattern == "" {
		return false
	}
	for _, exp := range strings.Split(pattern, ",") {
		if len(exp) == 0 {
			continue
		}

		var reverse, ok bool
		if exp[:1] == "!" {
			exp = exp[1:]
			reverse = true
		}

		if exp != "*" {
			// replace regexp
			prefix := strings.HasPrefix(exp, "*")
			suffix := strings.HasSuffix(exp, "*")
			exp = strings.Replace(exp, "*", "", -1)
			switch {
			case prefix && !suffix:
				ok = strings.HasSuffix(value, exp)
			case !prefix && suffix:
				ok = strings.HasPrefix(value, exp)
			case prefix && suffix:
				ok = strings.Contains(value, exp)
			default:
				ok = value == exp
			}
		} else {
			ok = true
		}

		if reverse {
			ok = !ok
		}
		if ok {
			return true
		}
	}
	return false
}

// TODO: remove next version, use Match instead
// AnyIE match ignore case, pattern is ||
func AnyIE(value, pattern string) bool {
	return Any(strings.ToLower(value), strings.ToLower(pattern))
}

func LineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

var MaxRange = 100

// RandPct ...
func RandPct(pct uint) bool {
	if pct == 0 {
		return false
	}
	if pct == uint(MaxRange) {
		return true
	}
	return RandInt(1, MaxRange) <= int(pct)
}

// RandInt [min,max]
func RandInt(min, max int) int {
	if max < min {
		min, max = max, min
	}

	if max == min {
		return min
	}

	return rand.Intn(max-min+1) + min
}

// RandFloat [min,max]
func RandFloat(min, max float64) float64 {
	if max < min {
		min, max = max, min
	}

	if max == min {
		return min
	}
	return min + rand.Float64()*(max-min)
}

// Round ...
func Round(f float64, digits int) float64 {
	p10 := math.Pow10(digits)
	p := f * p10
	return math.Round(p) / p10
}

func MatchV2(value, pattern string) bool {
	if pattern == "" {
		return false
	}

	// 分离排除规则和包含规则
	var excludePatterns, includePatterns []string

	for _, exp := range strings.Split(pattern, ",") {
		exp = strings.TrimSpace(exp)
		if exp == "" {
			continue
		}

		if strings.HasPrefix(exp, "!") {
			excludePatterns = append(excludePatterns, exp[1:])
		} else {
			includePatterns = append(includePatterns, exp)
		}
	}

	// 先检查排除规则（且逻辑）
	for _, exp := range excludePatterns {
		if wildcardMatch(value, exp) {
			return false
		}
	}

	// 后检查包含规则（或逻辑）
	for _, exp := range includePatterns {
		if wildcardMatch(value, exp) {
			return true
		}
	}

	// 特殊处理纯排除规则的情况
	return len(includePatterns) == 0 && len(excludePatterns) > 0
}

// 高性能通配符匹配（支持 * 在任意位置）
func wildcardMatch(s, pattern string) bool {
	// 空模式只匹配空字符串
	if pattern == "" {
		return s == ""
	}

	// 完全匹配星号
	if pattern == "*" {
		return true
	}

	// 拆解模式结构
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return s == pattern
	}

	// 检查前缀和后缀
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	if !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}

	// 检查中间部分
	start := len(parts[0])
	end := len(s) - len(parts[len(parts)-1])
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		index := strings.Index(s[start:end], part)
		if index == -1 {
			return false
		}
		start += index + len(part)
	}

	return true
}
