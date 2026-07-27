package crypto

import (
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck
)

// RIPEMD160 computes the RIPEMD-160 hash of data.
func RIPEMD160(data []byte) []byte {
	h := ripemd160.New()
	h.Write(data)
	return h.Sum(nil)
}
