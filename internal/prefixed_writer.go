package internal


import (
	"bytes"
	"fmt"
	"github.com/logrusorgru/aurora"
	"io"
)

type prefixedWriter struct {
	w      io.Writer
	prefix string
	wrote  int
}

func newPrefixedWriter(w io.Writer, name string, color aurora.Color) *prefixedWriter {
	prefix := ""

	if name != "" {
		prefixStr := fmt.Sprintf("[%s] ", name)
		prefix = aurora.Colorize(prefixStr, color).String()
	}

	return &prefixedWriter{
		w:      w,
		prefix: prefix,
	}
}

func (p2 prefixedWriter) DidWrite() bool {
	return p2.wrote > 0
}

func (p2 prefixedWriter) Write(p []byte) (int, error) {
	// Split on newlines so we can prefix each one
	lines := bytes.Split(p, []byte{'\n'})
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		line := append(l, '\n')
		_, _ = p2.w.Write(append([]byte(p2.prefix), line...))
	}

	n := len(p)
	p2.wrote += n

	return n, nil
}
