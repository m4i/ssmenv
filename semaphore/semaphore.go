package semaphore

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Semaphore provides golang.org/x/sync/errgroup.Group with limited concurrency.
type Semaphore struct {
	*errgroup.Group
	ctx context.Context
	c   chan struct{}
}

// New returns a new semaphore with the given limited concurrency.
func New(n int) *Semaphore {
	g, ctx := errgroup.WithContext(context.Background())
	return &Semaphore{g, ctx, make(chan struct{}, n)}
}

// Go calls the given function in a new goroutine.
func (s *Semaphore) Go(fn func() error) {
	s.Group.Go(func() error {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case s.c <- struct{}{}:
			defer func() { <-s.c }()
			return fn()
		}
	})
}
