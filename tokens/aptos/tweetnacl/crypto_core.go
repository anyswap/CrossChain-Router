package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes in an HSALSA20 intermediate key.
const HSALSA20_OUTPUTBYTES int = 32

// The number of bytes in the shared secret for crypto_core_hsalsa20.
const HSALSA20_INPUTBYTES int = 16

// The number of bytes in the secret keyfor crypto_core_hsalsa20.
const HSALSA20_KEYBYTES int = 32

// The number of bytes in the constant for crypto_core_hsalsa20.
const HSALSA20_CONSTBYTES int = 16

// The number of bytes in the calculated intermediate key.
const SALSA20_OUTPUTBYTES int = 64

// The number of bytes in the shared secret for crypto_core_salsa20.
const SALSA20_INPUTBYTES int = 16

// The number of bytes in the secret key for crypto_core_salsa20.
const SALSA20_KEYBYTES int = 32

// The number of bytes in the constant for crypto_core_salsa20.
const SALSA20_CONSTBYTES int = 16

// Wrapper function for crypto_core_hsalsa20.
//
// From the available documentation crypto_core_hsalsa20 apparently calculates an
// intermediate key (from a secret key and shared secret) for encrypting and
// authenticating packets.
//
// in is a HSALSA20_INPUTBYTES byte array containing the shared secret.
// key is a HSALSA20_KEYBYTES byte array containing the secret key.
// constant is a HSALSA20_CONSTBYTES byte array containing an apparently arbitrary 'constant'
// (IV ?) to be used for the intermediate key calculation.
//
func CryptoCoreHSalsa20(in, key, constant []byte) ([]byte, error) {
	if len(in) != HSALSA20_INPUTBYTES {
		return nil, fmt.Errorf("Error calculating HSALSA20 intermediate key (%v)", "invalid shared secret")
	}

	if len(key) != HSALSA20_KEYBYTES {
		return nil, fmt.Errorf("Error calculating HSALSA20 intermediate key (%v)", "invalid secret key")
	}

	if len(constant) != HSALSA20_CONSTBYTES {
		return nil, fmt.Errorf("Error calculating HSALSA20 intermediate key (%v)", "invalid constant")
	}

	out := make([]byte, HSALSA20_OUTPUTBYTES)

	rc := C.crypto_core_hsalsa20(makePtr(out),
		makePtr(in),
		makePtr(key),
		makePtr(constant))

	if rc == 0 {
		return out, nil
	}

	return nil, fmt.Errorf("Error calculating HSALSA20 intermediate key (error code %v)", rc)
}

// Wrapper function for crypto_core_salsa20.
//
// From the available documentation crypto_core_salsa20 apparently calculates an
// intermediate key (from a secret key and shared secret) for encrypting and
// authenticating packets.
//
func CryptoCoreSalsa20(in, key, constant []byte) ([]byte, error) {
	if len(in) != SALSA20_INPUTBYTES {
		return nil, fmt.Errorf("Error calculating SALSA20 intermediate key (%v)", "invalid shared secret")
	}

	if len(key) != SALSA20_KEYBYTES {
		return nil, fmt.Errorf("Error calculating SALSA20 intermediate key (%v)", "invalid secret key")
	}

	if len(constant) != SALSA20_CONSTBYTES {
		return nil, fmt.Errorf("Error calculating SALSA20 intermediate key (%v)", "invalid constant")
	}

	out := make([]byte, SALSA20_OUTPUTBYTES)

	rc := C.crypto_core_salsa20(makePtr(out),
		makePtr(in),
		makePtr(key),
		makePtr(constant))

	if rc == 0 {
		return out, nil
	}

	return nil, fmt.Errorf("Error calculating SALSA20 intermediate key (error code %v)", rc)
}
