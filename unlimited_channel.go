// Package unlimitedchannel provides a channel with unlimited capacity.
package unlimitedchannel

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pierrre/go-libs/goroutine"
)

// Channel is a channel with unlimited capacity
// It stores values in an in-memory queue.
// Sending values to the input channel is non-blocking.
//
// The input channel must be closed, in order to release resources.
// The output channel will be closed when all the resources have been released.
//
// It must be created with [New].
type Channel[T any] struct {
	in             chan T
	out            chan T
	len            atomic.Int64
	inClosed       atomic.Bool
	inClosedNotify chan struct{}
	inWait         goroutine.Waiter
	outWait        goroutine.Waiter
	sendAllOnClose bool

	cond  sync.Cond
	queue queue[T]
}

// New creates a new [Channel].
func New[T any](opts ...Option) *Channel[T] {
	o := buildOptions(opts)
	buffer := max(0, o.buffer)
	c := &Channel[T]{
		in:             make(chan T, buffer),
		out:            make(chan T, buffer),
		inClosedNotify: make(chan struct{}),
		sendAllOnClose: o.sendAllOnClose,
	}
	c.cond.L = new(sync.Mutex)
	if o.release != nil {
		*o.release = sync.OnceFunc(func() {
			c.release()
		})
	}
	ctx := context.Background()
	c.inWait = goroutine.Start(ctx, func(ctx context.Context) {
		c.runInput()
	})
	c.outWait = goroutine.Start(ctx, func(ctx context.Context) {
		c.runOutput()
	})
	return c
}

// Input returns the input channel.
func (c *Channel[T]) Input() chan<- T {
	return c.in
}

// Output returns the output channel.
func (c *Channel[T]) Output() <-chan T {
	return c.out
}

// Len returns the length of the channel.
//
// It's not guaranteed to be accurate.
// The value will eventually stabilize to the real value.
func (c *Channel[T]) Len() int {
	return len(c.out) + int(c.len.Load()) + len(c.in)
}

// Wait waits for the resources to be released.
func (c *Channel[T]) Wait() {
	c.inWait.Wait()
	c.outWait.Wait()
}

func (c *Channel[T]) release() {
	inOpen := true
	for inOpen {
		select {
		case _, inOpen = <-c.in:
		default:
			close(c.in)
		}
	}
	for range c.out {
	}
	c.Wait()
}

func (c *Channel[T]) runInput() {
	for v := range c.in {
		c.len.Add(1)
		c.cond.L.Lock()
		c.queue.enqueue(v)
		c.cond.L.Unlock()
		c.cond.Signal()
	}
	c.inClosed.Store(true)
	close(c.inClosedNotify)
	c.cond.Signal()
}

//nolint:gocyclo // Yes it's complex.
func (c *Channel[T]) runOutput() {
	defer close(c.out)
	var v T
	var ok bool
	var zero T
	for {
		if c.inClosed.Load() && !c.sendAllOnClose {
			return
		}
		if !ok {
			c.cond.L.Lock()
			for {
				v, ok = c.queue.dequeue()
				if ok {
					break
				}
				if c.inClosed.Load() {
					break
				}
				c.cond.Wait()
			}
			c.cond.L.Unlock()
			if !ok {
				return
			}
		}
		if c.inClosed.Load() {
			c.out <- v
		} else {
			select {
			case c.out <- v:
			default:
				select {
				case c.out <- v:
				case <-c.inClosedNotify:
					continue
				}
			}
		}
		v = zero
		ok = false
		c.len.Add(-1)
	}
}

type options struct {
	sendAllOnClose bool
	buffer         int
	release        *func()
}

func buildOptions(opts []Option) *options {
	o := &options{
		buffer: 100,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Option represents an option for [New].
type Option func(*options)

// WithSendAllOnClose sets the option to send all remaining values before closing the output channel.
// The default value is false, which means that the output channel will be closed as soon as the input channel is closed, without sending the remaining values.
// If true, all values from the output channel must be read until it is closed, in order to release resources.
func WithSendAllOnClose(send bool) Option {
	return func(o *options) {
		o.sendAllOnClose = send
	}
}

// WithBuffer sets the buffer size of the input/output channels.
// The default value is 100, and offers better performance than 0 (unbuffered).
// A negative value is equivalent to 0.
func WithBuffer(buffer int) Option {
	return func(o *options) {
		o.buffer = buffer
	}
}

func withRelease(release *func()) Option {
	return func(o *options) {
		o.release = release
	}
}
