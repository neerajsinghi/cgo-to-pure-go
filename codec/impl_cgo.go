//go:build cgo && !purego

package codec

import "github.com/neerajsinghi/cgo-to-pure-go/cgowrap"

// Implementation names the backend selected at build time.
const Implementation = "cgo"

// MaxVarintLen64 is the maximum encoded length of a 64-bit varint.
const MaxVarintLen64 = cgowrap.MaxVarintLen64

// Sentinel errors returned by the decoders.
var (
	ErrShort    = cgowrap.ErrShort
	ErrOverflow = cgowrap.ErrOverflow
	ErrCorrupt  = cgowrap.ErrCorrupt
)

// CRC32C returns the CRC-32C checksum of p, continuing from crc.
func CRC32C(crc uint32, p []byte) uint32 { return cgowrap.CRC32C(crc, p) }

// AppendUvarint appends the unsigned LEB128 encoding of v to dst.
func AppendUvarint(dst []byte, v uint64) []byte { return cgowrap.AppendUvarint(dst, v) }

// Uvarint decodes an unsigned LEB128 varint from the start of b.
func Uvarint(b []byte) (uint64, int, error) { return cgowrap.Uvarint(b) }

// AppendRLE appends the run-length encoding of src to dst.
func AppendRLE(dst, src []byte) []byte { return cgowrap.AppendRLE(dst, src) }

// DecodeRLE appends the decoding of the run-length stream src to dst.
func DecodeRLE(dst, src []byte) ([]byte, error) { return cgowrap.DecodeRLE(dst, src) }
