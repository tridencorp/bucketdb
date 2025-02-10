package db

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"os"
)

const(
	MaxIndexesPerFile = 10_000
	IndexSize         = 16
)

// Index will represent key in our database.
type Index struct {
	bucketId uint32  // 4 bytes
	size     uint32  // 4 bytes
	offset   uint64  // 8 bytes
}

type IndexFile struct {
	file *os.File

	// Maximum number of indexes per index file.
	maxNumber uint32
}

// Load index file.
func LoadIndexFile(coll *Collection) (*IndexFile, error) {
	path := coll.root + "/index.idx"
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, nil
	}

	indexFile := &IndexFile{file: file, maxNumber: MaxIndexesPerFile}
	return indexFile, nil
}

// Create an index for the given key/value and store it in the index file.
// This will allow us for faster lookups.
func (indexes *IndexFile) Add(key string, val []byte, offset uint64) error {	
	hash := HashKey(key)
	idx  := Index{bucketId: 1, size: uint32(len(val)), offset: offset}

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, idx)
	if err != nil {
		return err
	}

	pos := (hash % indexes.maxNumber) * IndexSize
	indexes.file.WriteAt(buf.Bytes(), int64(pos))
	return nil
}

// Hash key.
func HashKey(key string) uint32 {
  h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}
