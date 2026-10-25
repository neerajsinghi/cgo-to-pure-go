// Command crc32c prints the CRC-32C of each named file (or standard input)
// and which codec backend it was built with. It exists to demonstrate that
// the module builds as a fully static binary once cgo is gone:
//
//	CGO_ENABLED=0 go build -o crc32c ./cmd/crc32c
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/neerajsinghi/cgo-to-pure-go/codec"
)

func checksum(r io.Reader) (uint32, error) {
	var crc uint32
	buf := make([]byte, 64<<10)
	for {
		n, err := r.Read(buf)
		crc = codec.CRC32C(crc, buf[:n])
		if err == io.EOF {
			return crc, nil
		}
		if err != nil {
			return crc, err
		}
	}
}

func main() {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	fmt.Fprintf(os.Stderr, "backend: %s\n", codec.Implementation)

	if len(os.Args) < 2 {
		crc, err := checksum(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(out, "%08x  -\n", crc)
		return
	}
	status := 0
	for _, name := range os.Args[1:] {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 1
			continue
		}
		crc, err := checksum(f)
		f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 1
			continue
		}
		fmt.Fprintf(out, "%08x  %s\n", crc, name)
	}
	if status != 0 {
		out.Flush()
		os.Exit(status)
	}
}
