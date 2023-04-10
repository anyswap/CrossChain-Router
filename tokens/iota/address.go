package iota

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v2"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	urls := b.GetGatewayConfig().AllGatewayURLs
	for _, url := range urls {
		nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
		// fetch the node's info to know the min. required PoW score
		if info, err := nodeHTTPAPIClient.Info(ctx); err == nil {
			edAddr := ConvertStringToAddress(pubKeyHex)
			bech32Addr := edAddr.Bech32(iotago.NetworkPrefix(info.Bech32HRP))
			return bech32Addr, nil
		}
	}
	return "", errors.New("PublicKeyToAddress eror")
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	edAddr := ConvertPubKeyToAddr(mpcPubkey)
	if edAddr.String() == mpcAddress {
		return nil
	}
	return errors.New("VerifyMPCPubKey eror")
}

func Bech32ToEdAddr(bech32 string) (*iotago.Address, error) {
	if _, edAddr, err := iotago.ParseBech32(bech32); err == nil {
		return &edAddr, nil
	} else {
		return nil, err
	}
}
