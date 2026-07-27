//go:build !cgo

package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
)

const (
	yespowerN          = 2048
	yespowerR          = 8
	yespowerBlockSize  = 64
	yespowerScratchLen = 128 * yespowerR
)

func YespowerHash(personalization string, input []byte) [32]byte {
	data := append([]byte(personalization), input...)
	B := pbkdf2(data, data, 1, yespowerScratchLen)
	B = smix(B)
	out := pbkdf2(data, B, 1, 32)
	var hash [32]byte
	copy(hash[:], out)
	return hash
}

func pbkdf2(password, salt []byte, iter, keyLen int) []byte {
	if iter <= 0 {
		iter = 1
	}
	h := hmac.New(sha256.New, password)
	hSize := h.Size()
	numBlocks := (keyLen + hSize - 1) / hSize

	dk := make([]byte, 0, numBlocks*hSize)
	block := make([]byte, len(salt)+4)
	copy(block, salt)

	for blockNum := 1; blockNum <= numBlocks; blockNum++ {
		binary.BigEndian.PutUint32(block[len(salt):], uint32(blockNum))
		h.Reset()
		h.Write(block)
		T := h.Sum(nil)
		U := make([]byte, hSize)
		copy(U, T)

		for i := 1; i < iter; i++ {
			h.Reset()
			h.Write(U)
			U = h.Sum(nil)
			for j := 0; j < hSize; j++ {
				T[j] ^= U[j]
			}
		}
		dk = append(dk, T...)
	}
	return dk[:keyLen]
}

func smix(B []byte) []byte {
	V := make([]byte, yespowerN*yespowerScratchLen)
	X := make([]byte, yespowerScratchLen)
	copy(X, B)

	for i := 0; i < yespowerN; i++ {
		copy(V[i*yespowerScratchLen:(i+1)*yespowerScratchLen], X)
		blockMix(X)
	}

	for i := 0; i < yespowerN; i++ {
		j := integerify(X) & (yespowerN - 1)
		xorBlock(X, V[j*yespowerScratchLen:(j+1)*yespowerScratchLen])
		blockMix(X)
	}

	return X
}

func blockMix(B []byte) {
	X := make([]byte, yespowerBlockSize)
	copy(X, B[(2*yespowerR-1)*yespowerBlockSize:])

	Y := make([]byte, yespowerScratchLen)
	for i := 0; i < 2*yespowerR; i++ {
		for j := 0; j < yespowerBlockSize; j++ {
			X[j] ^= B[i*yespowerBlockSize+j]
		}
		salsa20_8(X)
		if i%2 == 0 {
			copy(Y[(i/2)*yespowerBlockSize:], X)
		} else {
			copy(Y[(yespowerR+i/2)*yespowerBlockSize:], X)
		}
	}
	copy(B, Y)
}

func salsa20_8(out []byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(out[4*i : 4*i+4])
	}

	var x0 [16]uint32
	copy(x0[:], x[:])

	for d := 0; d < 4; d++ {
		x[4] ^= rotl(x[0]+x[12], 7)
		x[8] ^= rotl(x[4]+x[0], 9)
		x[12] ^= rotl(x[8]+x[4], 13)
		x[0] ^= rotl(x[12]+x[8], 18)

		x[9] ^= rotl(x[5]+x[1], 7)
		x[13] ^= rotl(x[9]+x[5], 9)
		x[1] ^= rotl(x[13]+x[9], 13)
		x[5] ^= rotl(x[1]+x[13], 18)

		x[14] ^= rotl(x[10]+x[6], 7)
		x[2] ^= rotl(x[14]+x[10], 9)
		x[6] ^= rotl(x[2]+x[14], 13)
		x[10] ^= rotl(x[6]+x[2], 18)

		x[3] ^= rotl(x[15]+x[11], 7)
		x[7] ^= rotl(x[3]+x[15], 9)
		x[11] ^= rotl(x[7]+x[3], 13)
		x[15] ^= rotl(x[11]+x[7], 18)

		x[1] ^= rotl(x[0]+x[3], 7)
		x[2] ^= rotl(x[1]+x[0], 9)
		x[3] ^= rotl(x[2]+x[1], 13)
		x[0] ^= rotl(x[3]+x[2], 18)

		x[6] ^= rotl(x[5]+x[4], 7)
		x[7] ^= rotl(x[6]+x[5], 9)
		x[4] ^= rotl(x[7]+x[6], 13)
		x[5] ^= rotl(x[4]+x[7], 18)

		x[11] ^= rotl(x[10]+x[9], 7)
		x[8] ^= rotl(x[11]+x[10], 9)
		x[9] ^= rotl(x[8]+x[11], 13)
		x[10] ^= rotl(x[9]+x[8], 18)

		x[12] ^= rotl(x[15]+x[14], 7)
		x[13] ^= rotl(x[12]+x[15], 9)
		x[14] ^= rotl(x[13]+x[12], 13)
		x[15] ^= rotl(x[14]+x[13], 18)
	}

	for i := 0; i < 16; i++ {
		x[i] += x0[i]
	}

	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(out[4*i:4*i+4], x[i])
	}
}

func rotl(x uint32, b uint) uint32 {
	return (x << b) | (x >> (32 - b))
}

func integerify(B []byte) int {
	start := len(B) - yespowerBlockSize
	return int(binary.LittleEndian.Uint64(B[start : start+8]))
}

func xorBlock(dst, src []byte) {
	for i := range dst {
		dst[i] ^= src[i]
	}
}
