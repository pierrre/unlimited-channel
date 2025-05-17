// Package unlimitedchannel provides an unlimited channel.
package unlimitedchannel

import (
	"context"

	"github.com/pierrre/go-libs/goroutine"
)

// New creates a new channel with unlimited capacity.
// It stores values in an in-memory queue.
//
// The caller must close the input channel when it is no longer needed, in order to release resources.
// Closing the input channel will close the output channel.
func New[T any](opts ...Option) (input chan<- T, output <-chan T) {
	o := buildOptions(opts)
	in := make(chan T, o.buffer)
	out := make(chan T, o.buffer)
	ctx := o.context
	if ctx == nil {
		ctx = context.Background()
	}
	goroutine.Start(ctx, func(ctx context.Context) {
		run(in, out, o.sendAllOnClose)
	})
	return in, out
}

func run[T any](in <-chan T, out chan<- T, sendAllOnClose bool) { //nolint:gocyclo // Yes it's complex.
	defer close(out)
	q := new(queue[T])
	inOK := true // Indicates if the input channel is open.
	var outValue T
	outOK := false // Indicates if the output value is set.
	for {
		var inValue T
		if !outOK { // If the output value is not set.
			outValue, outOK = q.dequeue()
		}
		if !inOK { // If the input channel is closed.
			if !outOK { // If there is no more value to send to the output channel.
				return
			}
			if !sendAllOnClose { // If we don't need to send all remaining values to the output channel.
				return
			}
			out <- outValue // Send the remaining values to the output channel.
			outOK = false
			continue
		}
		if !outOK { // If there is no value to send to the output channel.
			inValue, inOK = <-in // Try to receive a value from the input channel.
			handleInput(inValue, inOK, &outValue, &outOK, q)
			continue
		}
		select { // Try to send the value to the output channel, before receiving a value from the input channel.
		case out <- outValue:
			outOK = false
			continue
		default: // The output channel was not ready.
		}
		select { // Try to receive a value from the input channel.
		case inValue, inOK = <-in:
			handleInput(inValue, inOK, &outValue, &outOK, q)
			continue
		default:
		}
		select { // Try to receive a value from the input channel, or send the value to the output channel.
		case inValue, inOK = <-in:
			handleInput(inValue, inOK, &outValue, &outOK, q)
		case out <- outValue:
			outOK = false
		}
	}
}

func handleInput[T any](inValue T, inOK bool, pOutValue *T, pOutOK *bool, q *queue[T]) {
	if !inOK { // If the input channel is closed.
		return
	}
	if !*pOutOK { // If the output value is not set.
		*pOutValue = inValue // Set the output value without adding it to the queue.
		*pOutOK = true
		return
	}
	q.enqueue(inValue) // Add the input value to the queue.
}

type options struct {
	context        context.Context //nolint:containedctx // It's OK.
	sendAllOnClose bool
	buffer         int
}

func buildOptions(opts []Option) *options {
	o := &options{
		buffer: 10,
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
// If true, all values from the output channel must be read until it is closed, in order to release resources.
func WithSendAllOnClose(send bool) Option {
	return func(o *options) {
		o.sendAllOnClose = send
	}
}

// WithBuffer sets the buffer size of the input/output channels.
// The default value is 10, and offers better performance than 0.
func WithBuffer(buffer int) Option {
	return func(o *options) {
		o.buffer = buffer
	}
}
