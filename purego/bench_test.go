package purego_test

import (
	"fmt"
	"testing"

	"github.com/neerajsinghi/cgo-to-pure-go/purego"
)

// These benchmarks build without cgo. The side-by-side comparison against the
// C library lives in package cgowrap.

func BenchmarkCRC32C(b *testing.B) {
	for _, size := range []int{8, 64, 1 << 10, 64 << 10} {
		buf := make([]byte, size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for range b.N {
				purego.CRC32C(0, buf)
			}
		})
	}
}
