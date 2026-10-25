// Package codec is the migration switch. Application code imports this
// package instead of cgowrap or purego directly, and a build constraint picks
// the implementation:
//
//	go build ./...                  // cgo available: C library (cgowrap)
//	go build -tags purego ./...     // force the pure-Go port
//	CGO_ENABLED=0 go build ./...    // no C toolchain: pure-Go port
//
// During a migration this lets you ship both, flip individual deployments to
// the port with a tag, and finally delete impl_cgo.go and package cgowrap
// without touching any caller.
package codec
