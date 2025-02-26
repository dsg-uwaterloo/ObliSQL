package oramexecutor

import (
	"bytes"
	"errors"
)

// Block represents a key-value pair
type Block struct {
	Key   [32]byte  // Fixed size 32 bytes for the key
	Value [512]byte // Fixed size 512 bytes for the value
}

// Bucket represents a collection of blocks
type Bucket struct {
	Blocks         []Block
	RealBlockCount int
}

type BucketRequest struct {
	BucketId int
	Bucket   Bucket
}

// Request represents incoming PUT/GET requests
type Request struct {
	Key   string
	Value string
}

// NewBlock creates a Block while ensuring fixed size for key and value.
func NewBlock(key, value string) (*Block, error) {
	var b Block

	// Ensure key is at most 32 bytes
	if len(key) > 32 {
		return nil, errors.New("key exceeds 32 bytes")
	}

	// Ensure value is at most 512 bytes
	if len(value) > 512 {
		return nil, errors.New("value exceeds 512 bytes")
	}

	// Copy key and value into fixed-size byte arrays
	copy(b.Key[:], key)
	copy(b.Value[:], value)

	return &b, nil
}

// GetKey returns the key as a string, trimming trailing null bytes.
func (b *Block) GetKey() string {
	return string(bytes.TrimRight(b.Key[:], "\x00")) // Trim trailing nulls
}

// GetValue returns the value as a string, trimming trailing null bytes.
func (b *Block) GetValue() string {
	return string(bytes.TrimRight(b.Value[:], "\x00")) // Trim trailing nulls
}
