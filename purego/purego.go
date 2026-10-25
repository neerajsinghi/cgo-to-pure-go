// Package purego is the "after" picture of this case study: a line-by-line
// port of the fastcodec C library to Go with no cgo, no unsafe and no
// assembly. Its API is identical to package cgowrap, and differential fuzz
// tests assert that the two produce byte-identical results.
//
// It provides CRC-32C (Castagnoli), unsigned LEB128 varints and a simple
// byte-oriented run-length encoding of (count, value) pairs.
package purego

import (
	"encoding/binary"
	"errors"
	"slices"
)

// MaxVarintLen64 is the maximum encoded length of a 64-bit varint.
const MaxVarintLen64 = 10

// Errors returned by the decoders. The messages match package cgowrap.
var (
	// ErrShort reports that the input ended in the middle of a value.
	ErrShort = errors.New("fastcodec: input too short")
	// ErrOverflow reports a varint that does not fit in 64 bits.
	ErrOverflow = errors.New("fastcodec: varint overflows 64 bits")
	// ErrCorrupt reports a malformed run-length stream.
	ErrCorrupt = errors.New("fastcodec: corrupt rle stream")
)

const castagnoli = 0x82F63B78

// table is computed at package initialisation, replacing the C library's
// explicit fc_init call. Go guarantees init runs once and happens-before any
// use, so there is no data race and no "forgot to call init" failure mode.
var table = makeTable()

func makeTable() *[8][256]uint32 {
	t := new([8][256]uint32)
	for i := range uint32(256) {
		c := i
		for range 8 {
			if c&1 == 1 {
				c = c>>1 ^ castagnoli
			} else {
				c >>= 1
			}
		}
		t[0][i] = c
	}
	for i := range 256 {
		c := t[0][i]
		for j := 1; j < 8; j++ {
			c = t[0][c&0xff] ^ c>>8
			t[j][i] = c
		}
	}
	return t
}

// CRC32C returns the CRC-32C (Castagnoli) checksum of p, continuing from crc.
// Use crc = 0 for a fresh checksum.
func CRC32C(crc uint32, p []byte) uint32 {
	t := table
	crc = ^crc
	for len(p) >= 8 {
		// binary.LittleEndian mirrors the C load32le helper: explicit byte
		// order, independent of the host.
		lo := binary.LittleEndian.Uint32(p) ^ crc
		hi := binary.LittleEndian.Uint32(p[4:])
		crc = t[7][lo&0xff] ^ t[6][lo>>8&0xff] ^ t[5][lo>>16&0xff] ^ t[4][lo>>24] ^
			t[3][hi&0xff] ^ t[2][hi>>8&0xff] ^ t[1][hi>>16&0xff] ^ t[0][hi>>24]
		p = p[8:]
	}
	for _, b := range p {
		crc = t[0][byte(crc)^b] ^ crc>>8
	}
	return ^crc
}

// AppendUvarint appends the unsigned LEB128 encoding of v to dst.
func AppendUvarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

// Uvarint decodes an unsigned LEB128 varint from the start of b and returns
// the value and the number of bytes read.
func Uvarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	for i, c := range b {
		if i == MaxVarintLen64 {
			return 0, 0, ErrOverflow
		}
		if c < 0x80 {
			if i == MaxVarintLen64-1 && c > 1 {
				return 0, 0, ErrOverflow
			}
			return x | uint64(c)<<s, i + 1, nil
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0, ErrShort
}

// AppendRLE appends the run-length encoding of src to dst.
func AppendRLE(dst, src []byte) []byte {
	for i := 0; i < len(src); {
		v := src[i]
		run := 1
		for i+run < len(src) && src[i+run] == v && run < 255 {
			run++
		}
		dst = append(dst, byte(run), v)
		i += run
	}
	return dst
}

// DecodeRLE appends the decoding of the run-length stream src to dst.
func DecodeRLE(dst, src []byte) ([]byte, error) {
	if len(src)%2 != 0 {
		return dst, ErrCorrupt
	}
	n := 0
	for i := 0; i < len(src); i += 2 {
		if src[i] == 0 {
			return dst, ErrCorrupt
		}
		n += int(src[i])
	}
	dst = slices.Grow(dst, n)
	for i := 0; i < len(src); i += 2 {
		v := src[i+1]
		for range src[i] {
			dst = append(dst, v)
		}
	}
	return dst, nil
}
