package db

import (
	"bytes"
	"os"
	"testing"
)

func TestOpenBucket(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _:= db1.Collection("test")

	bck, _ := OpenBucket(coll.root + "/1.bucket", 10, 5, 2)
	if bck == nil {
		t.Errorf("Expected Bucket object, got nil")
	}
}

func TestBucketWrite(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _ := db1.Collection("test")
	bck,  _ := OpenBucket(coll.root + "/1.bucket", 10, 5, 2)

	data := []byte("value1")
	_, size, _ := bck.Write(data)
	if size != int64(len(data)) {
		t.Errorf("Expected %d bytes to be written, got %d.", int64(len(data)), size)
	}
}

func TestBucketRead(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _ := db1.Collection("test")
	bck,  _ := OpenBucket(coll.root + "/1.bucket", 10, 5, 2)

	data1 := []byte("value1")
	bck.Write(data1)

	data2, _ := bck.Read(0, 6)

	if !bytes.Equal(data1, data2) {
		t.Errorf("Expected read to return %s, got %s.", data1, data2)
	}
}

func TestBucketResize(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _ := db1.Collection("test")
	bck,  _ := OpenBucket(coll.root + "/1.bucket", 10, 5, 2)

	data := []byte("value")
	for i := 0; i < 10; i++ {
		bck.Write(data)
	}

	if bck.offset.Load() != 50 {
		t.Errorf("Expected offset to be %d, got %d.", 50, bck.offset.Load())
	}

	if bck.sizeLimit != 80 {
		t.Errorf("Expected size limit to be %d, got %d.", 80, bck.sizeLimit)
	}
}

func TestGetLastBucket(t *testing.T) {
	path1 := "./db/collections/test/1/10.bucket"
	path2 := "./db/collections/test/2/300.bucket"
	path3 := "./db/collections/test/11/300.bucket"
	path4 := "./db/collections/test/12/100.bucket"

	os.MkdirAll(path1, 0755)
	os.MkdirAll(path2, 0755)
	os.MkdirAll(path3, 0755)
	os.MkdirAll(path4, 0755)
	defer os.RemoveAll("./db")

	expected := "./db/collections/test/12/100.bucket"
	path := getLastBucket("./db/collections/test")

	if path != expected {
		t.Errorf("Expected path to be %s, got %s.", expected, path)
	}
}

func TestNextBucket(t *testing.T) {
	bucketsPerDir := 2
	
	// 1. Dir is full.
	path1 := "./db/collections/test/1/1.bucket"
	path2 := "./db/collections/test/1/2.bucket"

	os.MkdirAll(path1, 0755)
	os.MkdirAll(path2, 0755)
	defer os.RemoveAll("./db")

	bucket := Bucket{
		ID: 2,
		Dir: "./db/collections/test/",
		bucketsPerDir: int16(bucketsPerDir),
	}

	bucket.nextBucket()
	
	if bucket.ID != 3 {
		t.Errorf("Expected bucket ID to be %d, got %d.", 3, bucket.ID)
	}
	
	expected := "./db/collections/test/2/3.bucket"
	if bucket.file.Name() != expected {
		t.Errorf("Expected file to be %s, got %s.", expected, bucket.file.Name())
	}

	// 2. Dir have space.
	bucket.nextBucket()

	if bucket.ID != 4 {
		t.Errorf("Expected bucket ID to be %d, got %d.", 4, bucket.ID)
	}

	expected = "./db/collections/test/2/4.bucket"
	if bucket.file.Name() != expected {
		t.Errorf("Expected file to be %s, got %s.", expected, bucket.file.Name())
	}
}