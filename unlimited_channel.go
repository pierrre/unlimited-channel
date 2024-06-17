// Package unlimitedchannel provides an unlimited channel.
package unlimitedchannel

import (
	"context"
	"sync"

	"github.com/pierrre/go-libs/goroutine"
)

// Channel is an unlimited channel.
// It can store an unlimited number of values.
//
// The channel returned by [Channel.In] must be closed in order to release resources.
type Channel[T any] struct {
	// Context is the context use to run the internal goroutine.
	// If not set, context.Background() is used.
	Context context.Context //nolint:containedctx // It's fine.

	once sync.Once

	queue queue[T]

	in  chan T
	out chan T
}

func (c *Channel[T]) ensureInit() {
	c.once.Do(c.init)
}

func (c *Channel[T]) init() {
	// Using buffered channels seems to improve performance.
	c.in = make(chan T, 10)
	c.out = make(chan T, 10)
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	goroutine.Start(ctx, func(context.Context) {
		c.run()
	})
}

func (c *Channel[T]) run() {
	defer close(c.out)
	defer c.queue.reset()
	for {
		outValue, okOutValue := c.queue.pick()
		var inValue T
		var okInValue bool
		if okOutValue {
			select {
			case inValue, okInValue = <-c.in:
			case c.out <- outValue:
				c.queue.dequeue()
				continue
			}
		} else {
			inValue, okInValue = <-c.in
		}
		if !okInValue {
			return
		}
		c.queue.enqueue(inValue)
	}
}

// In returns the input channel.
//
// It must be closed in order to release resources.
func (c *Channel[T]) In() chan<- T {
	c.ensureInit()
	return c.in
}

// Out returns the output channel.
//
// It is automatically closed when the input channel is closed.
func (c *Channel[T]) Out() <-chan T {
	c.ensureInit()
	return c.out
}
