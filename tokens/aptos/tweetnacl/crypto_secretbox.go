package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of zero padding bytes in the message for crypto_secretbox.
const SECRETBOX_ZEROBYTES int = 32

// The number of zero padding bytes for a crypto_secretbox ciphertext
const SECRETBOX_BOXZEROBYTES int = 16

// The number of bytes in the nonce used with crypto_secretbox and crypto_secretbox_open.
const SECRETBOX_NONCEBYTES int = 24

// The number of bytes in the secret key used with crypto_secretbox and crypto_secretbox_open.
const SECRETBOX_KEYBYTES int = 32

// Constant zero-filled byte array used for padding messages
var SECRETBOX_PADDING = []byte{0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00}

// Wrapper function for crypto_secretbox.
//
// Encrypts and authenticates a message using the supplied secret key and nonce. The
// zero padding required by crypto_secretbox is added internally and should not be
// included in the supplied message. Likewise the zero padding that prefixes the
// ciphertext returned by the crypto_secretbox C API is stripped from the returned
//  ciphertext.
//
// Ref. http://nacl.cr.yp.to/secretbox.html
func CryptoSecretBox(message, nonce, key []byte) ([]byte, error) {
	if len(nonce) != SECRETBOX_NONCEBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid nonce")
	}

	if len(key) != SECRETBOX_KEYBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid key")
	}

	buffer := make([]byte, len(message)+SECRETBOX_ZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:SECRETBOX_ZEROBYTES], SECRETBOX_PADDING)
	copy(buffer[SECRETBOX_ZEROBYTES:], message)

	rc := C.crypto_secretbox(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(key))

	if rc == 0 {
		return buffer[SECRETBOX_BOXZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error encrypting message (error code %v)", rc)
}

// Wrapper function for crypto_secretbox_open.
//
// Verifies and decrypts the ciphertext using the supplied secret key and nonce. The
// The zero padding required by the crypto_secretbox C API is added internally and
// should not be included in the supplied ciphertext. Likewise the zero padding that
// prefixes the plaintext returned by the crypto_secretbox C API is stripped from the
// returned plaintext.
//
// Ref. http://nacl.cr.yp.to/secretbox.html
func CryptoSecretBoxOpen(ciphertext, nonce, key []byte) ([]byte, error) {
	if len(nonce) != SECRETBOX_NONCEBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid nonce")
	}

	if len(key) != SECRETBOX_KEYBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid key")
	}

	buffer := make([]byte, len(ciphertext)+SECRETBOX_BOXZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:SECRETBOX_BOXZEROBYTES], SECRETBOX_PADDING)
	copy(buffer[SECRETBOX_BOXZEROBYTES:], ciphertext)

	rc := C.crypto_secretbox_open(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(key))

	if rc == 0 {
		return buffer[SECRETBOX_ZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error decrypting message (error code %v)", rc)
}
