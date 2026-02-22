package utils

import (
	"unsafe"
)

// UnsafeBytesToString converts a byte slice to string without allocation.
// The returned string must not be used after b is modified.
func UnsafeBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// UnsafeStringToBytes converts a string to a byte slice without allocation.
// The returned slice must be treated as read-only to avoid breaking string immutability.
func UnsafeStringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}
