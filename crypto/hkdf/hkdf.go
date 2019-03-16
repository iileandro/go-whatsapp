/*
HKDF is a simple key derivation function (KDF) based on
a hash-based message authentication code (HMAC). It was initially proposed by its authors as a building block in
various protocols and applications, as well as to discourage the proliferation of multiple KDF mechanisms.
The main approach HKDF follows is the "extract-then-expand" paradigm, where the KDF logically consists of two modules:
the first stage takes the input keying material and "extracts" from it a fixed-length pseudorandom key, and then the
second stage "expands" this key into several additional pseudorandom keys (the output of the KDF).
*/
package hkdf

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

/*
Expand expands a given key with the HKDF algorithm.
*/
func Expand(key []byte, length int, info string) ([]byte, error) {
	if info == "" {
		keyBlock := hmac.New(sha256.New, key)
		var out, last []byte

		var blockIndex byte = 1
		for i := 0; len(out) < length; i++ {
			keyBlock.Reset()
			//keyBlock.Write(append(append(last, []byte(info)...), blockIndex))
			keyBlock.Write(last)
			keyBlock.Write([]byte(info))
			keyBlock.Write([]byte{blockIndex})
			last = keyBlock.Sum(nil)
			blockIndex += 1
			out = append(out, last...)
		}
		return out[:length], nil
	} else {
		h := hkdf.New(sha256.New, key, nil, []byte(info))
		out := make([]byte, length)
		n, err := io.ReadAtLeast(h, out, length)
		if err != nil {
			return nil, err
		}
		if n != length {
			return nil, fmt.Errorf("new key to short")
		}

		return out[:length], nil
	}
}
