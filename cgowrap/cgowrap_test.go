//go:build cgo

package cgowrap_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/neerajsinghi/cgo-to-pure-go/cgowrap"
	"github.com/neerajsinghi/cgo-to-pure-go/internal/golden"
	"github.com/neerajsinghi/cgo-to-pure-go/purego"
)

var codec = golden.Codec{
	CRC32C:        cgowrap.CRC32C,
	AppendUvarint: cgowrap.AppendUvarint,
	Uvarint:       cgowrap.Uvarint,
	AppendRLE:     cgowrap.AppendRLE,
	DecodeRLE:     cgowrap.DecodeRLE,
	Errors: map[string]error{
		"short":    cgowrap.ErrShort,
		"overflow": cgowrap.ErrOverflow,
		"corrupt":  cgowrap.ErrCorrupt,
	},
}

// TestGolden runs the same frozen vectors that package purego runs.
func TestGolden(t *testing.T) { golden.Run(t, codec) }

// sameErr reports whether two errors from the two implementations are
// equivalent: both nil, or the same sentinel by message.
func sameErr(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}

// TestSentinelsMatch guards the error contract: callers switching packages
// must see the same messages.
func TestSentinelsMatch(t *testing.T) {
	tests := []struct{ c, g error }{
		{cgowrap.ErrShort, purego.ErrShort},
		{cgowrap.ErrOverflow, purego.ErrOverflow},
		{cgowrap.ErrCorrupt, purego.ErrCorrupt},
	}
	for _, tc := range tests {
		if !sameErr(tc.c, tc.g) {
			t.Errorf("%q != %q", tc.c, tc.g)
		}
	}
	if cgowrap.MaxVarintLen64 != purego.MaxVarintLen64 {
		t.Errorf("MaxVarintLen64 mismatch")
	}
}

// The fuzz targets below are the heart of the migration: for arbitrary input,
// the pure-Go port must be indistinguishable from the C library.

func FuzzDiffCRC32C(f *testing.F) {
	f.Add(uint32(0), []byte(""))
	f.Add(uint32(0), []byte("123456789"))
	f.Add(uint32(0xffffffff), bytes.Repeat([]byte{0xa5}, 67))
	f.Fuzz(func(t *testing.T, seed uint32, data []byte) {
		if c, g := cgowrap.CRC32C(seed, data), purego.CRC32C(seed, data); c != g {
			t.Fatalf("CRC32C(%08x, %x): cgo %08x, purego %08x", seed, data, c, g)
		}
	})
}

func FuzzDiffAppendUvarint(f *testing.F) {
	for _, v := range []uint64{0, 1, 127, 128, 1<<63 - 1, 1<<64 - 1} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v uint64) {
		if c, g := cgowrap.AppendUvarint(nil, v), purego.AppendUvarint(nil, v); !bytes.Equal(c, g) {
			t.Fatalf("AppendUvarint(%d): cgo %x, purego %x", v, c, g)
		}
	})
}

func FuzzDiffUvarint(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xac, 0x02, 0xff})
	f.Add(bytes.Repeat([]byte{0x80}, 11))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		cv, cn, cerr := cgowrap.Uvarint(data)
		gv, gn, gerr := purego.Uvarint(data)
		if cv != gv || cn != gn || !sameErr(cerr, gerr) {
			t.Fatalf("Uvarint(%x): cgo (%d,%d,%v), purego (%d,%d,%v)", data, cv, cn, cerr, gv, gn, gerr)
		}
	})
}

func FuzzDiffRLE(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("aaab"))
	f.Add(bytes.Repeat([]byte{'z'}, 511))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, g := cgowrap.AppendRLE(nil, data), purego.AppendRLE(nil, data)
		if !bytes.Equal(c, g) {
			t.Fatalf("AppendRLE(%x): cgo %x, purego %x", data, c, g)
		}
		dec, err := cgowrap.DecodeRLE(nil, c)
		if err != nil || !bytes.Equal(dec, data) {
			t.Fatalf("cgo round trip of %x: %x, %v", data, dec, err)
		}
	})
}

// FuzzDiffDecodeRLE feeds arbitrary (mostly invalid) streams to both decoders.
func FuzzDiffDecodeRLE(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte{3, 'a', 1, 'b'})
	f.Add([]byte{0, 'a'})
	f.Add([]byte{1})
	f.Fuzz(func(t *testing.T, data []byte) {
		c, cerr := cgowrap.DecodeRLE([]byte("p"), data)
		g, gerr := purego.DecodeRLE([]byte("p"), data)
		if !bytes.Equal(c, g) || !sameErr(cerr, gerr) {
			t.Fatalf("DecodeRLE(%x): cgo (%x,%v), purego (%x,%v)", data, c, cerr, g, gerr)
		}
		if cerr != nil && !errors.Is(cerr, cgowrap.ErrCorrupt) {
			t.Fatalf("unexpected error %v", cerr)
		}
	})
}
