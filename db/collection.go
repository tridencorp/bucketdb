package db

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync/atomic"
)

type Collection struct {
	file    *os.File
	bucket  *Bucket
	indexes *IndexFile

	// Collection root directory.
	root string

	// We will be using atomic.Swap() for each key.
	// In combination with WriteAt, it should give
	// us the ultimate concurrent writes.
	offset atomic.Int64
}

// Key structure that represents each key stored in collection.
// TODO: Maybe better naming will be Record?
type Key struct {
	data []byte
	// Key size
	size uint32
}

func NewKey(val []byte) *Key {
	return &Key{val, uint32(len(val))}
}

// Encode key to bytes.
func (key *Key) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Encode size.
	err := binary.Write(buf, binary.LittleEndian, key.size)
	if err != nil {
		return nil, err
	}

	// Add key data.
	_, err = buf.Write(key.data)
	if err != nil {
		return nil, err		
	}

	return buf.Bytes(), nil
}

// Open the collection. If it doesn't exist,
// create it with default values.
func (db *DB) Collection(name string) (*Collection, error) {
	// Build collection path.
	path := db.root + CollectionsPath + name + "/1.bucket"
	dir  := filepath.Dir(path)

	// Create directory structure. Do nothing if it already exist.
	if err := os.MkdirAll(dir, 0755)
	err != nil {
		return nil, err
	}

	// Open collection bucket file.
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	bucket, err := OpenBucket(dir + "/1.bucket")
	if err != nil {
		return nil, err
	}

	indexes, err  := LoadIndexFile(dir)
	if err != nil {
		return nil, err
	}

	coll := &Collection{
		file:    file, 
		bucket:  bucket, 
		root:    dir,
		indexes: indexes,
	}

	// TODO: because of file truncation we should track current 
	// data size and set our initial offset based on it.
	// offset, err := file.Seek(0, io.SeekEnd)
	coll.offset.Store(0)

	return coll, nil
}

// Store key in collection.
func (coll *Collection) Set(key string, val []byte) (int64, int64, error) {
	data, err := NewKey(val).Bytes()
	if err != nil {
		return 0, 0, err
	}

	off, size, err := coll.bucket.Write(data)

	// Index new key.
	err = coll.indexes.Add(key, data, uint64(off))
	if err != nil {
		return 0, 0, err
	}

	return off, size, err
}

// Get key from collection.
func (coll *Collection) Get(key string) ([]byte, error) {
	idx, err := coll.indexes.Get(key)
	if err != nil {
		return nil, err
	}

	val, err := coll.bucket.Read(int64(idx.Offset), int64(idx.Size))
	if err != nil {
		return nil, err
	}

	return val, err
}
