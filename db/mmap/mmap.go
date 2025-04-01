package mmap

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type Mmap struct {
	file   *os.File
	data   []byte
	Offset int
}

// Mmap file.
func Open(file *os.File, size, flags int) (*Mmap, error) {
	if flags == 0 {
		flags = unix.PROT_READ | unix.PROT_WRITE
	}

	// Use file size if necessary.
	if size == 0 {
		info, _ := file.Stat()
		size = int(info.Size())
	}

	data, err := unix.Mmap(int(file.Fd()), 0, size, flags, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	mmap := &Mmap{file: file, data: data, Offset: 0}
	return mmap, nil
}

// Sync data.
func (m *Mmap) Sync() error {
	return unix.Msync(m.data, unix.MS_SYNC)
}

// Write to mmaped file.
func (m *Mmap) Write(bytes []byte) int {
	n := copy(m.data[m.Offset:], bytes)
	m.Offset += len(bytes)
	return n
}

// Read n bytes from mmaped file.
func (m *Mmap) Read(n int) ([]byte, error) {
	data := make([]byte, n)

	n = copy(data, m.data)
	if n != len(data) {
		return data, fmt.Errorf("Mmap should read %d bytes, got only %d", len(data), n)
	} 

	return data, nil
}

// Resize the underlying file.
// TODO: Set offset properly.
func (m *Mmap) Resize(size int64) error {
	// Let's sync data before unmapping the file.
	err := m.Sync()
	if err != nil {
		return err
	}

	// To be safe we must unmap file before resizing.
	err = unix.Munmap(m.data)
	if err != nil {
		return err
	}

	// Resize the file.
	err = m.file.Truncate(size)
	if err != nil {
		return err
	}

	// Mmap file again.
	mmap, err := Open(m.file, int(size), 0)
	if err != nil {
		return err
	}

	// Assign new mapping.
	*m = *mmap
	return nil
}
