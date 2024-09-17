package main

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// RateLimit is a simple rate limiter that allows a maximum of tokensLimit requests at a time.
type RateLimit struct {
	sem *semaphore.Weighted
}

func NewRateLimit(tokensLimit int) *RateLimit {
	return &RateLimit{
		sem: semaphore.NewWeighted(int64(tokensLimit)),
	}
}

// Aquire blocks until a token is available.
func (r *RateLimit) Acquire() {
	r.sem.Acquire(context.Background(), 1)
}

// Release releases a token.
func (r *RateLimit) Release() {
	r.sem.Release(1)
}
