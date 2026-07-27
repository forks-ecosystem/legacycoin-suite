package crypto

import (
	"encoding/hex"
	"testing"
)

func TestYespowerHashKnownVector(t *testing.T) {
	// C yespower_tls with "LegacyCoinPoW" personalization
	// using header.Bytes() serialization (all LE):
	// Version (int32=1):      01 00 00 00  (offset 0-3)
	// PrevBlock (32 zero):    00...00       (offset 4-35)
	// MerkleRoot (32 zero):   00...00       (offset 36-67)
	// Timestamp (uint32=1):   01 00 00 00  (offset 68-71)
	// Bits (uint32=0x207fffff): ff ff 7f 20 (offset 72-75)
	// Nonce (uint32=1):       01 00 00 00  (offset 76-79)
	const pers = "LegacyCoinPoW"
	header := make([]byte, 80)
	header[0] = 0x01
	header[68] = 0x01
	header[72] = 0xff
	header[73] = 0xff
	header[74] = 0x7f
	header[75] = 0x20
	header[76] = 0x01

	hash := YespowerHash(pers, header)
	got := hex.EncodeToString(hash[:])
	want := "d76f6c3dc7a96542d2268b9901fb486bc7fa1d2e1d7db1de41d269cb61e5b0fd"

	if got != want {
		t.Fatalf("YespowerHash mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}
