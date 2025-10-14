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

// Match 与MT5匹配逻辑一致
// 参数:
//
//	value: 要检查的字符串
//	pattern: 掩码列表，用逗号分隔，支持通配符 '*' 和排除符 '!'
//
// 返回值:
//
//	bool: 如果字符串匹配掩码列表则返回true，否则返回false
//
// 掩码规则:
//   - '*' 匹配任意字符序列
//   - '!' 前缀表示排除该模式
//   - 多个掩码用逗号分隔
//   - 空格会被忽略
//
// 性能特点:
//   - 零内存分配
//   - 针对常见模式进行特殊优化
//   - 使用迭代算法避免递归开销
func Match(value, pattern string) bool {
	// 检查输入参数
	if pattern == "" || value == "" {
		return false
	}

	found := false
	// 优化：避免strings.Split，直接遍历字符串
	start := 0
	for i := 0; i <= len(pattern); i++ {
		if i == len(pattern) || pattern[i] == ',' {
			// 提取当前token
			token := pattern[start:i]
			start = i + 1

			// 去除前后空格
			tokenStart, tokenEnd := 0, len(token)
			for tokenStart < tokenEnd && token[tokenStart] == ' ' {
				tokenStart++
			}
			for tokenEnd > tokenStart && token[tokenEnd-1] == ' ' {
				tokenEnd--
			}

			if tokenStart == tokenEnd {
				continue
			}

			mask := token[tokenStart:tokenEnd]

			// 检查排除模式 (!)
			if len(mask) > 0 && mask[0] == '!' {
				if MatchSingle(value, mask[1:]) {
					return false // 如果匹配排除模式，直接返回false
				}
			} else {
				// 检查包含模式
				if MatchSingle(value, mask) {
					found = true
				}
			}
		}
	}

	return found
}

// MatchSingle 单个匹配
func MatchSingle(value, expr string) bool {
	// 检查输入参数
	if expr == "" || value == "" {
		return false
	}

	// 特殊处理：完全匹配星号
	if expr == "*" {
		return true
	}

	// 针对常见模式进行优化
	switch {
	case !strings.Contains(expr, "*"):
		// 没有通配符，直接字符串比较
		return value == expr
	case strings.HasPrefix(expr, "*") && strings.HasSuffix(expr, "*") && !strings.Contains(expr[1:len(expr)-1], "*"):
		// 模式：*text*，使用strings.Contains
		return strings.Contains(value, expr[1:len(expr)-1])
	case strings.HasPrefix(expr, "*") && !strings.Contains(expr[1:], "*"):
		// 模式：*text，使用strings.HasSuffix
		return strings.HasSuffix(value, expr[1:])
	case strings.HasSuffix(expr, "*") && !strings.Contains(expr[:len(expr)-1], "*"):
		// 模式：text*，使用strings.HasPrefix
		return strings.HasPrefix(value, expr[:len(expr)-1])
	default:
		// 复杂模式，使用迭代算法
		return matchIterative(value, expr)
	}
}

// matchIterative 高性能迭代匹配算法
// 使用两个指针和回溯机制，避免递归调用
// 时间复杂度：O(m*n)，空间复杂度：O(1)
func matchIterative(value, expr string) bool {
	vLen, eLen := len(value), len(expr)
	vIdx, eIdx := 0, 0
	backtrackV, backtrackE := -1, -1

	for vIdx < vLen {
		if eIdx < eLen && expr[eIdx] == '*' {
			// 遇到通配符，记录回溯点
			backtrackV = vIdx
			backtrackE = eIdx
			eIdx++
		} else if eIdx < eLen && expr[eIdx] == value[vIdx] {
			// 字符匹配
			vIdx++
			eIdx++
		} else if backtrackE != -1 {
			// 回溯到上一个通配符位置
			backtrackV++
			vIdx = backtrackV
			eIdx = backtrackE + 1
		} else {
			// 无法匹配
			return false
		}
	}

	// 跳过表达式末尾的通配符
	for eIdx < eLen && expr[eIdx] == '*' {
		eIdx++
	}

	return eIdx == eLen
}

// MatchIE 功能与Match相同，但不区分大小写
func MatchIE(value, pattern string) bool {
	return Match(strings.ToLower(value), strings.ToLower(pattern))
}

// Any pattern is ||
// NOTE: Deprecated, use Match instead
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

// AnyIE match ignore case, pattern is ||
// NOTE: Deprecated, use MatchCaseInsensitive instead
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
