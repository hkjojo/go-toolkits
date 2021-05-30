package utils

import (
	"fmt"
	"io"
	"strings"
)

// CSVBuffer ...
type CSVBuffer struct {
	cols []string
}

// NewCSVBuffer ...
func NewCSVBuffer() *CSVBuffer {
	return &CSVBuffer{
		cols: make([]string, 0, 8),
	}
}

// AddColumn ...
func (c *CSVBuffer) AddColumn(col ...interface{}) {
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

// FlushLine ...
func (c *CSVBuffer) FlushLine(w io.Writer) {
	w.Write([]byte(strings.Join(c.cols, ",") + "\r\n"))
	c.cols = c.cols[:0]
}

// WriteTitle ...
func (c *CSVBuffer) WriteTitle(w io.Writer) {
	w.Write([]byte("\xEF\xBB\xBF"))
}
