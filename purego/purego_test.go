package purego_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/neerajsinghi/cgo-to-pure-go/internal/golden"
	"github.com/neerajsinghi/cgo-to-pure-go/purego"
)

var codec = golden.Codec{
	CRC32C:        purego.CRC32C,
	AppendUvarint: purego.AppendUvarint,
	Uvarint:       purego.Uvarint,
	AppendRLE:     purego.AppendRLE,
	DecodeRLE:     purego.DecodeRLE,
	Errors: map[string]error{
		"short":    purego.ErrShort,
		"overflow": purego.ErrOverflow,
		"corrupt":  purego.ErrCorrupt,
	},
}

func TestGolden(t *testing.T) { golden.Run(t, codec) }

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// TestCRC32CAgainstStdlib cross-checks the port against hash/crc32 for every
// length around the 8-byte slicing boundary.
func TestCRC32CAgainstStdlib(t *testing.T) {
	buf := make([]byte, 1024)
	for i := range buf {
		buf[i] = byte(i*31 + 7)
	}
	for n := range len(buf) + 1 {
		if got, want := purego.CRC32C(0, buf[:n]), crc32.Checksum(buf[:n], castagnoli); got != want {
			t.Fatalf("len %d: got %08x want %08x", n, got, want)
		}
	}
}

func TestAppendPreservesPrefix(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]byte) []byte
		want []byte
	}{
		{"uvarint", func(b []byte) []byte { return purego.AppendUvarint(b, 300) }, []byte{'x', 0xac, 0x02}},
		{"rle", func(b []byte) []byte { return purego.AppendRLE(b, []byte("zz")) }, []byte{'x', 2, 'z'}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn([]byte{'x'}); !bytes.Equal(got, tc.want) {
				t.Errorf("got %x want %x", got, tc.want)
			}
		})
	}
}

// FuzzCRC32CStdlib checks the port against the standard library.
func FuzzCRC32CStdlib(f *testing.F) {
	f.Add(uint32(0), []byte("123456789"))
	f.Add(uint32(0xdeadbeef), bytes.Repeat([]byte{0xff}, 33))
	f.Fuzz(func(t *testing.T, seed uint32, data []byte) {
		if got, want := purego.CRC32C(seed, data), crc32.Update(seed, castagnoli, data); got != want {
			t.Fatalf("got %08x want %08x", got, want)
		}
	})
}

// FuzzUvarintStdlib checks the varint codec against encoding/binary.
func FuzzUvarintStdlib(f *testing.F) {
	f.Add([]byte{0xac, 0x02})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gn, err := purego.Uvarint(data)
		want, wn := binary.Uvarint(data)
		switch {
		case wn > 0:
			if err != nil || got != want || gn != wn {
				t.Fatalf("got %d,%d,%v want %d,%d", got, gn, err, want, wn)
			}
			if enc := purego.AppendUvarint(nil, got); !bytes.Equal(enc, binary.AppendUvarint(nil, want)) {
				t.Fatalf("AppendUvarint(%d) = %x", got, enc)
			}
		case wn == 0:
			if !errors.Is(err, purego.ErrShort) {
				t.Fatalf("got err %v want ErrShort", err)
			}
		default:
			if !errors.Is(err, purego.ErrOverflow) {
				t.Fatalf("got err %v want ErrOverflow", err)
			}
		}
	})
}

// FuzzRLERoundTrip checks that decoding inverts encoding.
func FuzzRLERoundTrip(f *testing.F) {
	f.Add([]byte("aaaabbbc"))
	f.Add(bytes.Repeat([]byte{0}, 600))
	f.Fuzz(func(t *testing.T, data []byte) {
		dec, err := purego.DecodeRLE(nil, purego.AppendRLE(nil, data))
		if err != nil || !bytes.Equal(dec, data) {
			t.Fatalf("round trip failed: %x, %v", dec, err)
		}
	})
}
