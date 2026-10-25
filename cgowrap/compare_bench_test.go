//go:build cgo

package cgowrap_test

import (
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/neerajsinghi/cgo-to-pure-go/cgowrap"
	"github.com/neerajsinghi/cgo-to-pure-go/purego"
)

// Side-by-side benchmarks. Sub-benchmark names are impl=<name>/size=<n> so
// the output can be fed straight into benchstat.

var sink uint64

type crcImpl struct {
	name string
	fn   func(uint32, []byte) uint32
}

func BenchmarkCRC32C(b *testing.B) {
	tab := crc32.MakeTable(crc32.Castagnoli)
	impls := []crcImpl{
		{"cgo", cgowrap.CRC32C},
		{"purego", purego.CRC32C},
		// Reference point: the standard library uses SSE4.2/ARMv8 CRC
		// instructions where available.
		{"stdlib", func(c uint32, p []byte) uint32 { return crc32.Update(c, tab, p) }},
	}
	for _, impl := range impls {
		for _, size := range []int{8, 64, 1 << 10, 64 << 10, 1 << 20} {
			buf := make([]byte, size)
			for i := range buf {
				buf[i] = byte(i)
			}
			b.Run(fmt.Sprintf("impl=%s/size=%d", impl.name, size), func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				var c uint32
				for range b.N {
					c = impl.fn(c, buf)
				}
				sink += uint64(c)
			})
		}
	}
}

func BenchmarkUvarint(b *testing.B) {
	type impl struct {
		name   string
		append func([]byte, uint64) []byte
		decode func([]byte) (uint64, int, error)
	}
	impls := []impl{
		{"cgo", cgowrap.AppendUvarint, cgowrap.Uvarint},
		{"purego", purego.AppendUvarint, purego.Uvarint},
	}
	for _, im := range impls {
		b.Run("impl="+im.name+"/op=append", func(b *testing.B) {
			b.ReportAllocs()
			buf := make([]byte, 0, 16)
			for i := range b.N {
				buf = im.append(buf[:0], uint64(i)*0x9E3779B97F4A7C15)
			}
			sink += uint64(len(buf))
		})
		enc := purego.AppendUvarint(nil, 1<<63)
		b.Run("impl="+im.name+"/op=decode", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				v, _, _ := im.decode(enc)
				sink += v
			}
		})
	}
}

func BenchmarkRLE(b *testing.B) {
	type impl struct {
		name   string
		encode func(dst, src []byte) []byte
		decode func(dst, src []byte) ([]byte, error)
	}
	impls := []impl{
		{"cgo", cgowrap.AppendRLE, cgowrap.DecodeRLE},
		{"purego", purego.AppendRLE, purego.DecodeRLE},
	}
	for _, im := range impls {
		for _, size := range []int{16, 1 << 10, 64 << 10} {
			// Runs of 1..16 identical bytes: a mix of short and long runs.
			src := make([]byte, 0, size)
			for i := 0; len(src) < size; i++ {
				for j := 0; j <= i%16 && len(src) < size; j++ {
					src = append(src, byte(i))
				}
			}
			enc := purego.AppendRLE(nil, src)
			b.Run(fmt.Sprintf("impl=%s/op=encode/size=%d", im.name, size), func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				dst := make([]byte, 0, 2*size)
				for range b.N {
					dst = im.encode(dst[:0], src)
				}
				sink += uint64(len(dst))
			})
			b.Run(fmt.Sprintf("impl=%s/op=decode/size=%d", im.name, size), func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				dst := make([]byte, 0, size)
				for range b.N {
					dst, _ = im.decode(dst[:0], enc)
				}
				sink += uint64(len(dst))
			})
		}
	}
}
