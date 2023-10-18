package unlimitedchannel

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/pierrre/assert"
)

func Example() {
	c := new(Channel[int])
	in := c.In()
	out := c.Out()
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
	c := new(Channel[int])
	in := c.In()
	out := c.Out()
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

func Benchmark(b *testing.B) {
	for _, count := range []int{0, 1, 10, 100, 1000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			c := new(Channel[int])
			in := c.In()
			out := c.Out()
			defer close(in)
			for i := 0; i < count; i++ {
				in <- 1
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				in <- 1
				<-out
			}
		})
	}
}
