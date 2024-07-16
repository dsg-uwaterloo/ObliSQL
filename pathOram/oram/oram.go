package oram

import (
	// mathrand "math/rand"
	// "crypto/aes"
	// "crypto/cipher"
	// "crypto/rand"
	// "encoding/json"
	// "errors"
	// "fmt"
	// "strings"
	// "io"
    // "github.com/go-redis/redis/v8" // using go-redis library
	// "context"
)

// Block represents a key-value pair
type Block struct {
	BlockId int
	Key   int    // dummy can have key -1
	Value string
}

// Bucket represents a collection of blocks
type Bucket struct {
	Blocks        []Block
	RealBlockCount int
}