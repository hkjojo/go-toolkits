package csv

import (
	"fmt"
	"io"
	"strings"
)

// Buffer ...
type Buffer struct {
	cols []string
}

// NewBuffer ...
func NewBuffer() *Buffer {
	return &Buffer{
		cols: make([]string, 0, 8),
	}
}

// AddColumn ...
func (c *Buffer) AddColumn(col ...interface{}) {
	if len(col) == 0 {
		c.cols = append(c.cols, "\t")
		return
	}

	var s string
	for _, v := range col {
		s += fmt.Sprintf("%v", v)
	}

	c.cols = append(c.cols, s)
}

// AddRow ...
func (c *Buffer) AddRow(cols ...interface{}) {
	if len(cols) == 0 {
		c.cols = append(c.cols, "\r\n")
		return
	}

	for _, col := range cols {
		value := fmt.Sprintf("%v", col)
		if value == "" {
			c.cols = append(c.cols, "\t")
			continue
		}
		c.cols = append(c.cols, value)
	}
}

// FlushLine ...
func (c *Buffer) FlushLine(w io.Writer) {
	w.Write([]byte(strings.Join(c.cols, ",") + "\r\n"))
	c.cols = c.cols[:0]
}

// WriteTitle ...
func (c *Buffer) WriteTitle(w io.Writer, titles ...string) {
	w.Write([]byte("\xEF\xBB\xBF"))
	if len(titles) != 0 {
		w.Write([]byte(strings.Join(titles, ",") + "\r\n"))
	}
}
