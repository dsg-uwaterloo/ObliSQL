package bucket

import (
	"pathOram/pkg/oram/block"
)

// Bucket represents a collection of blocks
type Bucket struct {
	Blocks         []block.Block
	RealBlockCount int
}
