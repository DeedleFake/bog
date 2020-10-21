// Package markdown contains some utilities for dealing with markdown
// wherever Blackfriday falls short.
package markdown

import (
	"io"

	"github.com/russross/blackfriday/v2"
)

// An errWriter is a writer that writes until a single error has been
// returned by the underlying writer, at which point it simply returns
// that error.
type errWriter struct {
	w   io.Writer
	err error
}

func (w *errWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}

	n, err := w.w.Write(data)
	w.err = err
	return n, err
}

// Render renders markdown to an io.Writer. It essentially replicates
// the internals of blackfriday.Run because, for some bizarre reason,
// the actual rendering logic is not exported anywhere other than
// that.
func Render(w io.Writer, node *blackfriday.Node, renderer blackfriday.Renderer) error {
	ew := errWriter{w: w}

	renderer.RenderHeader(&ew, node)
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return renderer.RenderNode(&ew, node, entering)
	})
	renderer.RenderFooter(&ew, node)

	return ew.err
}
