package db

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Keeping file related data in one place. It will be easier
// to use this in concurrent world.
type File struct {
	fd *os.File

	// We will be using atomic.Add() for each key.
	// In combination with WriteAt, it should give
	// us the ultimate concurrent writes.
	offset atomic.Int64

	// If offset reach size limit, we resize the file.
	// We double it's size.
	sizeLimit uint64
}

type Bucket struct {
	ID  uint32
	Dir string

	// This is the main file to which we will read and write.
	file atomic.Pointer[File]

	// Number of bucket files per directory.
	bucketsPerDir int16

	// Keep track of the number of keys in the bucket.
	keysCount atomic.Int64
	keysLimit uint64

	// Mutex.
	mux sync.RWMutex
}

func OpenBucket(root string, keysLimit uint32, sizeLimit int64, bucketsPerDir int32) (*Bucket, error) {
	f, err := getLastBucket(root)
	if err != nil {
		return nil, err
	}

	// TODO: Temporary values untill we have proper bucket management.[]
	bucket := &Bucket{
		ID:1, 
		Dir: root,
		keysLimit: uint64(keysLimit),
		bucketsPerDir: int16(bucketsPerDir),
	}

	file := &File{fd: f, sizeLimit: uint64(sizeLimit)}

	bucket.file.Store(file)
	file.offset.Store(getOffset(bucket))

	return bucket, nil;
}

// Find the last bucket ID for given root.
// Empty string in response mesteans that there is no bucket yet.
func getLastBucket(root string) (*os.File, error) {
	// Sort directories.
	dirs, _ := os.ReadDir(root)
	max := 0

	for _, dir := range dirs {
		id, _ := strconv.Atoi(dir.Name())
		if id > max { max = id }
	}

	// Directory is empty, no buckets yet, so we have to create first one.
	if max == 0 {
		root = filepath.Join(root, "1")
		os.MkdirAll(root, 0755)

		root = filepath.Join(root, "1.bucket")
		file, err := os.OpenFile(root, os.O_RDWR|os.O_CREATE, 0644)
		return file, err
	}

	// Sort files.
	root += fmt.Sprintf("/%d", max)
	files, _ := os.ReadDir(root)

	for _, file := range files {
		// Split .bucket file.
		fileId := strings.Split(file.Name(), ".")[0]

		id, _ := strconv.Atoi(fileId) 
		if id > max { max = id }
	}

	root = filepath.Join(root, fmt.Sprintf("%d.bucket", max))
	file, err := os.OpenFile(root, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Create next bucket.
func (bucket *Bucket) nextBucket() (*os.File, error) {
	id := bucket.ID + 1
	file := &File{}

	// Based on buckets per dir we can calculate folder ID in which
	// bucket should be.
	folderId := int(math.Ceil(float64(id) / float64(bucket.bucketsPerDir)))

	path := filepath.Join(bucket.Dir, fmt.Sprintf("%d", folderId))
	err  := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	path = filepath.Join(bucket.Dir, fmt.Sprintf("%d", folderId), fmt.Sprintf("%d.bucket", id))
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)

	bucket.ID = id

	// We created new bucket, there are no keys yet so we must restart counters, 
	// offsets, ...
	file.fd = fd
	file.offset.Store(0)

	bucket.keysCount.Store(0)
	bucket.file.Store(file)

	return fd, err
}

// Write data to bucket.
//
// TODO: Should buckets know about keys and other
// types ? Should they operate only on raw bytes ?
func (bucket *Bucket) Write(data []byte) (int64, int64, error) {
	// Load file - we want to load all file related data at once.
	file := bucket.file.Load()

	count := bucket.keysCount.Add(1)
	limit := int64(bucket.keysLimit)

	// TODO: File, offset and keysCount must be in the same struct
	// and we will use atomics to load them.

	// We are adding len to atomic value and then deducting it
	// from the result, this should give us space for our data.
	totalOff := file.offset.Add(int64(len(data)))
	writeOff := totalOff - int64(len(data))

	off  := int64(0)
	size := int64(0)

	// Resize the file when we reach size limit.
	if totalOff >= int64(file.sizeLimit) {
		bucket.mux.Lock()
		// Check if our condition is still valid - some other goroutine 
		// could changed the size limit in the time we was waiting for lock.
		if totalOff >= int64(file.sizeLimit) {
			err := bucket.resize(file)
			if err != nil {
				return 0, 0, err
			}
		}
		bucket.mux.Unlock()
	}

	// We reached keys limit, we must create next bucket.
	// TODO: check if some other goroutine didn't created new bucket in meantime.
	if count >= limit {
		bucket.mux.Lock()
		_, err := bucket.nextBucket()
		if err != nil {
			return 0, 0, err
		}
		bucket.mux.Unlock()
	}

	if count <= limit {
		bucket.mux.RLock()
		off, size, _ = bucket.write(file.fd, writeOff, data)
		bucket.mux.RUnlock()
	}

	return off, size, nil
}

func (bucket *Bucket) resize(file *File) error {
	file.sizeLimit = file.sizeLimit * 2
	err := file.fd.Truncate(int64(file.sizeLimit))
	return err
}

// Getting last offset from which we can start writing data.
// For now we just do it dead simple, read file from beginning
// record by record till end of data. 
// It would basically be done only for last block - the one we are currently writing to. 
// Other blocks will be immutable (so no offset needed).
func getOffset(bucket *Bucket) int64 {
	it := Iterator{bucket: bucket}
	_, size, _ := it.Iterate()
	return size
}

func (bucket *Bucket) write(file *os.File, off int64, data []byte) (int64, int64, error) {
	// We are using WriteAt because, when carefully
	// handled, it's concurrent-friendly.
	size, err := file.WriteAt(data, off)
	if err != nil {
		return off, int64(size), err
	}

	return off, int64(size), nil
}

// Read data from bucket.
func (bucket *Bucket) Read(offset int64, size int64) ([]byte, error) {
	data := make([]byte, size)
	file := bucket.file.Load()

	_, err := file.fd.ReadAt(data, offset)
	if err != nil {
		return nil, err
	}

	return data, nil
}
