// Package cgowrap is the "before" picture of this case study: a thin cgo
// binding over fastcodec, a small C library vendored in this directory
// (fastcodec.h, fastcodec.c).
//
// It exposes exactly the same API as package purego. When cgo is disabled
// (CGO_ENABLED=0) the package compiles to nothing, which keeps
// `CGO_ENABLED=0 go build ./...` working for the rest of the module.
//
// This file intentionally has no build constraint so the package always has
// at least one Go file.
package cgowrap
