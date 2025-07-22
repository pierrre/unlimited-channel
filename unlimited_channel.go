// Package unlimitedchannel provides a channel with unlimited capacity.
package unlimitedchannel

import (
	"context"
	"sync"

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
	cond           sync.Cond
	queue          queue[T]
	len            int
	inClosed       bool
	inClosedNotify chan struct{}
	inWait         goroutine.Waiter
	outWait        goroutine.Waiter
	sendAllOnClose bool
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
	ctx := o.context
	if ctx == nil {
		ctx = context.Background()
	}
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
func (c *Channel[T]) Len() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return len(c.out) + c.len + len(c.in)
}

// Wait waits for the worker to finish processing.
func (c *Channel[T]) Wait() {
	c.inWait.Wait()
	c.outWait.Wait()
}

func (c *Channel[T]) release() {
	inOpen := true
	for inOpen { // Drain the input channel, and ensure it is closed.
		select {
		case _, inOpen = <-c.in:
		default:
			close(c.in)
		}
	}
	for range c.out { // Drain the output channel until it is closed.
	}
	c.Wait()
}

func (c *Channel[T]) runInput() {
	for v := range c.in {
		c.cond.L.Lock()
		c.len++
		c.queue.enqueue(v)
		c.cond.Signal()
		c.cond.L.Unlock()
	}
	c.cond.L.Lock()
	c.inClosed = true
	c.cond.Signal()
	c.cond.L.Unlock()
}

func (c *Channel[T]) runOutput() {
	defer close(c.out)
	for {
	}
}

func (c *Channel[T]) getFromQueue() (v T, ok bool) {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	for {
		if c.inClosed {
			return v, false
		}
		v, ok = c.queue.dequeue()
		if ok {
			return v, true
		}
		c.cond.Wait()
	}
}

func (c *Channel[T]) sendToOutput(v T) bool {

}

type options struct {
	context        context.Context //nolint:containedctx // It's OK.
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

// WithContext sets the [context.Context] for the channel.
// It's used to run the goroutine that handles the channel.
// Cancelling the context has no effect on the channel.
// It uses [context.Background] by default.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.context = ctx
	}
}

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
