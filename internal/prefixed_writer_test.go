package internal_test

import (
	"github.com/gschier/wyp/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPrefixedWriter_Write(t *testing.T) {
	t.Run("works with single line", func(t *testing.T) {
		w := newWriter()
		pw := internal.NewPrefixedWriter(w, "name", nil)
		_, _ = pw.Write([]byte("Hello"))
		assert.Contains(t, w.s, "[name] ")
	})

	t.Run("works with multiple lines", func(t *testing.T) {
		w := newWriter()
		pw := internal.NewPrefixedWriter(w, "name", nil)
		_, _ = pw.Write([]byte("First line\nSecond line\n"))
		assert.Equal(t, "[name] First line\n[name] Second line\n", w.s)
	})

	t.Run("doesn't prefix if last line didn't end in newline", func(t *testing.T) {
		w := newWriter()
		pw := internal.NewPrefixedWriter(w, "name", nil)
		_, _ = pw.Write([]byte("First line "))
		_, _ = pw.Write([]byte("More stuff"))
		assert.Equal(t, "[name] First line More stuff", w.s)
	})

	t.Run("should prefix carriage returns", func(t *testing.T) {
		w := newWriter()
		pw := internal.NewPrefixedWriter(w, "name", nil)
		_, _ = pw.Write([]byte("First line\r"))
		_, _ = pw.Write([]byte("Overwrite first line"))
		assert.Equal(t, "[name] First line\r[name] Overwrite first line", w.s)
	})

	t.Run("should prefix carriage returns same line", func(t *testing.T) {
		w := newWriter()
		pw := internal.NewPrefixedWriter(w, "name", nil)
		_, _ = pw.Write([]byte("First line\rOverwrite\n"))
		assert.Equal(t, "[name] First line\r[name] Overwrite\n", w.s)
	})
}

type writer struct {
	s string
}

func newWriter() *writer {
	return &writer{s: ""}
}

func (w *writer) Write(p []byte) (n int, err error) {
	w.s += string(p)
	return len(p), nil
}
