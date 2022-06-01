package stellar

const (
	// PubKeyBytesLenCompressed is compressed pubkey byte length
	PubKeyBytesLenCompressed = 33
	// PubKeyBytesLenUncompressed is uncompressed pubkey byte length
	PubKeyBytesLenUncompressed = 65
)

const (
	pubkeyCompressed byte = 0x2
)

// ImportPublicKey converts pubkey to ripple pubkey
func ImportPublicKey(pubkey []byte) *EdsaPublic {
	return &EdsaPublic{pub: pubkey}
}

// EdsaPublic struct ripple ecdsa pubkey key
type EdsaPublic struct {
	pub []byte
}
