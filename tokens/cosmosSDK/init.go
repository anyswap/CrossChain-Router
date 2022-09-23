package cosmosSDK

import (
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codecTypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func NewCosmosRestClient(urls []string) *CosmosRestClient {
	return &CosmosRestClient{
		BaseUrls: urls,
		TxConfig: BuildNewTxConfig(),
	}
}

func (c *CosmosRestClient) SetBaseUrls(urls []string) {
	c.BaseUrls = urls
}

func BuildNewTxConfig() cosmosClient.TxConfig {
	interfaceRegistry := codecTypes.NewInterfaceRegistry()
	bankTypes.RegisterInterfaces(interfaceRegistry)
	PublicKeyRegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	return authTx.NewTxConfig(protoCodec, authTx.DefaultSignModes)
}

func PublicKeyRegisterInterfaces(registry codecTypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptoTypes.PubKey)(nil), &secp256k1.PubKey{})
}
