// Package bufpool provides a shared pool of buffers.
package bufpool

import (
	"bytes"
	"sync"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Get retrieves a buffer from the pool.
func Get() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

// Put resets a buffer and places it into the pool.
func Put(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}
