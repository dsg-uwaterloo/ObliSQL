package oramexecutor

// Block represents a key-value pair
type Block struct {
	Key   string // dummy can have key -1
	Value string
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
