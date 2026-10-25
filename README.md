# cgo-to-pure-go

[![CI](https://github.com/neerajsinghi/cgo-to-pure-go/actions/workflows/ci.yml/badge.svg)](https://github.com/neerajsinghi/cgo-to-pure-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/neerajsinghi/cgo-to-pure-go.svg)](https://pkg.go.dev/github.com/neerajsinghi/cgo-to-pure-go)

A hands-on case study in replacing a cgo dependency with pure Go, without
changing behaviour along the way.

The repo holds a small C library, `fastcodec`, which I wrote for this case
study. It plays the part of a "legacy native dependency". Next to it are a cgo
binding, a line-by-line pure-Go port with the same API, and the safety net
that makes the switch boring: golden vectors, differential fuzzing,
benchmarks and a build-tag switch.

`fastcodec` is small, but every piece of it is a real, well-specified format:

| Function | Format |
|---|---|
| `CRC32C(crc, p)` | CRC-32C (Castagnoli, reflected poly `0x82F63B78`), slicing-by-8. Check value `"123456789"` gives `0xE3069283`. |
| `AppendUvarint(dst, v)` / `Uvarint(b)` | Unsigned LEB128, the same wire format as `encoding/binary` and Protocol Buffers. |
| `AppendRLE(dst, src)` / `DecodeRLE(dst, src)` | Byte run-length encoding: `(count, value)` pairs with `1 <= count <= 255`. |

## Layout

```
cgowrap/              before: the vendored C library + its cgo binding
  fastcodec.h/.c        portable C99, caller-owned buffers, return-code errors
  cgowrap.go            //go:build cgo
  cgowrap_test.go       golden vectors + differential fuzz targets (cgo vs purego)
  compare_bench_test.go side-by-side benchmarks (cgo / purego / stdlib)
purego/               after: the pure-Go port (no cgo, no unsafe, no asm)
codec/                the migration switch: picks a backend by build tags
  impl_cgo.go           //go:build cgo && !purego
  impl_purego.go        //go:build !cgo || purego
internal/golden/      loader + runner for testdata/golden.json
testdata/golden.json  frozen behaviour: CRC, varint and RLE vectors, plus error cases
cmd/crc32c/           tiny CLI used to show static builds
```

```sh
make test          # go test -race ./...  (cgo + pure Go)
make test-purego   # CGO_ENABLED=0 tests, plus the -tags purego switch
make bench         # side-by-side benchmarks
make fuzz          # every differential fuzz target, FUZZTIME each (default 30s)
make build-static  # CGO_ENABLED=0 linux/amd64 static binary
```

## Why remove cgo?

cgo is a fine tool. It is also a tax that every build, every engineer and
every deploy pays, and it adds up:

- **Cross-compilation.** With cgo, `GOOS=linux go build` from a Mac needs a
  Linux C cross-toolchain. Without it, every `GOOS/GOARCH` pair just works.
  In this repo, `CGO_ENABLED=1 GOOS=linux go build ./cmd/crc32c` from macOS
  fails inside `runtime/cgo`. The pure build cross-compiles to linux/arm64 in
  under a second.
- **Static binaries and `FROM scratch`.** A cgo binary links libc
  dynamically by default, so it needs a base image with a matching libc.
  `CGO_ENABLED=0` gives a single static ELF (`make build-static` prints
  `statically linked`) that runs in `scratch` or `distroless/static`.
- **Tooling.** cgo builds need a C compiler on every developer machine and CI
  runner. The race detector cannot see inside C code. pprof shows C frames
  as opaque. Coverage, fuzzing and `go vet` stop at the boundary, and
  memory-safety bugs in C show up as crashes with no Go stack trace.
- **Call overhead.** Every cgo call costs tens of nanoseconds of fixed
  overhead (stack switch, scheduler handoff). For small, frequent calls
  that overhead is the whole cost (see the results below).
- **Hidden allocations.** Passing `&local` to C makes `local` escape to the
  heap (`go build -gcflags=-m ./cgowrap` reports `moved to heap: out`), so
  APIs with out-params allocate on every call.

## Migration recipe

The order matters. Each step ends with a green build, so you can stop at any
point and still be able to ship.

1. **Freeze behaviour with golden vectors.** Before touching anything, write
   down what the current implementation does, including the error paths:
   `testdata/golden.json` holds known-answer vectors (RFC 3720 CRC-32C
   vectors, varint edge cases such as `MaxUint64` and an over-long 10th byte,
   malformed RLE streams). `internal/golden.Run` runs them against any
   implementation, and the cgo binding has to pass them first. That proves
   the vectors describe reality, not intent.
2. **Port.** Translate function by function and keep the structure, so a
   reviewer can read the C and the Go side by side (`purego/purego.go`).
   Optimise later, if at all.
3. **Differential fuzzing.** Native Go fuzz targets feed the same arbitrary
   input to both implementations and require identical values, lengths and
   errors (`cgowrap/cgowrap_test.go`: `FuzzDiffCRC32C`, `FuzzDiffUvarint`,
   `FuzzDiffAppendUvarint`, `FuzzDiffRLE`, `FuzzDiffDecodeRLE`). Their seed
   corpora run on every `go test`. Run `make fuzz` for real exploration. Where
   a reference exists, fuzz against it as well: `purego` is checked against
   `hash/crc32` and `encoding/binary`.
4. **Benchmark.** Measure both implementations across input sizes, with
   `-benchmem`, before deciding anything (results below).
5. **Switch with build tags.** Callers import `codec`, never a backend
   directly. `//go:build cgo && !purego` selects the C library, and
   `//go:build !cgo || purego` selects the port. Roll out `-tags purego`
   per service or environment while both stay available. `CGO_ENABLED=0`
   falls back to the port automatically. CI tests all three configurations.
6. **Remove.** Once the port has run in production long enough, delete
   `codec/impl_cgo.go` and `cgowrap/`, and drop the C toolchain from your
   build images. Keep the golden vectors and the reference fuzz targets for
   good.

## Pitfalls to look for

These are the classes of bug that differential fuzzing exists to catch. Each
one appears, or is deliberately avoided, in this repo.

- **Integer promotion and unsigned semantics.** In C, `p[3] << 24` promotes
  the `uint8_t` to a signed `int`, and shifting into the sign bit is
  undefined behaviour. The C code casts to `uint32_t` first. Go has no
  implicit promotion, so the port must spell out the same widths: `crc` is
  `uint32`, the varint accumulator is `uint64`, and the RLE run counter is
  capped at 255 before it is truncated to a byte. Also watch for `size_t`
  versus `int`. `cap - o < 2` is written that way in C because `o + 2 > cap`
  can wrap.
- **Overflow edge cases are part of the contract.** A 10th varint byte
  greater than 1 is an overflow error, not a silent truncation. A 10-byte
  input whose last byte still has the continuation bit set is "short", not
  "overflow". Both are golden vectors, and both match `encoding/binary`.
- **Endianness.** The C code assembles 32-bit words byte by byte
  (`load32le`) instead of casting `uint8_t*` to `uint32_t*`. A cast would be
  host-endian and possibly misaligned. The port uses
  `binary.LittleEndian.Uint32`, which states the same thing explicitly.
- **Struct packing and layout.** If a C API passes structs, never mirror them
  with a hand-written Go struct and `unsafe.Pointer`: padding and alignment
  differ between ABIs. `fastcodec` sidesteps this with scalar out-params.
  When you port, turn the struct into an ordinary Go type and compare
  field by field in tests.
- **Memory ownership.** C never allocates in `fastcodec`. The caller sizes
  every buffer (`slices.Grow`, plus `fc_rle_decoded_len` to size the decode),
  and C never keeps a pointer after the call returns, which is what the cgo
  pointer-passing rules require. Ports often find hidden ownership rules
  here, for example "the caller must free" or "valid until the next call".
  Write each one down as a test.
- **errno and error mapping.** `fastcodec` returns status codes and never
  touches `errno`. Libraries that do set it need the two-value cgo form
  `n, err := C.f()`, and a port has to reproduce which inputs fail, not just
  that they fail. Both packages export sentinel errors with identical
  messages, and the fuzz targets compare errors as well as values.
- **Global state and initialisation.** The C library needs `fc_init()` to
  build its tables, and calling it concurrently with `fc_crc32c` would be a
  data race. The binding calls it from `init()`. The port uses a package-level
  `var table = makeTable()`, which removes the "forgot to init" failure mode
  entirely.

## Results

Medians of 3 runs (`go test -bench . -benchmem -count 3 ./cgowrap/`), Go 1.27.1
**darwin/amd64**. The amd64 toolchain runs under Rosetta 2 on an Apple M1 Max
(`cpu: VirtualApple @ 2.50GHz`), so treat absolute numbers as indicative.
The cgo rows were noticeably noisier than the pure-Go rows between runs. Raw
output is reproducible with `make bench`.

### CRC-32C

| Input size | cgo | purego | purego vs cgo | stdlib `hash/crc32` (hardware CRC) |
|---:|---:|---:|---:|---:|
| 8 B | 37.7 ns | 6.4 ns | **5.9x faster** | 13.8 ns |
| 64 B | 62.6 ns | 45.2 ns | 1.4x faster | 16.7 ns |
| 1 KiB | 1,201 ns | 717 ns | 1.7x faster | 82 ns |
| 64 KiB | 47.3 µs | 45.8 µs | ~same | 3.0 µs |
| 1 MiB | 738 µs | 734 µs | ~same | 49 µs |

Every CRC row has 0 allocations.

### Varint and RLE

| Benchmark | cgo | purego | cgo allocs | purego allocs |
|---|---:|---:|---:|---:|
| `AppendUvarint` | 38.2 ns | 9.0 ns | 0 | 0 |
| `Uvarint` decode | 48.0 ns | 12.7 ns | 1 (16 B) | 0 |
| RLE encode, 16 B | 60.1 ns | 36.6 ns | 1 (8 B) | 0 |
| RLE encode, 1 KiB | 1,076 ns | 736 ns | 1 (8 B) | 0 |
| RLE encode, 64 KiB | 142 µs (range 41–147 µs) | 49.1 µs | 1 | 0 |
| RLE decode, 16 B | 120 ns | 58.8 ns | 2 (16 B) | 0 |
| RLE decode, 1 KiB | 950 ns | 694 ns | 2 (16 B) | 0 |
| RLE decode, 64 KiB | 50.0 µs | 40.7 µs | 2 | 0 |

### Native arm64, for comparison

The same benchmarks on the same machine with a native darwin/arm64 Go 1.23.0
toolchain (medians of 3): CRC-32C 8 B: cgo 53.9 ns vs purego 5.5 ns. 64 KiB:
cgo 37.0 µs vs purego 41.1 µs. 1 MiB: cgo 566 µs vs purego 636 µs. Uvarint
decode: cgo 71.0 ns vs purego 8.0 ns. Natively, clang's `-O2` loop is about
10% faster than the Go port on large buffers.

### Reading the numbers

- **Small calls: the call overhead dominates.** Under 1 KiB the C code is
  not slow. Crossing the boundary is. A CRC over 8 bytes takes about 6 ns of
  real work, and cgo adds about 30 to 50 ns around it. This is the common
  shape of real workloads: many small checksums, varints or lookups per
  request.
- **Large buffers: roughly a wash.** Once the call is amortised, a faithful
  port of the same algorithm runs within about 10% of the C code. Whether
  the Go port or the C code comes out ahead depends on the target.
- **Allocations disappear.** Out-params passed to C escape to the heap, so
  the cgo `Uvarint` and RLE paths allocate on every call. The pure versions
  allocate nothing.
- **The biggest win came after the port.** Once the code is pure Go, it can
  use what the standard library already provides: `hash/crc32` with
  Castagnoli uses SSE4.2 or ARMv8 CRC instructions and is about 15x faster
  than either table-driven version at 64 KiB. Being able to make that
  change, behind the same golden vectors, is a large part of why the
  migration pays off.

## Adopting this approach in your project

1. Put the cgo dependency behind a package you own, with a Go-shaped API
   (slices in, slices and errors out). If callers already use `C.` types,
   fix that first.
2. Write `testdata/golden.json`-style vectors from the current binary.
   Include error cases and boundary values, and generate a few large or
   random inputs with a fixed seed.
3. Add the port as a sibling package with the same exported API, and a
   shared test runner (like `internal/golden`) so both implementations face
   the same tests.
4. Add differential fuzz targets for every exported function. Seed them with
   the golden inputs and run them in CI for a few minutes nightly.
5. Add the `codec`-style switch with
   `//go:build cgo && !purego` / `//go:build !cgo || purego`, and test both in
   CI, including one `CGO_ENABLED=0` job.
6. Benchmark with realistic call sizes, not only large buffers, and include
   `-benchmem`.
7. Flip the default and delete the cgo path once production agrees with
   your tests.

## Status

v0.1: the C library, the binding, the port, golden vectors, differential
fuzzing, benchmarks and the build-tag switch all work and are tested in CI
(Ubuntu, gcc). Possible next steps: a streaming `hash.Hash32` wrapper, a
nightly fuzzing workflow, and benchstat-formatted results from a native Linux
runner.

## License

[MIT](LICENSE) © 2026 Neeraj Singhi
