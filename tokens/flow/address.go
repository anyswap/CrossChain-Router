package flow

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return address != ""
}

func (b *Bridge) GetAccountNonce(account, publicKey string) (uint64, error) {
	return 0, nil
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return "", nil
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	return nil
}
