package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
)

// KeyPair holds a private/public key pair.
type KeyPair struct {
	PrivKey *btcec.PrivateKey
	PubKey  *btcec.PublicKey
}

// GenerateKeyPair generates a new secp256k1 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("key generation failed: %w", err)
	}
	return &KeyPair{
		PrivKey: privKey,
		PubKey:  privKey.PubKey(),
	}, nil
}

// FromPrivKeyBytes restores a KeyPair from raw 32-byte private key.
func FromPrivKeyBytes(b []byte) (*KeyPair, error) {
	if len(b) != 32 {
		return nil, errors.New("private key must be 32 bytes")
	}
	privKey, pubKey := btcec.PrivKeyFromBytes(b)
	return &KeyPair{PrivKey: privKey, PubKey: pubKey}, nil
}

// PrivKeyBytes returns the 32-byte private key.
func (kp *KeyPair) PrivKeyBytes() []byte {
	return kp.PrivKey.Serialize()
}

// PubKeyBytes returns the compressed 33-byte public key.
func (kp *KeyPair) PubKeyBytes() []byte {
	return kp.PubKey.SerializeCompressed()
}

// PubKeyHash returns RIPEMD160(SHA256(pubkey)).
func (kp *KeyPair) PubKeyHash() []byte {
	h := sha256.Sum256(kp.PubKeyBytes())
	return RIPEMD160(h[:])
}

// Address returns the LegacyCoin address (Base58Check of version+pubKeyHash).
func (kp *KeyPair) Address() string {
	return PubKeyHashToAddress(kp.PubKeyHash())
}

// PubKeyHashToAddress encodes a pubkey hash as a LegacyCoin address.
func PubKeyHashToAddress(pubKeyHash []byte) string {
	// version byte 0x00 (same as Bitcoin mainnet P2PKH)
	versioned := append([]byte{0x00}, pubKeyHash...)
	checksum := checksum4(versioned)
	return Base58Encode(append(versioned, checksum...))
}

// AddressToPubKeyHash decodes a LegacyCoin address to its pubkey hash.
func AddressToPubKeyHash(addr string) ([]byte, error) {
	decoded, err := Base58Decode(addr)
	if err != nil {
		return nil, err
	}
	if len(decoded) != 25 {
		return nil, errors.New("invalid address length")
	}
	payload := decoded[:21]
	gotChecksum := decoded[21:]
	wantChecksum := checksum4(payload)
	for i := range gotChecksum {
		if gotChecksum[i] != wantChecksum[i] {
			return nil, errors.New("address checksum mismatch")
		}
	}
	if payload[0] != 0x00 {
		return nil, errors.New("unknown address version byte")
	}
	return payload[1:], nil
}

func checksum4(data []byte) []byte {
	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])
	return h2[:4]
}

// Sign signs a 32-byte hash with the private key (DER-encoded).
func (kp *KeyPair) Sign(hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, errors.New("hash must be 32 bytes")
	}
	sig := ecdsa.Sign(kp.PrivKey, hash)
	return sig.Serialize(), nil
}

// Verify verifies a DER-encoded signature against a hash and compressed pubkey.
func Verify(pubKeyBytes, hash, sigBytes []byte) bool {
	pubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return false
	}
	sig, err := ecdsa.ParseDERSignature(sigBytes)
	if err != nil {
		return false
	}
	return sig.Verify(hash, pubKey)
}

// RandomBytes returns n cryptographically random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

// HexToBytes decodes a hex string.
func HexToBytes(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
