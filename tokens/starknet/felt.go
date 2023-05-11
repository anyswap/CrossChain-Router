package starknet

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
)

const (
	FIELD_PRIME string = "3618502788666131213697322783095070105623107215331596699973092056135872020481"
)

var (
	MaxFelt     = StrToFelt(FIELD_PRIME)
	asciiRegexp = regexp.MustCompile(`^([[:graph:]]|[[:space:]]){1,31}$`)
)

// Felt represents Field Element or Felt from cairo.
type Felt struct {
	*big.Int
}

// Big converts a Felt to its big.Int representation.
func (f *Felt) Big() *big.Int {
	return new(big.Int).SetBytes(f.Int.Bytes())
}

// StrToFelt converts a string containing a decimal, hexadecimal or UTF8 charset into a Felt.
func StrToFelt(str string) *Felt {
	f := new(Felt)
	if ok := f.strToFelt(str); ok {
		return f
	}
	return nil
}

func (f *Felt) strToFelt(str string) bool {
	if b, ok := new(big.Int).SetString(str, 0); ok {
		f.Int = b
		return ok
	}
	if asciiRegexp.MatchString(str) {
		hexStr := hex.EncodeToString([]byte(str))
		if b, ok := new(big.Int).SetString(hexStr, 16); ok {
			f.Int = b
			return ok
		}
	}
	return false
}

// BigToFelt converts a big.Int to its Felt representation.
func BigToFelt(b *big.Int) *Felt {
	return &Felt{Int: b}
}

// BytesToFelt converts a []byte to its Felt representation.
func BytesToFelt(b []byte) *Felt {
	return &Felt{Int: new(big.Int).SetBytes(b)}
}

// String converts a Felt into its 'short string' representation.
func (f *Felt) ShortString() string {
	str := string(f.Bytes())
	if asciiRegexp.MatchString(str) {
		return str
	}
	return ""
}

// String converts a Felt into its hexadecimal string representation and implement fmt.Stringer.
func (f *Felt) String() string {
	return fmt.Sprintf("0x%x", f)
}

// MarshalJSON implements the json Marshaller interface for Felt to marshal types to []byte.
func (f Felt) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, f.String())), nil
}

// UnmarshalJSON implements the json Unmarshaller interface to unmarshal []byte into types.
func (f *Felt) UnmarshalJSON(p []byte) error {
	if string(p) == "null" || len(p) == 0 {
		return nil
	}

	var s string
	// parse double quotes
	if p[0] == 0x22 {
		s = string(p[1 : len(p)-1])
	} else {
		s = string(p)
	}

	if ok := f.strToFelt(s); !ok {
		return fmt.Errorf("unmarshalling big int: %s", string(p))
	}

	return nil
}
