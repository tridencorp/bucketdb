package buckets

import (
	"bucketdb/tests"
	"os"
	"testing"

	"golang.org/x/exp/rand"
)

func TestOpenBuckets(t *testing.T) {
	conf := Config{2, 1_000_000, 2, 100}

	_, err := Open("./test", conf)
	defer os.RemoveAll("./test")

	tests.Assert(t, err, nil)
}

func TestRefCount(t *testing.T) {
	conf := Config{2, 1_000_000, 2, 100}
	buckets, _ := Open("./test", conf)
	defer os.RemoveAll("./test")

	tests.RunConcurrently(50_000, func(){
		b := buckets.Last()
		buckets.Put(b)	
	})

	tests.Assert(t, 1, buckets.items[1].refCount.Load())
}

func TestGet(t *testing.T) {
	conf := Config{2, 1_000_000, 2, 100}
	buckets, _ := Open("./test", conf)
	defer os.RemoveAll("./test")

	buckets.Open(1)
	buckets.Open(2)
	buckets.Open(3)
	buckets.Open(4)

	tests.RunConcurrently(50_000, func(){
		id := rand.Intn(3) + 1

		bucket := buckets.Get(id)
		buckets.Put(bucket)
	})
}