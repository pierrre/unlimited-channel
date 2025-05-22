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
	sendAllOnClose bool
}

// New creates a new [Channel].
func New[T any](opts ...Option) *Channel[T] {
	o := buildOptions(opts)
	buffer := max(0, o.buffer)
	c := &Channel[T]{
		in:             make(chan T, buffer),
		out:            make(chan T, buffer),
		sendAllOnClose: o.sendAllOnClose,
	}
	ctx := o.context
	if ctx == nil {
		ctx = context.Background()
	}
	goroutine.Start(ctx, func(ctx context.Context) {
		defer close(c.out)
		c.run()
	})
	if o.release != nil {
		*o.release = sync.OnceFunc(func() {
			c.release()
		})
	}
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

func (c *Channel[T]) run() { //nolint:gocyclo // Yes it's complex.
	q := new(queue[T])
	var inValue T
	inOpen := true      // Indicates if the input channel is open.
	inReceived := false // Indicate if the input channel received something (a value or closed).
	var outValue T
	outOK := false // Indicates if the output value is set.
	var zero T
	for {
		if inReceived { // If the input channel received something (a value or closed).
			inReceived = false
			if inOpen { // If the input channel is open, a value was received.
				if !outOK { // If the output value is not set.
					outValue = inValue // Set the output value with the input value,  without adding it to the queue.
					outOK = true
				} else {
					q.enqueue(inValue) // Add the input value to the queue.
				}
				inValue = zero
			}
		}
		if !outOK { // If the output value is not set.
			outValue, outOK = q.dequeue() // Try to get a value from the queue.
		}
		if !inOpen { // If the input channel is closed.
			if !outOK { // If there is no more value to send to the output channel.
				return
			}
			if !c.sendAllOnClose { // If we don't need to send all remaining values to the output channel.
				return
			}
			c.out <- outValue // Send the remaining values to the output channel.
			outOK = false
			continue
		}
		if !outOK { // If there is no value to send to the output channel.
			inValue, inOpen = <-c.in // Try to receive a value from the input channel.
			inReceived = true
			continue
		}
		select { // Try to send the value to the output channel, before receiving a value from the input channel.
		case c.out <- outValue:
			outValue = zero
			outOK = false
			continue
		default: // The output channel was not ready.
		}
		select { // Try to receive a value from the input channel.
		case inValue, inOpen = <-c.in:
			inReceived = true
			continue
		default: // The input channel was not ready.
		}
		select { // Try to receive a value from the input channel, or send the value to the output channel.
		case inValue, inOpen = <-c.in:
			inReceived = true
		case c.out <- outValue:
			outValue = zero
			outOK = false
		}
	}
}

func (c *Channel[T]) release() {
	inClosed := false
	for !inClosed { // Drain the input channel, and wa
		select {
		case _, ok := <-c.in:
			if !ok {
				inClosed = true
			}
		default:
			close(c.in)
		}
	}
	for range c.out {
	}
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
