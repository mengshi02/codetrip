//go:build !windows

package vecfile

import (
	"syscall"
)

// mmapFile memory-maps a file using syscall.Mmap (Unix).
func mmapFile(fd int, size int) ([]byte, error) {
	return syscall.Mmap(fd, 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
}

// munmapFile unmaps a previously mmap'd byte slice (Unix).
func munmapFile(data []byte) error {
	return syscall.Munmap(data)
}