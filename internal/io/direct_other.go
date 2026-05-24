//go:build !linux

package io

import "os"

func openDirect(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm)
}
