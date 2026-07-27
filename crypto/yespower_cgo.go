//go:build cgo

package crypto

/*
#cgo CFLAGS: -O3 -fPIC
#cgo LDFLAGS: -lm
#include <string.h>
#include "yespower.h"

static void yespower_hash_legacy(const unsigned char *input, size_t inputlen, unsigned char *output) {
    yespower_params_t params = {
        YESPOWER_1_0,
        2048,
        32,
        (const unsigned char *)"LegacyCoinPoW",
        13
    };
    yespower_binary_t result;
    yespower_tls(input, inputlen, &params, &result);
    memcpy(output, result.uc, 32);
}
*/
import "C"
import "unsafe"

func YespowerHash(personalization string, input []byte) [32]byte {
	var hash [32]byte
	C.yespower_hash_legacy(
		(*C.uchar)(unsafe.Pointer(&input[0])),
		C.size_t(len(input)),
		(*C.uchar)(unsafe.Pointer(&hash[0])),
	)
	return hash
}
