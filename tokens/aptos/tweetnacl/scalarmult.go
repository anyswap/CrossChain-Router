package tweetnacl

/*
#include "tweetnacl.h"
*/
import "C"

import (
	"fmt"
)

// The number of bytes in the group element component of scalar multiplication.
const SCALARMULT_BYTES int = 32

// The number of bytes in the integer component of scalar multiplication.
const SCALARMULT_SCALARBYTES int = 32

// Wrapper function for crypto_scalarmult_base.
//
// Computes the scalar product of a standard group element and an integer <code>n</code>.
//
// Ref. http://nacl.cr.yp.to/scalarmult.html
func ScalarMultBase(n []byte) ([]byte, error) {
	if len(n) != SCALARMULT_SCALARBYTES {
		return nil, fmt.Errorf("Error calculating base scalar product (%v)", "invalid integer")
	}

	q := make([]byte, SCALARMULT_BYTES)
	rc := C.crypto_scalarmult_base(makePtr(q), makePtr(n))

	if rc == 0 {
		return q, nil
	}

	return nil, fmt.Errorf("Error calculating base scalar scalar product (%v)", rc)
}

// Wrapper function for crypto_scalarmult.
//
// Computes the scalar product of a group element p and an integer n.
//
// Ref. http://nacl.cr.yp.to/scalarmult.html
func ScalarMult(n, p []byte) ([]byte, error) {
	if len(n) != SCALARMULT_SCALARBYTES {
		return nil, fmt.Errorf("Error calculating scalar product (%v)", "invalid integer")
	}

	if len(p) != SCALARMULT_BYTES {
		return nil, fmt.Errorf("Error calculating scalar product (%v)", "invalid group element")
	}

	q := make([]byte, SCALARMULT_BYTES)
	rc := C.crypto_scalarmult(makePtr(q), makePtr(n), makePtr(p))

	if rc == 0 {
		return q, nil
	}

	return nil, fmt.Errorf("Error calculating scalar multiplication (error code %v)", rc)
}
