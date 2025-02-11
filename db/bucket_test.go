package db

import (
	"bytes"
	"testing"
)

func TestOpenBucket(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _:= db1.Collection("test")

	bck, _ := OpenBucket(coll.root + "/1.bucket")
	if bck == nil {
		t.Errorf("Expected Bucket object, got nil")
	}
}

func TestBucketWrite(t *testing.T) {
	db1, _ := Open("./db")
	db1.Delete()

	coll, _ := db1.Collection("test")
	bck,  _ := OpenBucket(coll.root + "/1.bucket")

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
	bck,  _ := OpenBucket(coll.root + "/1.bucket")

	data1 := []byte("value1")
	bck.Write(data1)

	data2, _ := bck.Read(0, 6)

	if !bytes.Equal(data1, data2) {
		t.Errorf("Expected read to return %s, got %s.", data1, data2)
	}
}
