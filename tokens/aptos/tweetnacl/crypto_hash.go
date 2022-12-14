package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes returned by CryptHash.
const HASH_BYTES int = 64

// The size of the state byte array for crypto_hashblocks.
const HASHBLOCKS_STATEBYTES int = 64

// The block size for the message for crypto_hashblocks.
const HASHBLOCKS_BLOCKBYTES int = 128

// Wrapper function for crypto_hash.
//
// Calculates a SHA-512 hash of the message.
//
// Ref. http://nacl.cr.yp.to/hash.html
func CryptoHash(message []byte) ([]byte, error) {
	hash := make([]byte, HASH_BYTES)
	N := (C.ulonglong)(len(message))

	rc := C.crypto_hash(makePtr(hash),
		makePtr(message),
		N)

	if rc == 0 {
		return hash, nil
	}

	return nil, fmt.Errorf("Error calculating SHA-512 hash (error code %v)", rc)
}
