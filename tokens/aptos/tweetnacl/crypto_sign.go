package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes added to a message for a signature.
const SIGN_BYTES int = 64

// The number of bytes in a signing key pair public key.
const SIGN_PUBLICKEYBYTES int = 32

// The number of bytes in a signing key pair secret key.
const SIGN_SECRETKEYBYTES int = 64

// Wrapper function for crypto_sign_keypair.
//
// Randomly generates a secret key and corresponding public key.
//
// Ref. http://nacl.cr.yp.to/sign.html
func CryptoSignKeyPair() (*KeyPair, error) {
	pk := make([]byte, SIGN_PUBLICKEYBYTES)
	sk := make([]byte, SIGN_SECRETKEYBYTES)

	rc := C.crypto_sign_keypair(makePtr(pk), makePtr(sk))

	if rc == 0 {
		return &KeyPair{PublicKey: pk, SecretKey: sk}, nil
	}

	return nil, fmt.Errorf("Error generating signing key pair (%v)", rc)
}

func CryptoSignKeyPairFromSeed(seed []byte) (*KeyPair, error) {
	if len(seed) != SIGN_PUBLICKEYBYTES {
		return nil, fmt.Errorf("Error seed (%v)", "invalid seed")
	}

	pk := make([]byte, SIGN_PUBLICKEYBYTES)
	sk := make([]byte, SIGN_SECRETKEYBYTES)

	for i := 0; i < 32; i++ {
		sk[i] = seed[i]
	}

	rc := C.crypto_sign_keypair_seed(makePtr(pk), makePtr(sk))

	if rc == 0 {
		return &KeyPair{PublicKey: pk, SecretKey: sk}, nil
	}

	return nil, fmt.Errorf("Error generating signing key pair (%v)", rc)
}

// Wrapper function for crypto_sign.
//
// Signs a message using a secret signing key and returns the signed message. Be
// aware that this function internally allocates a buffer the same size as the
// signed message.
//
//
//
// Ref. http://nacl.cr.yp.to/sign.html
func CryptoSign(message, key []byte) ([]byte, error) {
	if len(key) != SIGN_SECRETKEYBYTES {
		return nil, fmt.Errorf("Error signing message (%v)", "invalid secret key")
	}

	signed := make([]byte, len(message)+SIGN_BYTES)
	N := uint64(len(signed))
	M := len(message)

	rc := C.crypto_sign(
		makePtr(signed),
		(*C.ulonglong)(&N),
		makePtr(message),
		(C.ulonglong)(M),
		makePtr(key))

	if rc == 0 {
		return signed, nil
	}

	return nil, fmt.Errorf("Error signing message (%v)", rc)
}

// Wrapper function for crypto_sign_open.
//
// Verifies a signed message against a public key. Be warned that this function
// reuses the 'signed' message to store the unsigned message i.e. use a copy
// of the signed message if retaining it is important.
//
// Ref. http://nacl.cr.yp.to/sign.html
func CryptoSignOpen(signed, key []byte) ([]byte, error) {
	if len(key) != SIGN_PUBLICKEYBYTES {
		return nil, fmt.Errorf("Error opening signed message (%v)", "invalid public key")
	}

	message := make([]byte, len(signed))
	N := uint64(len(message))
	M := uint64(len(signed))

	rc := C.crypto_sign_open(
		makePtr(message),
		(*C.ulonglong)(&N),
		makePtr(signed),
		(C.ulonglong)(M),
		makePtr(key))

	if rc == 0 {
		return message[:N], nil
	}

	return nil, fmt.Errorf("Error opening signed message (%v)", rc)
}
