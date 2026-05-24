package io

import "sync"

const defaultBufSize = 4 << 20 // 4 MiB

// BufPool is a pool of reusable byte-slice buffers.
// Reusing buffers across requests avoids per-request GC pressure
// and keeps heap churn low during high-throughput transfers.
type BufPool struct {
	p sync.Pool
}

// NewBufPool creates a pool whose buffers are size bytes each.
func NewBufPool(size int) *BufPool {
	return &BufPool{
		p: sync.Pool{
			New: func() any {
				buf := make([]byte, size)
				return &buf
			},
		},
	}
}

// DefaultBufPool is a package-level pool with the default buffer size.
var DefaultBufPool = NewBufPool(defaultBufSize)

// Get retrieves a buffer from the pool.
func (bp *BufPool) Get() *[]byte {
	return bp.p.Get().(*[]byte)
}

// Put returns a buffer to the pool.
func (bp *BufPool) Put(buf *[]byte) {
	bp.p.Put(buf)
}
