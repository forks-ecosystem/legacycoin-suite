package crypto

import (
	"errors"
	"math/big"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var bigZero = big.NewInt(0)
var bigBase = big.NewInt(58)

// Base58Encode encodes bytes to Base58.
func Base58Encode(input []byte) string {
	n := new(big.Int).SetBytes(input)
	var result []byte
	mod := new(big.Int)
	for n.Cmp(bigZero) > 0 {
		n.DivMod(n, bigBase, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}
	// Add leading '1' for each leading zero byte
	for _, b := range input {
		if b != 0x00 {
			break
		}
		result = append(result, base58Alphabet[0])
	}
	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

// Base58Decode decodes a Base58-encoded string to bytes.
func Base58Decode(input string) ([]byte, error) {
	n := big.NewInt(0)
	for _, c := range input {
		idx := strings.IndexRune(base58Alphabet, c)
		if idx < 0 {
			return nil, errors.New("invalid base58 character")
		}
		n.Mul(n, bigBase)
		n.Add(n, big.NewInt(int64(idx)))
	}
	decoded := n.Bytes()
	// Add leading zero bytes
	var leadingZeros int
	for _, c := range input {
		if c != rune(base58Alphabet[0]) {
			break
		}
		leadingZeros++
	}
	result := make([]byte, leadingZeros+len(decoded))
	copy(result[leadingZeros:], decoded)
	return result, nil
}
