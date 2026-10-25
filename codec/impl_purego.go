//go:build !cgo || purego

package codec

import "github.com/neerajsinghi/cgo-to-pure-go/purego"

// Implementation names the backend selected at build time.
const Implementation = "purego"

// MaxVarintLen64 is the maximum encoded length of a 64-bit varint.
const MaxVarintLen64 = purego.MaxVarintLen64

// Sentinel errors returned by the decoders.
var (
	ErrShort    = purego.ErrShort
	ErrOverflow = purego.ErrOverflow
	ErrCorrupt  = purego.ErrCorrupt
)

// CRC32C returns the CRC-32C checksum of p, continuing from crc.
func CRC32C(crc uint32, p []byte) uint32 { return purego.CRC32C(crc, p) }

// AppendUvarint appends the unsigned LEB128 encoding of v to dst.
func AppendUvarint(dst []byte, v uint64) []byte { return purego.AppendUvarint(dst, v) }

// Uvarint decodes an unsigned LEB128 varint from the start of b.
func Uvarint(b []byte) (uint64, int, error) { return purego.Uvarint(b) }

// AppendRLE appends the run-length encoding of src to dst.
func AppendRLE(dst, src []byte) []byte { return purego.AppendRLE(dst, src) }

// DecodeRLE appends the decoding of the run-length stream src to dst.
func DecodeRLE(dst, src []byte) ([]byte, error) { return purego.DecodeRLE(dst, src) }
