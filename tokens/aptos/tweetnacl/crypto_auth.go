package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes in the authenticator.
const ONETIMEAUTH_BYTES int = 16

// The number of bytes in the secret key used to generate the authenticator.
const ONETIMEAUTH_KEYBYTES int = 32

// Wrapper function for crypto_onetimeauth.
//
// Uses the supplied secret key to calculate an authenticator for the message.
//
// Ref. http://nacl.cr.yp.to/onetimeauth.html
func CryptoOneTimeAuth(message, key []byte) ([]byte, error) {

	if len(key) != ONETIMEAUTH_KEYBYTES {
		return nil, fmt.Errorf("Error calculating one time authenticator (%v)", "invalid key")
	}

	authenticator := make([]byte, ONETIMEAUTH_BYTES)
	N := (C.ulonglong)(len(message))

	rc := C.crypto_onetimeauth(makePtr(authenticator),
		makePtr(message),
		N,
		makePtr(key))

	if rc == 0 {
		return authenticator, nil
	}

	return nil, fmt.Errorf("Error calculating one time authenticator (%v)", rc)
}

// Wrapper function for crypto_onetimeauth_verify.
//
// Uses the supplied secret key to verify the authenticator for the message.
//
// Ref. http://nacl.cr.yp.to/onetimeauth.html
func CryptoOneTimeAuthVerify(authenticator, message, key []byte) (bool, error) {

	if len(authenticator) != ONETIMEAUTH_BYTES {
		return false, fmt.Errorf("Error verifying one time authenticator (%v)", "invalid authenticator")
	}

	if len(key) != ONETIMEAUTH_KEYBYTES {
		return false, fmt.Errorf("Error verifying one time authenticator (%v)", "invalid key")
	}

	N := (C.ulonglong)(len(message))

	rc := C.crypto_onetimeauth_verify(makePtr(authenticator),
		makePtr(message),
		N,
		makePtr(key))

	if rc == 0 {
		return true, nil
	}

	return false, fmt.Errorf("Error verifying one time authenticator (error code %v)", rc)
}
