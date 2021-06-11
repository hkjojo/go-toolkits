package utils

import (
	"bytes"
	"io"
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
