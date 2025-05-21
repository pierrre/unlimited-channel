package unlimitedchannel

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/pierrre/assert"
)

func Example() {
	in, out := New[int]()
	in <- 1
	in <- 2
	v := <-out
	fmt.Println(v)
	v = <-out
	fmt.Println(v)
	close(in)
	_, ok := <-out
	fmt.Println("open:", ok)
	// Output:
	// 1
	// 2
	// open: false
}

func Test(t *testing.T) {
	in, out := New[int](WithContext(t.Context()))
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
	_, ok := <-out
	assert.Equal(t, ok, false)
}

func TestCloseRemaining(t *testing.T) {
	in, out := New[int](WithBuffer(0), WithSendAllOnClose(false))
	for range 10 {
		in <- 1
	}
	close(in)
	count := 0
	for range out {
		count++
	}
	assert.Less(t, count, 10)
}

func TestCloseSendAll(t *testing.T) {
	in, out := New[int](WithBuffer(0), WithSendAllOnClose(true))
	for range 10 {
		in <- 1
	}
	close(in)
	count := 0
	for range out {
		count++
	}
	assert.Equal(t, count, 10)
}

func TestWithBuffer(t *testing.T) {
	size := 1000
	in, out := New[int](WithBuffer(size))
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

func TestWithBufferNegative(t *testing.T) {
	in, out := New[int](WithBuffer(-1))
	in <- 1
	close(in)
	count := 0
	for range out {
		count++
	}
	assert.Equal(t, count, 0)
}

func TestSlowReceiver(t *testing.T) {
	in, out := New[int](WithBuffer(0))
	in <- 1
	time.Sleep(1 * time.Millisecond)
	<-out
	close(in)
}

func Benchmark(b *testing.B) {
	in, out := New[int]()
	defer close(in)
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
	in, out := New[int]()
	defer close(in)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkEmptyNoBuffer(b *testing.B) {
	in, out := New[int](WithBuffer(0))
	defer close(in)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			in <- 1
			<-out
		}
	})
}

func BenchmarkWithManyElements(b *testing.B) {
	in, out := New[int]()
	defer close(in)
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
			in, out := New[int](WithBuffer(buffer))
			defer close(in)
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
