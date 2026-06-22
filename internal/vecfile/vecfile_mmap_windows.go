//go:build windows

package vecfile

import (
	"os"
)

// mmapFile reads the file into memory as a fallback on Windows.
// Windows does not support syscall.Mmap, so we use a regular file read.
func mmapFile(fd int, size int) ([]byte, error) {
	// fd is not used on Windows; reopen by reading the whole file.
	// The caller ensures the file is already open, so we read from os.File.
	f := os.NewFile(uintptr(fd), "")
	if f == nil {
		return nil, os.ErrInvalid
	}
	// Seek to beginning before reading
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	_, err := f.ReadAt(data, 0)
	return data, err
}

// munmapFile is a no-op on Windows since data is a regular byte slice.
func munmapFile(data []byte) error {
	return nil
}