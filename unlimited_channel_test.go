package unlimitedchannel

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
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
	close(in)
	_, ok := <-out
	fmt.Println("open:", ok)
	c.Wait()
	// Output:
	// 1
	// 2
	// open: false
}

func newTestChannel(tb testing.TB, opts ...Option) *Channel[int] {
	tb.Helper()
	var release func()
	c := New[int](slices.Concat([]Option{withRelease(&release)}, opts)...)
	tb.Cleanup(release)
	return c
}

func Test(t *testing.T) {
	c := newTestChannel(t)
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
	close(in)
	c.Wait()
	_, ok := <-out
	assert.Equal(t, ok, false)
	assert.Equal(t, c.Len(), 0)
}

func TestCloseRemaining(t *testing.T) {
	c := newTestChannel(t, WithBuffer(0), WithSendAllOnClose(false))
	in, out := c.Input(), c.Output()
	for range 10 {
		in <- 1
	}
	time.Sleep(1 * time.Millisecond) // Ensure that it's sending to the output. TODO: use testing/synctest with Go 1.25.
	close(in)
	c.Wait()
	_, ok := <-out
	assert.Equal(t, ok, false)
	assert.Equal(t, c.Len(), 10)
}

func TestCloseSendAll(t *testing.T) {
	c := newTestChannel(t, WithBuffer(0), WithSendAllOnClose(true))
	in, out := c.Input(), c.Output()
	for range 10 {
		in <- 1
	}
	close(in)
	count := 0
	for range out {
		count++
	}
	c.Wait()
	assert.Equal(t, count, 10)
	assert.Equal(t, c.Len(), 0)
}

func TestWithRelease(t *testing.T) {
	c := newTestChannel(t, WithBuffer(0), WithSendAllOnClose(true))
	in := c.Input()
	in <- 1
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
