package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes in a 'secret' for the crypto_verify_16 function.
const VERIFY16_BYTES int = 16

// The number of bytes in a 'secret' for the crypto_verify_32 function.
const VERIFY32_BYTES int = 32

// Wrapper function for crypto_verify_16.
//
// Compares two 'secrets' encoded as 16 byte arrays in a time independent
// of the content of the arrays.
//
// Ref. http://nacl.cr.yp.to/verify.html
func CryptoVerify16(x, y []byte) (bool, error) {
	if len(x) != VERIFY16_BYTES {
		return false, fmt.Errorf("Error verifying 16 byte arrays (%v)", "invalid X")
	}

	if len(y) != VERIFY16_BYTES {
		return false, fmt.Errorf("Error verifying 16 byte arrays (%v)", "invalid Y")
	}

	rc := C.crypto_verify_16(makePtr(x), makePtr(y))

	if rc == 0 {
		return true, nil
	}

	if rc == -1 {
		return false, nil
	}

	return false, fmt.Errorf("Error verifying 16 byte arrays (%v)", rc)
}

// Wrapper function for crypto_verify_32.
//
// Compares two 'secrets' encoded as 32 byte arrays in a time independent
// of the content of the arrays.
//
// Ref. http://nacl.cr.yp.to/verify.html
func CryptoVerify32(x, y []byte) (bool, error) {
	if len(x) != VERIFY32_BYTES {
		return false, fmt.Errorf("Error verifying 32 byte arrays (%v)", "invalid X")
	}

	if len(y) != VERIFY32_BYTES {
		return false, fmt.Errorf("Error verifying 32 byte arrays (%v)", "invalid Y")
	}

	rc := C.crypto_verify_32(makePtr(x), makePtr(y))

	if rc == 0 {
		return true, nil
	}

	if rc == -1 {
		return false, nil
	}

	return false, fmt.Errorf("Error verifying 32 byte arrays (%v)", rc)
}
