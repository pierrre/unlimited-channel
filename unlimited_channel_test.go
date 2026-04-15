package unlimitedchannel

import (
	"fmt"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pierrre/assert"
)

func Example() {
	c := New[int]()
	in, out := c.Input(), c.Output()
	in <- 1
	in <- 2
	v := <-out
	fmt.Println(v)
	v = <-out
	fmt.Println(v)
	c.Close()
	_, ok := <-out
	fmt.Println("open:", ok)
	// Output:
	// 1
	// 2
	// open: false
}

func newTestChannel(tb testing.TB, opts ...Option) *Channel[int] {
	tb.Helper()
	c := New[int](opts...)
	tb.Cleanup(c.Close)
	return c
}

func Test(t *testing.T) {
	c := newTestChannel(t, WithContext(t.Context()))
	in, out := c.Input(), c.Output()
	in <- 1
	in <- 2
	v := <-out
	assert.Equal(t, v, 1)
	v = <-out
	assert.Equal(t, v, 2)
	select {
	case <-out:
		t.Fatal("should not be here")
	default:
	}
	c.Close()
	_, ok := <-out
	assert.False(t, ok)
}

func TestCloseDiscard(t *testing.T) {
	c := newTestChannel(t, WithBuffer(0))
	in, out := c.Input(), c.Output()
	for range 10 {
		in <- 1
	}
	c.Close()
	count := 0
	for range out {
		count++
	}
	assert.Zero(t, count)
}

func TestCloseSendAll(t *testing.T) {
	c := newTestChannel(t, WithBuffer(0))
	in, out := c.Input(), c.Output()
	for range 10 {
		in <- 1
	}
	close(in)
	count := 0
	for range out {
		count++
	}
	assert.Equal(t, count, 10)
	assert.Equal(t, c.Len(), 0)
}

func TestWithBuffer(t *testing.T) {
	size := 1000
	c := newTestChannel(t, WithBuffer(size))
	in, out := c.Input(), c.Output()
	for range size {
		in <- 1
	}
	close(in)
	count := 0
	for range out {
		count++
	}
	assert.Equal(t, count, size)
}

func TestSlowReceiver(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTestChannel(t, WithBuffer(0))
		in, out := c.Input(), c.Output()
		in <- 1
		time.Sleep(1 * time.Second)
		<-out
	})
}

func Benchmark(b *testing.B) {
	c := newTestChannel(b)
	in, out := c.Input(), c.Output()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		in <- 1
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkEmpty(b *testing.B) {
	c := newTestChannel(b)
	in, out := c.Input(), c.Output()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkEmptyNoBuffer(b *testing.B) {
	c := newTestChannel(b, WithBuffer(0))
	in, out := c.Input(), c.Output()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkWithManyElements(b *testing.B) {
	c := newTestChannel(b)
	in, out := c.Input(), c.Output()
	for range 10000 {
		in <- 1
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkWithBuffer(b *testing.B) {
	for _, buffer := range []int{0, 1, 2, 4, 8, 16, 32, 64, 128} {
		b.Run(strconv.Itoa(buffer), func(b *testing.B) {
			c := newTestChannel(b, WithBuffer(buffer))
			in, out := c.Input(), c.Output()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				in <- 1
				for pb.Next() {
					in <- 1
					<-out
				}
			})
		})
	}
}
