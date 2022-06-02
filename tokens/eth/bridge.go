package eth

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}
)

// Bridge eth bridge
type Bridge struct {
	CustomConfig
	*base.NonceSetterBase
	Signer        types.Signer
	SignerChainID *big.Int
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CustomConfig:    NewCustomConfig(),
		NonceSetterBase: base.NewNonceSetterBase(),
	}
}

// CustomConfig custom config
type CustomConfig struct {
	// some chain's rpc is slow and need config a longer rpc timeout
	RPCClientTimeout int
	// eg. RSK chain do not check mixed case or not same as eth
	DontCheckAddressMixedCase bool
}

// NewCustomConfig new custom config
func NewCustomConfig() CustomConfig {
	return CustomConfig{
		RPCClientTimeout:          client.GetDefaultTimeout(false),
		DontCheckAddressMixedCase: false,
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	chainID, err := common.GetBigIntFromStr(b.ChainConfig.ChainID)
	if err != nil {
		logErrFunc("wrong chainID",
			"chainID", b.ChainConfig.ChainID,
			"blockChain", b.ChainConfig.BlockChain,
			"err", err)
		return
	}
	err = b.InitExtraCustoms()
	if err != nil {
		logErrFunc("init extra custons failed",
			"chainID", b.ChainConfig.ChainID,
			"blockChain", b.ChainConfig.BlockChain,
			"err", err)
		return
	}
	err = b.initSigner(chainID)
	if err != nil {
		logErrFunc("init signer failed",
			"chainID", b.ChainConfig.ChainID,
			"blockChain", b.ChainConfig.BlockChain,
			"err", err)
		return
	}
}

func (b *Bridge) initSigner(chainID *big.Int) (err error) {
	signerChainID, err := b.GetSignerChainID()
	if err != nil && router.IsIniting {
		for i := 0; i < router.RetryRPCCountInInit; i++ {
			if signerChainID, err = b.GetSignerChainID(); err == nil {
				break
			}
			time.Sleep(router.RetryRPCIntervalInInit)
		}
	}
	if err != nil {
		log.Error("get signer chain ID failed", "chainID", chainID, "err", err)
		return err
	}
	if chainID.Cmp(signerChainID) != 0 {
		log.Error("chain ID mismatch", "inconfig", chainID, "inbridge", signerChainID)
		return err
	}
	b.SignerChainID = signerChainID
	if params.IsDynamicFeeTxEnabled(signerChainID.String()) {
		b.Signer = types.MakeSigner("London", signerChainID)
	} else {
		b.Signer = types.MakeSigner("EIP155", signerChainID)
	}
	return nil
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	var routerFactory, routerWNative string
	if tokens.IsERC20Router() {
		routerFactory, err = b.GetFactoryAddress(routerContract)
		if err != nil {
			log.Warn("get router factory address failed", "routerContract", routerContract, "err", err)
		}
		routerWNative, err = b.GetWNativeAddress(routerContract)
		if err != nil {
			log.Warn("get router wNative address failed", "routerContract", routerContract, "err", err)
		}
	}
	routerMPC, err := b.GetMPCAddress(routerContract)
	if err != nil {
		log.Warn("get router mpc address failed", "routerContract", routerContract, "err", err)
		return err
	}
	if common.HexToAddress(routerMPC) == (common.Address{}) {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Warn("get mpc public key failed", "mpc", routerMPC, "err", err)
		return err
	}
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC:     routerMPC,
			RouterFactory: routerFactory,
			RouterWNative: routerWNative,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative)

	if mongodb.HasClient() {
		var nextSwapNonce uint64
		for i := 0; i < 3; i++ {
			nextSwapNonce, err = mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
			if err == nil {
				break
			}
		}
		b.InitSwapNonce(b, routerMPC, nextSwapNonce)
	}

	return nil
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if tokenCfg == nil || !tokens.IsERC20Router() {
		return
	}

	tokenID := tokenCfg.TokenID

	if tokenCfg.ContractVersion >= MintBurnWrapperTokenVersion {
		log.Info("ignore wrapper token config checking",
			"tokenID", tokenID, "tokenAddr", tokenAddr,
			"decimals", tokenCfg.Decimals, "ContractVersion", tokenCfg.ContractVersion)
		return
	}

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	decimals, errt := b.GetErc20Decimals(tokenAddr)
	if errt != nil {
		logErrFunc("get token decimals failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", errt)
		return
	}
	if decimals != tokenCfg.Decimals {
		logErrFunc("token decimals mismatch", "tokenID", tokenID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		return
	}
	routerContract := tokenCfg.RouterContract
	if routerContract == "" {
		routerContract = b.GetChainConfig().RouterContract
	}
	err := b.checkTokenMinter(routerContract, tokenCfg)
	if err != nil && tokenCfg.IsStandardTokenVersion() {
		logErrFunc("check token minter failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	underlying, err := b.GetUnderlyingAddress(tokenAddr)
	if err != nil && tokenCfg.IsStandardTokenVersion() {
		logErrFunc("get underlying address failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	tokenCfg.SetUnderlying(underlying) // init underlying address
}

func (b *Bridge) checkTokenMinter(routerContract string, tokenCfg *tokens.TokenConfig) (err error) {
	if !tokenCfg.IsStandardTokenVersion() {
		return nil
	}
	tokenAddr := tokenCfg.ContractAddress
	var minterAddr string
	var isMinter bool
	switch tokenCfg.ContractVersion {
	default:
		isMinter, err = b.IsMinter(tokenAddr, routerContract)
		if err != nil {
			return err
		}
		if !isMinter {
			return fmt.Errorf("%v is not minter", routerContract)
		}
		return nil
	case 3:
		minterAddr, err = b.GetVaultAddress(tokenAddr)
	case 2, 1:
		minterAddr, err = b.GetOwnerAddress(tokenAddr)
	}
	if err != nil {
		return err
	}
	if common.HexToAddress(minterAddr) != common.HexToAddress(routerContract) {
		return fmt.Errorf("minter mismatch, have '%v' config '%v'", minterAddr, routerContract)
	}
	return nil
}

// GetSignerChainID default way to get signer chain id
// use chain ID first, if missing then use network ID instead.
// normally this way works, but sometimes it failed (eg. ETC),
// then we should overwrite this function
// NOTE: call after chain config setted
func (b *Bridge) GetSignerChainID() (*big.Int, error) {
	switch strings.ToUpper(b.ChainConfig.BlockChain) {
	default:
		chainID, err := b.ChainID()
		if err != nil {
			return nil, err
		}
		if chainID.Sign() != 0 {
			return chainID, nil
		}
		return b.NetworkID()
	case "ETHCLASSIC":
		return b.getETCSignerChainID()
	}
}

func (b *Bridge) getETCSignerChainID() (*big.Int, error) {
	networkID, err := b.NetworkID()
	if err != nil {
		return nil, err
	}
	var chainID uint64
	switch networkID.Uint64() {
	case 1:
		chainID = 61 // mainnet
	case 6:
		chainID = 6 // kotti
	case 7:
		chainID = 63 // mordor
	default:
		log.Warnf("unsupported etc network id '%v'", networkID)
		return nil, errors.New("unsupported etc network id")
	}
	return new(big.Int).SetUint64(chainID), nil
}

// InitExtraCustoms init extra customs
func (b *Bridge) InitExtraCustoms() error {
	clientTimeout := params.GetRPCClientTimeout(b.ChainConfig.ChainID)
	if clientTimeout != 0 {
		b.RPCClientTimeout = clientTimeout
	} else {
		timeoutStr := params.GetCustom(b.ChainConfig.ChainID, "sendtxTimeout")
		if timeoutStr != "" {
			timeout, err := common.GetUint64FromStr(timeoutStr)
			if err != nil {
				log.Error("get sendtxTimeout failed", "err", err)
				return err
			}
			if timeout != 0 {
				b.RPCClientTimeout = int(timeout)
			}
		}
	}

	flag := params.GetCustom(b.ChainConfig.ChainID, "dontCheckAddressMixedCase")
	b.DontCheckAddressMixedCase = strings.EqualFold(flag, "true")

	return nil
}
