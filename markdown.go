package main

import (
	"io"

	"github.com/russross/blackfriday/v2"
)

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

// RenderMarkdown renders markdown to an io.Writer. It essentially
// replicates the internals of blackfriday.Run because, for some
// bizarre reason, the actual rendering logic is not exported anywhere
// other than that.
func RenderMarkdown(w io.Writer, node *blackfriday.Node, renderer blackfriday.Renderer) error {
	ew := errWriter{w: w}

	renderer.RenderHeader(&ew, node)
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return renderer.RenderNode(&ew, node, entering)
	})
	renderer.RenderFooter(&ew, node)

	return ew.err
}
