//go:build darwin

package main

// Link the UniformTypeIdentifiers framework required by Wails on macOS 11+.

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"
