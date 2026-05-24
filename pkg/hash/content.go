package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// OfBytes returns the hex-encoded SHA-256 of b.
func OfBytes(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// OfReader streams r through SHA-256 and returns the hex digest.
func OfReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// OfFile opens path and hashes its contents.
func OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return OfReader(f)
}

// ShardDir returns the two-level prefix directory for a content hash,
// e.g. "ab/cd" for hash "abcd1234...". Keeps directory entry counts sane.
func ShardDir(contentHash string) (l1, l2 string) {
	return contentHash[0:2], contentHash[2:4]
}
