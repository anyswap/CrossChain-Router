package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes in a crypto_box public key
const BOX_PUBLICKEYBYTES int = 32

// The number of bytes in a crypto_box secret key
const BOX_SECRETKEYBYTES int = 32

// The number of bytes for a crypto_box nonce.
const BOX_NONCEBYTES int = 24

// The number of zero padding bytes for a crypto_box message
const BOX_ZEROBYTES int = 32

// The number of zero padding bytes for a crypto_box ciphertext
const BOX_BOXZEROBYTES int = 16

// The number of bytes in an initialised crypto_box_beforenm key buffer.
const BOX_BEFORENMBYTES int = 32

// Constant zero-filled byte array used for padding messages
var BOX_PADDING = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

type KeyPair struct {
	PublicKey []byte
	SecretKey []byte
}

// Wrapper function for crypto_box_keypair.
//
// Randomly generates a secret key and a corresponding public key. It guarantees that the secret key
// has BOX_PUBLICKEYBYTES bytes and that the public key has BOX_SECRETKEYBYTES bytes,
// returns a KeyPair initialised with a crypto_box public/private key pair.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBoxKeyPair() (*KeyPair, error) {
	pk := make([]byte, BOX_PUBLICKEYBYTES)
	sk := make([]byte, BOX_SECRETKEYBYTES)
	rc := C.crypto_box_keypair(makePtr(pk), makePtr(sk))

	if rc == 0 {
		return &KeyPair{PublicKey: pk, SecretKey: sk}, nil
	}

	return nil, fmt.Errorf("Error generating key pair (error code %v)", rc)
}

// Wrapper function for crypto_box.
//
// Encrypts and authenticates the message using the secretKey, publicKey and nonce. The zero padding
// required by the crypto_box C API is added internally and should not be included in the supplied
// ciphertext. Likewise the zero padding that prefixes the ciphertext returned by the crypto_box C
// API is stripped from the returned ciphertext.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBox(message, nonce, publicKey, secretKey []byte) ([]byte, error) {
	if len(nonce) != BOX_NONCEBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid nonce")
	}

	if len(publicKey) != BOX_PUBLICKEYBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid public key")
	}

	if len(secretKey) != BOX_SECRETKEYBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid secret key")
	}

	buffer := make([]byte, len(message)+BOX_ZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:BOX_ZEROBYTES], BOX_PADDING)
	copy(buffer[BOX_ZEROBYTES:], message)

	rc := C.crypto_box(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(publicKey),
		makePtr(secretKey))

	if rc == 0 {
		return buffer[BOX_BOXZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error encrypting message (%v)", rc)
}

// Wrapper function for crypto_box_open.
//
// Verifies and decrypts the ciphertext using the secretKey, publicKey and nonce. The zero padding
// required by the crypto_box C API is added internally and should not be included in the supplied
// message. Likewise the zero padding that prefixes the plaintext returned by the crypto_box C API
// is stripped from the returned plaintext.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBoxOpen(ciphertext, nonce, publicKey, secretKey []byte) ([]byte, error) {
	if len(nonce) != BOX_NONCEBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid nonce")
	}

	if len(publicKey) != BOX_PUBLICKEYBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid public key")
	}

	if len(secretKey) != BOX_SECRETKEYBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid secret key")
	}

	buffer := make([]byte, len(ciphertext)+BOX_BOXZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:BOX_BOXZEROBYTES], BOX_PADDING)
	copy(buffer[BOX_BOXZEROBYTES:], ciphertext)

	rc := C.crypto_box_open(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(publicKey),
		makePtr(secretKey))

	if rc == 0 {
		return buffer[BOX_ZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error decrypting message (error code %v)", rc)
}

// Wrapper function for crypto_box_beforenm.
//
// Calculates a 32 byte shared key for the  hashed key-exchange described for curve 25519.
//
// Applications that send several messages to the same receiver can gain speed by splitting
// CryptoBox into two steps, CryptoBoxBeforeNM and CryptoBoxAfterNM. Similarly, applications
// that receive several messages from the same sender can gain speed by splitting CryptoBoxOpen
// into two steps, CryptoBoxBeforeNM and CryptoBoxAfterNMOpen.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBoxBeforeNM(publicKey, secretKey []byte) ([]byte, error) {
	if len(publicKey) != BOX_PUBLICKEYBYTES {
		return nil, fmt.Errorf("Error generating shared key(%v)", "invalid public key")
	}

	if len(secretKey) != BOX_SECRETKEYBYTES {
		return nil, fmt.Errorf("Error generating shared key (%v)", "invalid secret key")
	}

	key := make([]byte, BOX_BEFORENMBYTES)

	rc := C.crypto_box_beforenm(makePtr(key),
		makePtr(publicKey),
		makePtr(secretKey))

	if rc == 0 {
		return key, nil
	}

	return nil, fmt.Errorf("Error generating shared key (error code %v)", rc)
}

// Wrapper function for crypto_box_afternm.
//
// Encrypts a message using the shared key generated by CryptoBoxBeforeNM. The zero padding
// required by the crypto_box C API is added internally and should not be included in the
// supplied message. Likewise the zero padding that prefixes the ciphertext returned by the
// crypto_box C API is stripped from the returned ciphertext.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBoxAfterNM(message, nonce, key []byte) ([]byte, error) {
	if len(nonce) != BOX_NONCEBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid nonce")
	}

	if len(key) != BOX_BEFORENMBYTES {
		return nil, fmt.Errorf("Error encrypting message (%v)", "invalid shared key")
	}

	buffer := make([]byte, len(message)+BOX_ZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:BOX_ZEROBYTES], BOX_PADDING)
	copy(buffer[BOX_ZEROBYTES:], message)

	rc := C.crypto_box_afternm(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(key))

	if rc == 0 {
		return buffer[BOX_BOXZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error encrypting message (error code %v)", rc)
}

// Wrapper function for crypto_box_open_afternm.
//
// Verifies and decrypts a message using the shared key generated by CryptoBoxBeforeNM. The zero
// padding required by the crypto_box C API is added internally and should not be included in
// the supplied message. Likewise the zero padding that prefixes the plaintext returned by the
// crypto_box C API is stripped from the returned plaintext.
//
// Ref. http://nacl.cr.yp.to/box.html
func CryptoBoxOpenAfterNM(ciphertext, nonce, key []byte) ([]byte, error) {
	if len(nonce) != BOX_NONCEBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid nonce")
	}

	if len(key) != BOX_BEFORENMBYTES {
		return nil, fmt.Errorf("Error decrypting message (%v)", "invalid shared key")
	}

	buffer := make([]byte, len(ciphertext)+BOX_BOXZEROBYTES)
	N := (C.ulonglong)(len(buffer))

	copy(buffer[0:BOX_BOXZEROBYTES], BOX_PADDING)
	copy(buffer[BOX_BOXZEROBYTES:], ciphertext)

	rc := C.crypto_box_open_afternm(makePtr(buffer),
		makePtr(buffer),
		N,
		makePtr(nonce),
		makePtr(key))

	if rc == 0 {
		return buffer[BOX_ZEROBYTES:], nil
	}

	return nil, fmt.Errorf("Error decrypting message (error code %v)", rc)
}
