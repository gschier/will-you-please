package internal

import (
	"fmt"
	"github.com/logrusorgru/aurora"
	"io"
)

type prefixedWriter struct {
	w        io.Writer
	prefix   []byte
	wrote    int
	lastByte byte
}

func NewPrefixedWriter(w io.Writer, name string, color *aurora.Color) *prefixedWriter {
	prefix := ""

	if name != "" {
		prefixStr := fmt.Sprintf("[%s] ", name)
		prefix = prefixStr
		if color != nil {
			prefix = aurora.Colorize(prefixStr, *color).String()
		}
	}

	return &prefixedWriter{
		w:      w,
		prefix: []byte(prefix),
	}
}

func (p2 *prefixedWriter) DidWrite() bool {
	return p2.wrote > 0
}

func (p2 *prefixedWriter) Write(p []byte) (int, error) {
	// Split on newlines so we can prefix each one
	count := 0
	for _, b := range p {
		// If it's the first char or the last one was a newline, write a prefix
		if p2.wrote == 0 || p2.lastByte == '\n' || p2.lastByte == '\r' {
			_, _ = p2.w.Write(p2.prefix)
		}

		_, _ = p2.w.Write([]byte{b})
		p2.lastByte = b
		p2.wrote++
		count++
	}

	return count, nil
}
