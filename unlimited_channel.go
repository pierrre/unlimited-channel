// Package unlimitedchannel provides a channel with unlimited capacity.
package unlimitedchannel

import (
	"context"
	"sync/atomic"

	"github.com/pierrre/go-libs/goroutine"
)

// Channel is a channel with unlimited capacity.
// It stores values in an in-memory queue.
//
// It must be created with [New], and closed with [Channel.Close] to release resources.
type Channel[T any] struct {
	in     chan T
	out    chan T
	length atomic.Int64
	wait   func()
}

// New creates a new [Channel].
func New[T any](opts ...Option) *Channel[T] {
	o := buildOptions(opts)
	buffer := max(0, o.buffer)
	c := &Channel[T]{
		in:  make(chan T, buffer),
		out: make(chan T, buffer),
	}
	c.wait = goroutine.Start(o.context, func(ctx context.Context) {
		c.run()
	}).Wait
	return c
}

// Input returns the input channel.
//
// Closing the input channel notifies the channel that no more values will be sent, and it should release resources when all values are sent to the output channel.
// However calling [Channel.Close] is still required to release resources.
func (c *Channel[T]) Input() chan<- T {
	return c.in
}

// Output returns the output channel.
//
// It is closed when all resources have been released.
func (c *Channel[T]) Output() <-chan T {
	return c.out
}

// Len returns the length of the channel.
//
// The result may not be consistent with the actual number of values in the channel.
func (c *Channel[T]) Len() int {
	l := len(c.out)
	l += int(c.length.Load())
	l += len(c.in)
	return l
}

// Close closes the channel.
//
// It releases all resources used by the channel: buffered values are discarded, and the input/output channels are closed.
func (c *Channel[T]) Close() {
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
	c.wait()
}

func (c *Channel[T]) run() { //nolint:gocyclo // Yes it's complex.
	defer close(c.out)
	q := new(queue[T])
	in := c.in
	var inValue T
	inOpen := true      // Indicates whether the input channel is open.
	inReceived := false // Indicates whether the input channel received something (a value or close event).
	out := c.out
	var outValue T
	outValueOK := false // Indicates whether the output value is set.
	outSent := false    // Indicates whether the output value was sent to the output channel.
	var zero T
	for {
		if outSent { // If the output value was sent to the output channel.
			outSent = false
			outValue = zero // Reset the output value.
			outValueOK = false
			c.length.Add(-1)
		}
		if inReceived { // If the input channel received something (a value or closed).
			inReceived = false
			if inOpen { // If the input channel is open, a value was received.
				if !outValueOK { // If the output value is not set.
					outValue = inValue // Set the output value with the input value, without adding it to the queue.
					outValueOK = true
				} else {
					q.enqueue(inValue) // Add the input value to the queue.
				}
				inValue = zero
				c.length.Add(1)
			}
		}
		if !outValueOK { // If the output value is not set.
			outValue, outValueOK = q.dequeue() // Try to get a value from the queue.
		}
		if !inOpen { // If the input channel is closed.
			if !outValueOK { // If there is no more value to send to the output channel.
				return
			}
			out <- outValue // Send the remaining values to the output channel.
			outSent = true
			continue
		}
		if !outValueOK { // If there is no value to send to the output channel.
			inValue, inOpen = <-in // Try to receive a value from the input channel.
			inReceived = true
			continue
		}
		select { // Try to send the value to the output channel, before receiving a value from the input channel.
		case out <- outValue:
			outSent = true
			continue
		default: // The output channel was not ready.
		}
		select { // Try to receive a value from the input channel.
		case inValue, inOpen = <-in:
			inReceived = true
			continue
		default: // The input channel was not ready.
		}
		select { // Try to receive a value from the input channel, or send the value to the output channel.
		case inValue, inOpen = <-in:
			inReceived = true
		case out <- outValue:
			outSent = true
		}
	}
}

type options struct {
	context context.Context //nolint:containedctx // It's OK.
	buffer  int
}

func buildOptions(opts []Option) *options {
	o := &options{
		context: context.Background(),
		buffer:  100,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Option represents an option for [New].
type Option func(*options)

// WithContext sets the [context.Context] for the channel.
// It runs the goroutine that handles the channel.
// Cancelling the context has no effect on the channel.
// It uses [context.Background] by default.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.context = ctx
	}
}

// WithBuffer sets the buffer size of the input/output channels.
// The default value is 100, which usually performs better than 0 (unbuffered).
// A negative value is equivalent to 0.
func WithBuffer(buffer int) Option {
	return func(o *options) {
		o.buffer = buffer
	}
}
