// Package golden loads the frozen test vectors in testdata/golden.json and
// runs them against any implementation of the fastcodec API. Both cgowrap and
// purego use it, so the vectors are the single source of truth for behaviour.
package golden

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Codec is the API shared by cgowrap and purego.
type Codec struct {
	CRC32C        func(crc uint32, p []byte) uint32
	AppendUvarint func(dst []byte, v uint64) []byte
	Uvarint       func(b []byte) (uint64, int, error)
	AppendRLE     func(dst, src []byte) []byte
	DecodeRLE     func(dst, src []byte) ([]byte, error)

	// Errors maps the golden error names ("short", "overflow", "corrupt")
	// to the implementation's sentinel errors.
	Errors map[string]error
}

type vectors struct {
	CRC32C []struct {
		Name, Input, CRC string
	} `json:"crc32c"`
	Uvarint []struct {
		Name    string
		Value   uint64
		Encoded string
	} `json:"uvarint"`
	UvarintErrors []struct {
		Name, Input, Err string
	} `json:"uvarint_errors"`
	RLE []struct {
		Name, Input, Encoded string
	} `json:"rle"`
	RLEErrors []struct {
		Name, Input, Err string
	} `json:"rle_errors"`
}

// path returns the absolute path of testdata/golden.json, independent of the
// package directory the test runs in.
func path() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "golden.json")
}

func load(t *testing.T) vectors {
	t.Helper()
	raw, err := os.ReadFile(path())
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// decode parses a hex string, or "repeat:HH:N" meaning byte HH repeated N
// times (used to keep long runs readable).
func decode(t *testing.T, s string) []byte {
	t.Helper()
	if rest, ok := strings.CutPrefix(s, "repeat:"); ok {
		b, n, _ := strings.Cut(rest, ":")
		v := decode(t, b)
		count, err := strconv.Atoi(n)
		if err != nil || len(v) != 1 {
			t.Fatalf("bad repeat spec %q", s)
		}
		return bytes.Repeat(v, count)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

// Run checks every golden vector against c.
func Run(t *testing.T, c Codec) {
	v := load(t)

	t.Run("crc32c", func(t *testing.T) {
		for _, tc := range v.CRC32C {
			t.Run(tc.Name, func(t *testing.T) {
				want, err := strconv.ParseUint(tc.CRC, 16, 32)
				if err != nil {
					t.Fatal(err)
				}
				in := decode(t, tc.Input)
				if got := c.CRC32C(0, in); got != uint32(want) {
					t.Errorf("CRC32C(%x) = %08x, want %08x", in, got, want)
				}
				// Incremental updates must match the one-shot result.
				var crc uint32
				for i := range in {
					crc = c.CRC32C(crc, in[i:i+1])
				}
				if crc != uint32(want) {
					t.Errorf("byte-at-a-time CRC32C = %08x, want %08x", crc, want)
				}
			})
		}
	})

	t.Run("uvarint", func(t *testing.T) {
		for _, tc := range v.Uvarint {
			t.Run(tc.Name, func(t *testing.T) {
				want := decode(t, tc.Encoded)
				if got := c.AppendUvarint(nil, tc.Value); !bytes.Equal(got, want) {
					t.Errorf("AppendUvarint(%d) = %x, want %x", tc.Value, got, want)
				}
				// Trailing bytes must be ignored and the length reported.
				got, n, err := c.Uvarint(append(want, 0xAA))
				if err != nil || got != tc.Value || n != len(want) {
					t.Errorf("Uvarint(%x) = %d, %d, %v; want %d, %d, nil", want, got, n, err, tc.Value, len(want))
				}
			})
		}
		for _, tc := range v.UvarintErrors {
			t.Run("error/"+tc.Name, func(t *testing.T) {
				_, _, err := c.Uvarint(decode(t, tc.Input))
				if !errors.Is(err, c.Errors[tc.Err]) {
					t.Errorf("Uvarint(%s) error = %v, want %v", tc.Input, err, c.Errors[tc.Err])
				}
			})
		}
	})

	t.Run("rle", func(t *testing.T) {
		for _, tc := range v.RLE {
			t.Run(tc.Name, func(t *testing.T) {
				in, want := decode(t, tc.Input), decode(t, tc.Encoded)
				enc := c.AppendRLE(nil, in)
				if !bytes.Equal(enc, want) {
					t.Errorf("AppendRLE = %x, want %x", enc, want)
				}
				dec, err := c.DecodeRLE([]byte("prefix"), enc)
				if err != nil || !bytes.Equal(dec, append([]byte("prefix"), in...)) {
					t.Errorf("DecodeRLE round trip = %x, %v", dec, err)
				}
			})
		}
		for _, tc := range v.RLEErrors {
			t.Run("error/"+tc.Name, func(t *testing.T) {
				_, err := c.DecodeRLE(nil, decode(t, tc.Input))
				if !errors.Is(err, c.Errors[tc.Err]) {
					t.Errorf("DecodeRLE(%s) error = %v, want %v", tc.Input, err, c.Errors[tc.Err])
				}
			})
		}
	})
}
