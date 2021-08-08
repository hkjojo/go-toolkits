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

// Match pattern is &&
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

		if reverse {
			ok = !ok
		}
		if !ok {
			return false
		}
	}
	return true
}

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
