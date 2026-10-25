package codec_test

import (
	"testing"

	"github.com/neerajsinghi/cgo-to-pure-go/codec"
	"github.com/neerajsinghi/cgo-to-pure-go/internal/golden"
)

// TestGolden runs the frozen vectors against whichever backend the build
// constraints selected. CI runs it both with cgo and with CGO_ENABLED=0.
func TestGolden(t *testing.T) {
	t.Logf("implementation: %s", codec.Implementation)
	golden.Run(t, golden.Codec{
		CRC32C:        codec.CRC32C,
		AppendUvarint: codec.AppendUvarint,
		Uvarint:       codec.Uvarint,
		AppendRLE:     codec.AppendRLE,
		DecodeRLE:     codec.DecodeRLE,
		Errors: map[string]error{
			"short":    codec.ErrShort,
			"overflow": codec.ErrOverflow,
			"corrupt":  codec.ErrCorrupt,
		},
	})
}
