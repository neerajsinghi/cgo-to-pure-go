//go:build cgo

package cgowrap

/*
#cgo CFLAGS: -O2 -std=c99
#include "fastcodec.h"
*/
import "C"

import (
	"errors"
	"slices"
	"strconv"
	"unsafe"
)

// MaxVarintLen64 is the maximum encoded length of a 64-bit varint.
const MaxVarintLen64 = C.FC_MAX_VARINT_LEN64

// Errors returned by the decoders. The messages are identical to the ones in
// package purego so that the two implementations are observably equivalent.
var (
	// ErrShort reports that the input ended in the middle of a value.
	ErrShort = errors.New("fastcodec: input too short")
	// ErrOverflow reports a varint that does not fit in 64 bits.
	ErrOverflow = errors.New("fastcodec: varint overflows 64 bits")
	// ErrCorrupt reports a malformed run-length stream.
	ErrCorrupt = errors.New("fastcodec: corrupt rle stream")
)

func init() {
	// The C library keeps its CRC tables in static storage and requires
	// explicit initialisation; package init runs exactly once, before any
	// goroutine can call into this package.
	C.fc_init()
}

// statusErr maps a C status code to a Go error. The C library never uses
// errno, so there is no need for the two-value form of cgo calls.
func statusErr(rc C.int) error {
	switch rc {
	case C.FC_OK:
		return nil
	case C.FC_ERR_SHORT:
		return ErrShort
	case C.FC_ERR_OVERFLOW:
		return ErrOverflow
	case C.FC_ERR_CORRUPT:
		return ErrCorrupt
	default:
		// FC_ERR_DST cannot happen: every caller sizes dst first.
		panic("cgowrap: unexpected fastcodec status " + strconv.Itoa(int(rc)))
	}
}

// cbytes returns a C pointer to the backing array of b, or nil for a slice
// with zero capacity. Passing Go memory to C is allowed by the cgo pointer
// rules because a []byte contains no Go pointers and C does not retain it.
func cbytes(b []byte) *C.uint8_t {
	return (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(b)))
}

// CRC32C returns the CRC-32C (Castagnoli) checksum of p, continuing from crc.
// Use crc = 0 for a fresh checksum.
func CRC32C(crc uint32, p []byte) uint32 {
	return uint32(C.fc_crc32c(C.uint32_t(crc), cbytes(p), C.size_t(len(p))))
}

// AppendUvarint appends the unsigned LEB128 encoding of v to dst.
func AppendUvarint(dst []byte, v uint64) []byte {
	dst = slices.Grow(dst, MaxVarintLen64)
	tail := dst[len(dst):cap(dst)]
	n := C.fc_uvarint_put(cbytes(tail), C.uint64_t(v))
	return dst[:len(dst)+int(n)]
}

// Uvarint decodes an unsigned LEB128 varint from the start of b and returns
// the value and the number of bytes read.
func Uvarint(b []byte) (uint64, int, error) {
	var out C.uint64_t
	var used C.size_t
	if err := statusErr(C.fc_uvarint_get(cbytes(b), C.size_t(len(b)), &out, &used)); err != nil {
		return 0, 0, err
	}
	return uint64(out), int(used), nil
}

// AppendRLE appends the run-length encoding of src to dst.
func AppendRLE(dst, src []byte) []byte {
	dst = slices.Grow(dst, 2*len(src))
	tail := dst[len(dst):cap(dst)]
	var written C.size_t
	rc := C.fc_rle_encode(cbytes(tail), C.size_t(len(tail)), cbytes(src), C.size_t(len(src)), &written)
	if err := statusErr(rc); err != nil {
		panic(err)
	}
	return dst[:len(dst)+int(written)]
}

// DecodeRLE appends the decoding of the run-length stream src to dst.
func DecodeRLE(dst, src []byte) ([]byte, error) {
	var n C.size_t
	if err := statusErr(C.fc_rle_decoded_len(cbytes(src), C.size_t(len(src)), &n)); err != nil {
		return dst, err
	}
	dst = slices.Grow(dst, int(n))
	tail := dst[len(dst):cap(dst)]
	var written C.size_t
	rc := C.fc_rle_decode(cbytes(tail), C.size_t(len(tail)), cbytes(src), C.size_t(len(src)), &written)
	if err := statusErr(rc); err != nil {
		return dst, err
	}
	return dst[:len(dst)+int(written)], nil
}
