package db

import (
	"bucketdb/db/buckets"
	"bucketdb/db/index"
	"os"
	"path/filepath"
	"sync/atomic"
)

type Collection struct {
	bucket  *buckets.Bucket
	buckets *buckets.Buckets

	indexes *index.File
	config  buckets.Config

	// Collection root directory.
	root string

	// We will be using atomic.Swap() for each key.
	// In combination with WriteAt, it should give
	// us the ultimate concurrent writes.
	offset atomic.Int64
}

// Open the collection. If it doesn't exist,
// create one with default values.
func (db *DB) Collection(name string, conf buckets.Config) (*Collection, error) {
	// Build collection path.
	path := db.root + CollectionsPath + name
	return newCollection(path, conf)
}

func newCollection(path string, conf buckets.Config) (*Collection, error) {
	// Build collection path.
	dir := filepath.Dir(path)

	// Create directory structure. Do nothing if it already exist.
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	// Open buckets.
	buckets, err := buckets.Open(path, conf)
	if err != nil {
		return nil, err
	}

	indexes, err := index.Load(dir, 100_000)
	if err != nil {
		return nil, err
	}

	coll := &Collection {
		buckets: buckets, 
		root:    path,
		indexes: indexes,
		config:  conf,
	}

	// TODO: because of file truncation we should track current 
	// data size and set our initial offset based on it.
	coll.offset.Store(0)

	return coll, nil
}

// Open or create new hash.
func (coll *Collection) Hash(name string) (*Hash, error) {
	root := coll.root + "/hashes/"
	keys, err := newCollection(root, coll.config)
	if err != nil {
		return nil, err 
	}

	return &Hash{root: root, keys: keys}, nil
}

// Store key in collection.
func (c *Collection) Set(key string, val []byte) (int64, int64, error) {
	data, err := buckets.NewKV(key, val).Bytes()

	bucket := c.buckets.Last()
	off, size, id, err := bucket.Write(data)

	// Index new key.
	err = c.indexes.Set([]byte(key), len(data), uint64(off), id)
	if err != nil {
		return 0, 0, err
	}

	return off, size, err
}

// Get key from collection.
func (coll *Collection) Get(key string) ([]byte, error) {
	idx, err := coll.indexes.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	// TODO: Based on index we need to pick proper bucket.
	raw, err := coll.bucket.Read(int64(idx.Offset), int64(idx.Size))
	if err != nil {
		return nil, err
	}

	kv := new(buckets.KV)
	kv.FromBytes(raw)
	return kv.Val, err
}

// Delete key from collection.
// TODO: Set Deleted flag for Key.
func (coll *Collection) Del(key string) error {
	return coll.indexes.Del([]byte(key))	
}

// Update key from collection.
func (coll *Collection) Update(key string, val []byte) error {
	_, _, err := coll.Set(key, val)
	if err != nil {
		return err
	}

	return nil
}
