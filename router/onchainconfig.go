package router

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"

	ethclient "github.com/jowenshaw/gethclient"
	ethcommon "github.com/jowenshaw/gethclient/common"
	ethtypes "github.com/jowenshaw/gethclient/types"
	"github.com/jowenshaw/gethclient/types/ethereum"
)

var (
	routerConfigContract   ethcommon.Address
	routerConfigClients    []*ethclient.Client
	routerWebSocketClients []*ethclient.Client
	routerConfigCtx        = context.Background()

	channels   = make([]chan ethtypes.Log, 0, 3)
	subscribes = make([]ethereum.Subscription, 0, 3)

	// topic of event 'UpdateConfig()'
	updateConfigTopic = ethcommon.HexToHash("0x22590461e7ba17e1fe7580cb0ea47f283d3b2248f04873dfbe926d08fe4c5ab9")

	latestUpdateConfigBlock uint64
)

// InitRouterConfigClients init router config clients
func InitRouterConfigClients() {
	onchainCfg := params.GetRouterConfig().Onchain
	InitRouterConfigClientsWithArgs(onchainCfg.Contract, onchainCfg.APIAddress)
	routerWebSocketClients = InitWebSocketClients(onchainCfg.WSServers)
}

// InitWebSocketClients init
func InitWebSocketClients(wsServers []string) []*ethclient.Client {
	var err error
	wsClients := make([]*ethclient.Client, len(wsServers))
	for i, wsServer := range wsServers {
		wsClients[i], err = ethclient.Dial(wsServer)
		if err != nil {
			log.Fatal("init router config web socket clients failed", "wsServer", wsServer, "err", err)
		}
	}
	return wsClients
}

// InitRouterConfigClientsWithArgs init standalone
func InitRouterConfigClientsWithArgs(configContract string, gateways []string) {
	var err error
	routerConfigContract = ethcommon.HexToAddress(configContract)
	routerConfigClients = make([]*ethclient.Client, len(gateways))
	for i, gateway := range gateways {
		routerConfigClients[i], err = ethclient.Dial(gateway)
		if err != nil {
			log.Fatal("init router config clients failed", "gateway", gateway, "err", err)
		}
	}
}

// CallOnchainContract call onchain contract
func CallOnchainContract(data hexutil.Bytes, blockNumber string) (result []byte, err error) {
	msg := ethereum.CallMsg{
		To:   &routerConfigContract,
		Data: data,
	}
	for _, cli := range routerConfigClients {
		result, err = cli.CallContract(routerConfigCtx, msg, nil)
		if err != nil && IsIniting {
			for i := 0; i < RetryRPCCountInInit; i++ {
				if result, err = cli.CallContract(routerConfigCtx, msg, nil); err == nil {
					return result, nil
				}
				time.Sleep(RetryRPCIntervalInInit)
			}
		}
		if err == nil {
			return result, nil
		}
	}
	log.Debug("call onchain contract error", "contract", routerConfigContract.String(), "data", data, "err", err)
	return nil, err
}

// SubscribeUpdateConfig subscribe update ID and reload configs
func SubscribeUpdateConfig(callback func() bool) {
	if len(routerWebSocketClients) == 0 {
		return
	}
	SubscribeRouterConfig([]ethcommon.Hash{updateConfigTopic})
	for _, ch := range channels {
		go processUpdateConfig(ch, callback)
	}
}

func processUpdateConfig(ch <-chan ethtypes.Log, callback func() bool) {
	for {
		rlog := <-ch

		// sleep random in a second to mess steps
		rNum, _ := rand.Int(rand.Reader, big.NewInt(1000))
		time.Sleep(time.Duration(rNum.Uint64()) * time.Millisecond)

		blockNumber := rlog.BlockNumber
		oldBlock := atomic.LoadUint64(&latestUpdateConfigBlock)
		if blockNumber > oldBlock {
			atomic.StoreUint64(&latestUpdateConfigBlock, blockNumber)
			log.Info("start reload router config", "oldBlock", oldBlock, "blockNumber", blockNumber, "timestamp", time.Now().Unix())
			callback()
		}
	}
}

// SubscribeRouterConfig subscribe router config
func SubscribeRouterConfig(topics []ethcommon.Hash) {
	fq := ethereum.FilterQuery{
		Addresses: []ethcommon.Address{routerConfigContract},
		Topics:    [][]ethcommon.Hash{topics},
	}
	for i, cli := range routerWebSocketClients {
		ch := make(chan ethtypes.Log)
		sub, err := cli.SubscribeFilterLogs(routerConfigCtx, fq, ch)
		if err != nil {
			log.Error("subscribe 'UpdateConfig' event failed", "index", i, "err", err)
			continue
		}
		channels = append(channels, ch)
		subscribes = append(subscribes, sub)
	}
	log.Info("subscribe 'UpdateConfig' event finished", "subscribes", len(subscribes))
}

func parseChainConfig(data []byte) (config *tokens.ChainConfig, err error) {
	offset, overflow := common.GetUint64(data, 0, 32)
	if overflow {
		return nil, abicoder.ErrParseDataError
	}
	if uint64(len(data)) < offset+224 {
		return nil, abicoder.ErrParseDataError
	}
	data = data[32:]
	blockChain, err := abicoder.ParseStringInData(data, 0)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}
	routerContract, err := abicoder.ParseStringInData(data, 32)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}
	confirmations := common.GetBigInt(data, 64, 32).Uint64()
	initialHeight := common.GetBigInt(data, 96, 32).Uint64()
	extra, err := abicoder.ParseStringInData(data, 128)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}
	config = &tokens.ChainConfig{
		BlockChain:     blockChain,
		RouterContract: routerContract,
		Confirmations:  confirmations,
		InitialHeight:  initialHeight,
		Extra:          extra,
	}
	return config, nil
}

// GetChainConfig abi
func GetChainConfig(chainID *big.Int) (*tokens.ChainConfig, error) {
	if chainID == nil || chainID.Sign() == 0 {
		return nil, errors.New("chainID is zero")
	}
	funcHash := common.FromHex("0x19ed16dc")
	data := abicoder.PackDataWithFuncHash(funcHash, chainID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return nil, err
	}
	config, err := parseChainConfig(res)
	if err != nil {
		return nil, err
	}
	config.ChainID = chainID.String()
	return config, nil
}

func parseTokenConfig(data []byte) (config *tokens.TokenConfig, err error) {
	offset, overflow := common.GetUint64(data, 0, 32)
	if overflow {
		return nil, abicoder.ErrParseDataError
	}
	if uint64(len(data)) < offset+224 {
		return nil, abicoder.ErrParseDataError
	}
	data = data[offset:]
	decimals := uint8(common.GetBigInt(data, 0, 32).Uint64())
	contractAddress, err := abicoder.ParseStringInData(data, 32)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}
	contractVersion := common.GetBigInt(data, 64, 32).Uint64()
	routerContract, err := abicoder.ParseStringInData(data, 96)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}
	extra, err := abicoder.ParseStringInData(data, 128)
	if err != nil {
		return nil, abicoder.ErrParseDataError
	}

	config = &tokens.TokenConfig{
		Decimals:        decimals,
		ContractAddress: contractAddress,
		ContractVersion: contractVersion,
		RouterContract:  routerContract,
		Extra:           extra,
	}
	return config, err
}

// GetTokenConfig abi
func GetTokenConfig(chainID *big.Int, token string) (tokenCfg *tokens.TokenConfig, err error) {
	funcHash := common.FromHex("0x459511d1")
	data := abicoder.PackDataWithFuncHash(funcHash, token, chainID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return nil, err
	}
	config, err := parseTokenConfig(res)
	if err != nil {
		return nil, err
	}
	config.TokenID = token
	return config, nil
}

func parseSwapConfig(data []byte) (config *tokens.SwapConfig, err error) {
	if uint64(len(data)) < 3*32 {
		return nil, abicoder.ErrParseDataError
	}
	maximumSwap := common.GetBigInt(data, 0, 32)
	minimumSwap := common.GetBigInt(data, 32, 32)
	bigValueThreshold := common.GetBigInt(data, 64, 32)
	config = &tokens.SwapConfig{
		MaximumSwap:       maximumSwap,
		MinimumSwap:       minimumSwap,
		BigValueThreshold: bigValueThreshold,
	}
	return config, err
}

func callAndParseSwapConfigResult(data []byte) (*tokens.SwapConfig, error) {
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return nil, err
	}
	config, err := parseSwapConfig(res)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetSwapConfig abi
func GetSwapConfig(tokenID string, fromChainID, toChainID *big.Int) (*tokens.SwapConfig, error) {
	funcHash := common.FromHex("0x4da7163c")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID, fromChainID, toChainID)
	return callAndParseSwapConfigResult(data)
}

// GetActualSwapConfig abi
func GetActualSwapConfig(tokenID string, fromChainID, toChainID *big.Int) (*tokens.SwapConfig, error) {
	funcHash := common.FromHex("0xd5637235")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID, fromChainID, toChainID)
	return callAndParseSwapConfigResult(data)
}

func parseFeeConfig(data []byte) (config *tokens.FeeConfig, err error) {
	if uint64(len(data)) < 3*32 {
		return nil, abicoder.ErrParseDataError
	}
	maximumSwapFee := common.GetBigInt(data, 0, 32)
	minimumSwapFee := common.GetBigInt(data, 32, 32)
	swapFeeRatePerMillion := common.GetBigInt(data, 64, 32).Uint64()
	config = &tokens.FeeConfig{
		MaximumSwapFee:        maximumSwapFee,
		MinimumSwapFee:        minimumSwapFee,
		SwapFeeRatePerMillion: swapFeeRatePerMillion,
	}
	return config, err
}

func callAndParseFeeConfigResult(data []byte) (*tokens.FeeConfig, error) {
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return nil, err
	}
	config, err := parseFeeConfig(res)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetFeeConfig abi
func GetFeeConfig(tokenID string, fromChainID, toChainID *big.Int) (*tokens.FeeConfig, error) {
	funcHash := common.FromHex("0x1aed1c97")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID, fromChainID, toChainID)
	return callAndParseFeeConfigResult(data)
}

// GetActualFeeConfig abi
func GetActualFeeConfig(tokenID string, fromChainID, toChainID *big.Int) (*tokens.FeeConfig, error) {
	funcHash := common.FromHex("0xae409e9a")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID, fromChainID, toChainID)
	return callAndParseFeeConfigResult(data)
}

// GetCustomConfig abi
func GetCustomConfig(chainID *big.Int, key string) (string, error) {
	funcHash := common.FromHex("0x61387d61")
	data := abicoder.PackDataWithFuncHash(funcHash, chainID, key)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return "", err
	}
	if len(res) == 0 {
		return "", nil
	}
	return abicoder.ParseStringInData(res, 0)
}

// GetMPCPubkey abi
func GetMPCPubkey(mpcAddress string) (pubkey string, err error) {
	funcHash := common.FromHex("0x9f1cdedd")
	data := abicoder.PackDataWithFuncHash(funcHash, mpcAddress)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		if common.IsHexAddress(mpcAddress) && strings.ToLower(mpcAddress) == mpcAddress {
			mixAddress := common.HexToAddress(mpcAddress).Hex()
			data = abicoder.PackDataWithFuncHash(funcHash, mixAddress)
			res, err = CallOnchainContract(data, "latest")
			if err == nil {
				return abicoder.ParseStringInData(res, 0)
			}
		}
		return "", err
	}
	return abicoder.ParseStringInData(res, 0)
}

// IsChainIDExist abi
func IsChainIDExist(chainID *big.Int) (exist bool, err error) {
	funcHash := common.FromHex("0xfd15ea70")
	data := abicoder.PackDataWithFuncHash(funcHash, chainID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return false, err
	}
	return common.GetBigInt(res, 0, 32).Sign() != 0, nil
}

// IsTokenIDExist abi
func IsTokenIDExist(tokenID string) (exist bool, err error) {
	funcHash := common.FromHex("0xaf611ca0")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return false, err
	}
	return common.GetBigInt(res, 0, 32).Sign() != 0, nil
}

// GetAllChainIDs abi
func GetAllChainIDs() (chainIDs []*big.Int, err error) {
	funcHash := common.FromHex("0xe27112d5")
	res, err := CallOnchainContract(funcHash, "latest")
	if err != nil {
		return nil, err
	}
	return abicoder.ParseNumberSliceAsBigIntsInData(res, 0)
}

// GetAllTokenIDs abi
func GetAllTokenIDs() (tokenIDs []string, err error) {
	funcHash := common.FromHex("0x684a10b3")
	res, err := CallOnchainContract(funcHash, "latest")
	if err != nil {
		return nil, err
	}
	return abicoder.ParseStringSliceInData(res, 0)
}

// GetMultichainToken abi
func GetMultichainToken(tokenID string, chainID *big.Int) (tokenAddr string, err error) {
	funcHash := common.FromHex("0xb735ab5a")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID, chainID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return "", err
	}
	return abicoder.ParseStringInData(res, 0)
}

// MultichainToken struct
type MultichainToken struct {
	ChainID      *big.Int
	TokenAddress string
}

func parseMultichainTokens(data []byte) (mcTokens []MultichainToken, err error) {
	offset, overflow := common.GetUint64(data, 0, 32)
	if overflow {
		return nil, abicoder.ErrParseDataError
	}
	length, overflow := common.GetUint64(data, offset, 32)
	if overflow {
		return nil, abicoder.ErrParseDataError
	}
	if uint64(len(data)) < offset+32+length*96 {
		return nil, abicoder.ErrParseDataError
	}
	mcTokens = make([]MultichainToken, length)
	arrData := data[offset+32:]
	for i := uint64(0); i < length; i++ {
		offset, overflow = common.GetUint64(arrData, i*32, 32)
		if overflow {
			return nil, abicoder.ErrParseDataError
		}
		if uint64(len(arrData)) < offset+96 {
			return nil, abicoder.ErrParseDataError
		}
		innerData := arrData[offset:]
		mcTokens[i].ChainID = common.GetBigInt(innerData, 0, 32)
		mcTokens[i].TokenAddress, err = abicoder.ParseStringInData(innerData, 32)
		if err != nil {
			return nil, err
		}
	}
	return mcTokens, nil
}

// GetAllMultichainTokens abi
func GetAllMultichainTokens(tokenID string) ([]MultichainToken, error) {
	funcHash := common.FromHex("0x8fcb62a3")
	data := abicoder.PackDataWithFuncHash(funcHash, tokenID)
	res, err := CallOnchainContract(data, "latest")
	if err != nil {
		return nil, err
	}
	return parseMultichainTokens(res)
}
