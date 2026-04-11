// Package unlimitedchannel provides a channel with unlimited capacity.
package unlimitedchannel

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pierrre/go-libs/goroutine"
)

// Channel is a channel with unlimited capacity.
// It stores values in an in-memory queue.
// Sending values to the input channel is non-blocking.
//
// The input channel must be closed to release resources.
// The output channel will be closed when all the resources have been released.
//
// It must be created with [New].
type Channel[T any] struct {
	in             chan T
	out            chan T
	sendAllOnClose bool
	length         atomic.Int64
	wait           goroutine.Waiter
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
	if o.release != nil {
		*o.release = sync.OnceFunc(func() {
			c.release()
		})
	}
	ctx := o.context
	if ctx == nil {
		ctx = context.Background()
	}
	c.wait = goroutine.Start(ctx, func(ctx context.Context) {
		defer close(c.out)
		c.run()
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
	l := len(c.out)
	l += int(c.length.Load())
	l += len(c.in)
	return l
}

// Wait waits for the worker to finish processing.
func (c *Channel[T]) Wait() {
	c.wait.Wait()
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

func (c *Channel[T]) run() { //nolint:gocyclo // Yes it's complex.
	q := new(queue[T])
	in := c.in
	var inValue T
	inOpen := true      // Indicates whether the input channel is open.
	inReceived := false // Indicates whether the input channel received something (a value or close event).
	out := c.out
	var outValue T
	outValueOK := false // Indicates whether the output value is set.
	outSent := false    // Indicates whether the output value was sent to the output channel.
	sendAllOnClose := c.sendAllOnClose
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
			if !sendAllOnClose { // If we don't need to send all remaining values to the output channel.
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
// It runs the goroutine that handles the channel.
// Cancelling the context has no effect on the channel.
// It uses [context.Background] by default.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.context = ctx
	}
}

// WithSendAllOnClose sets the option to send all remaining values before closing the output channel.
// The default value is false.
// In that case, the output channel is closed as soon as the input channel is closed, without sending the remaining values.
// If true, all values from the output channel must be read until it is closed to release resources.
func WithSendAllOnClose(send bool) Option {
	return func(o *options) {
		o.sendAllOnClose = send
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

func withRelease(release *func()) Option {
	return func(o *options) {
		o.release = release
	}
}
