package cardano

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/btcsuite/btcutil/bech32"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.ReSwapableBridge
	_ tokens.ReSwapable = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once

	defRPCClientTimeout = 60
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
	devnetNetWork  = "devnet"
)

// Bridge block bridge inherit from btc bridge
type Bridge struct {
	*tokens.CrossChainBridgeBase
	*base.ReSwapableBridgeBase
	RpcClient      cardanosdk.Node
	FakePrikey     crypto.PrvKey
	ProtocolParams *cardanosdk.ProtocolParams
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	priStr, _ := bech32.EncodeFromBase256("addr_sk", priv)
	fakePrikey, err := crypto.NewPrvKey(priStr)
	if err != nil {
		panic(err)
	}
	instance := &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		ReSwapableBridgeBase: base.NewReSwapableBridgeBase(),
		FakePrikey:           fakePrikey,
	}
	instance.RPCClientTimeout = defRPCClientTimeout
	BridgeInstance = instance

	return instance
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs[GetStubChainID(mainnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(devnetNetWork).String()] = true
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("CARDANO"))
	switch network {
	case mainnetNetWork:
	case testnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(1))
	case devnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(2))
	default:
		log.Fatalf("unknown network %v", network)
	}
	stubChainID.Mod(stubChainID, tokens.StubChainIDBase)
	stubChainID.Add(stubChainID, tokens.StubChainIDBase)
	return stubChainID
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	b.CrossChainBridgeBase.InitAfterConfig()
	chainId := b.GetChainConfig().GetChainID()
	apiKey := params.GetCustom(b.ChainConfig.ChainID, "APIKey")
	if apiKey != "" {
		if chainId.Cmp(GetStubChainID(mainnetNetWork)) == 0 {
			b.RpcClient = NewNode(cardanosdk.Mainnet, CardanoMainNet, apiKey)
		} else {
			b.RpcClient = NewNode(cardanosdk.Testnet, CardanoPreProd, apiKey)
		}
		protocolParams, err := b.RpcClient.ProtocolParams()
		if err != nil {
			panic(err)
		}
		b.ProtocolParams = protocolParams
	}

	timeoutStr := params.GetCustom(b.ChainConfig.ChainID, "TxTimeout")
	if timeoutStr != "" {
		timeout, err := common.GetUint64FromStr(timeoutStr)
		if err != nil {
			log.Error("cardano TxTimeout config failed", "err", err)
		}
		if timeout > 0 {
			b.ReSwapableBridgeBase.SetTimeoutConfig(timeout)
		}
	}

	reswapMaxAmountRateStr := params.GetCustom(b.ChainConfig.ChainID, "ReswapMaxAmountRate")
	if reswapMaxAmountRateStr != "" {
		reswapMaxAmountRate, err := common.GetUint64FromStr(reswapMaxAmountRateStr)
		if err != nil {
			log.Error("cardano ReswapMaxAmountRate config failed", "err", err)
		}
		if reswapMaxAmountRate > 0 {
			b.ReSwapableBridgeBase.SetReswapMaxValueRate(reswapMaxAmountRate)
		}
	}
}

func (b *Bridge) GetTip() (tip *cardanosdk.NodeTip, err error) {
	useAPI, _ := strconv.ParseBool(params.GetCustom(b.ChainConfig.ChainID, "UseAPI"))
	if useAPI {
		if b.RpcClient == nil {
			return nil, nil
		}
		return b.RpcClient.Tip()
	} else {
		urls := b.GatewayConfig.AllGatewayURLs
		for _, url := range urls {
			result, err := GetCardanoTip(url)
			if err == nil {
				b.flushProtocolParams(result)
				return &cardanosdk.NodeTip{
					Block: result.Cardano.Tip.BlockNumber,
					Epoch: result.Cardano.Tip.Epoch.Number,
					Slot:  result.Cardano.Tip.SlotNo,
				}, nil
			}
		}
		return nil, tokens.ErrTxNotFound
	}
}

// GetLatestBlockNumber gets latest block number
func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	useAPI := false
	if b.ChainConfig != nil {
		useAPI, _ = strconv.ParseBool(params.GetCustom(b.ChainConfig.ChainID, "UseAPI"))
	}
	if useAPI {
		if b.RpcClient == nil {
			return 0, nil
		}
		if tip, err := b.RpcClient.Tip(); err == nil {
			protocolParams, err := b.RpcClient.ProtocolParams()
			if err != nil {
				b.ProtocolParams = protocolParams
			}
			return tip.Block, nil
		} else {
			return 0, err
		}
	} else {
		urls := b.GatewayConfig.AllGatewayURLs
		for _, url := range urls {
			result, err := GetCardanoTip(url)
			if err == nil {
				b.flushProtocolParams(result)
				return result.Cardano.Tip.BlockNumber, nil
			}
		}
		return 0, tokens.ErrTxNotFound
	}
}

func (b *Bridge) flushProtocolParams(result *TipResponse) {
	b.ProtocolParams = &cardanosdk.ProtocolParams{
		CoinsPerUTXOWord: cardanosdk.Coin(result.Cardano.Tip.Epoch.ProtocolParams.CoinsPerUtxoByte),
		KeyDeposit:       cardanosdk.Coin(result.Cardano.Tip.Epoch.ProtocolParams.KeyDeposit),
		MaxBlockBodySize: uint(result.Cardano.Tip.Epoch.ProtocolParams.MaxBlockBodySize),
		MaxTxExUnits:     result.Cardano.Tip.Epoch.ProtocolParams.MaxBlockExMem,
		MaxTxSize:        uint(result.Cardano.Tip.Epoch.ProtocolParams.MaxTxSize),
		MinFeeA:          cardanosdk.Coin(result.Cardano.Tip.Epoch.ProtocolParams.MinFeeA),
		MinFeeB:          cardanosdk.Coin(result.Cardano.Tip.Epoch.ProtocolParams.MinFeeB),
		MinPoolCost:      cardanosdk.Coin(result.Cardano.Tip.Epoch.ProtocolParams.MinPoolCost),
	}
}

// GetLatestBlockNumberOf gets latest block number from single api
func (b *Bridge) GetLatestBlockNumberOf(url string) (num uint64, err error) {
	return b.GetLatestBlockNumber()
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(txHash string) (*Transaction, error) {
	useAPI, _ := strconv.ParseBool(params.GetCustom(b.ChainConfig.ChainID, "UseAPI"))
	if useAPI {
		txt, err := b.RpcClient.(*BlockfrostNode).GetTransactionByHash(txHash)
		if err != nil {
			return nil, err
		}
		txMetadata, err := b.RpcClient.(*BlockfrostNode).GetTransactionMetadataByHash(txHash)
		if err != nil {
			return nil, err
		}
		utxos, err := b.RpcClient.(*BlockfrostNode).GetTransactionUtxoByHash(txHash)
		if err != nil {
			return nil, err
		}

		metadata := []Metadata{}
		for _, md := range *txMetadata {
			if md.Label == MetadataKey {
				tmp, _ := json.Marshal(md.JsonMetadata)
				var mv MetadataValue
				err = json.Unmarshal(tmp, &mv)
				if err != nil {
					return nil, err
				}
				metadata = append(metadata, Metadata{
					Key:   md.Label,
					Value: mv,
				})
			}
		}

		input := []Input{}
		for _, v := range utxos.Inputs {
			details, _ := json.Marshal(v.Amount)
			input = append(input, Input{
				Address: v.Address,
				Value:   string(details),
			})
		}
		output := []Output{}
		for index, v := range utxos.Outputs {
			ts := []Token{}
			var value string
			for _, a := range v.Amount {
				if a.Unit == AdaAsset {
					value = a.Quantity
					continue
				}
				ts = append(ts, Token{
					Asset: Asset{
						PolicyId:  a.Unit[:56],
						AssetName: a.Unit[56:],
					},
					Quantity: a.Quantity,
				})
			}

			output = append(output, Output{
				Address: v.Address,
				Value:   value,
				Tokens:  ts,
				Index:   uint64(index),
			})
		}

		tx := &Transaction{
			Block: Block{
				SlotNo: uint64(txt.Slot),
				Number: uint64(txt.BlockHeight),
			},
			Hash:          txt.Hash,
			Metadata:      metadata,
			Inputs:        input,
			Outputs:       output,
			ValidContract: true,
		}
		return tx, nil
	} else {
		urls := b.GatewayConfig.AllGatewayURLs
		for _, url := range urls {
			result, err := GetTransactionByHash(url, txHash)
			if err == nil {
				return result, nil
			}
		}
		return nil, tokens.ErrTxNotFound
	}

}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetUtxosByAddress(address string) (*[]Output, error) {
	if !b.IsValidAddress(address) {
		return nil, errors.New("GetUtxosByAddress address is empty")
	}
	urls := b.GatewayConfig.AllGatewayURLs
	for _, url := range urls {
		result, err := GetUtxosByAddress(url, address)
		if err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrOutputLength
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	if res, err := b.GetTransactionByHash(txHash); err != nil {
		return nil, err
	} else {
		if !res.ValidContract {
			ClearTransactionChainingKeyCache(txHash)
			return nil, tokens.ErrTxIsNotValidated
		} else {
			status.BlockHeight = res.Block.Number
			status.Receipt = nil
			if lastHeight, err := b.GetLatestBlockNumber(); err != nil {
				return nil, err
			} else if lastHeight > res.Block.Number {
				status.Confirmations = lastHeight - res.Block.Number
				if status.Confirmations > b.GetChainConfig().Confirmations {
					ClearTransactionChainingKeyCache(txHash)
				}
			}
		}
	}
	return status, nil
}
func (b *Bridge) GetCurrentThreshold() (*uint64, error) {
	tip, err := b.GetTip()
	if err != nil {
		return nil, err
	}
	t := tip.Slot - b.GetChainConfig().Confirmations
	return &t, nil
}
