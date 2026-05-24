package io

import (
	"fmt"
	"io"
	"net"
	"os"
)

// SendFile transfers the contents of src directly to dst using the kernel's
// sendfile(2) syscall on Linux, avoiding a userspace copy.
// Falls back to io.CopyBuffer on platforms that don't support it.
//
// dst must be a *net.TCPConn (or implement io.ReaderFrom) for the fast path.
func SendFile(dst io.Writer, src *os.File) (int64, error) {
	// net.TCPConn implements io.ReaderFrom; the Go runtime routes that
	// to sendfile(2) on Linux automatically.
	if rf, ok := dst.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(src)
		if err != nil {
			return n, fmt.Errorf("sendfile: %w", err)
		}
		return n, nil
	}

	// Fallback: buffered copy using a pooled buffer.
	bufp := DefaultBufPool.Get()
	defer DefaultBufPool.Put(bufp)
	return io.CopyBuffer(dst, src, *bufp)
}

// CopyBuffered copies from src to dst using a pooled buffer.
// Use this for non-file sources (e.g. HTTP request bodies).
func CopyBuffered(dst io.Writer, src io.Reader) (int64, error) {
	bufp := DefaultBufPool.Get()
	defer DefaultBufPool.Put(bufp)
	n, err := io.CopyBuffer(dst, src, *bufp)
	if err != nil {
		return n, fmt.Errorf("buffered copy: %w", err)
	}
	return n, nil
}

// OpenDirect opens a file with O_DIRECT on Linux (bypasses page cache).
// On other platforms it falls back to a normal open, so dev/test on macOS works.
func OpenDirect(path string, flag int, perm os.FileMode) (*os.File, error) {
	return openDirect(path, flag, perm)
}

// TCPConn attempts to unwrap w down to a *net.TCPConn.
// Returns nil if w is not (or does not wrap) a TCP connection.
func TCPConn(w io.Writer) *net.TCPConn {
	type unwrapper interface{ Unwrap() io.Writer }
	for {
		if tc, ok := w.(*net.TCPConn); ok {
			return tc
		}
		u, ok := w.(unwrapper)
		if !ok {
			return nil
		}
		w = u.Unwrap()
	}
}
