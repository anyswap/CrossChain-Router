package flow

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
)

var (
	retryRPCCount           = 3
	retryRPCInterval        = 1 * time.Second
	defaultGasLimit  uint64 = 1_000_000
)

// BuildRawTransaction build raw tx
//nolint:gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.Input != nil {
		return nil, fmt.Errorf("forbid build raw swap tx with input data")
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
	}
	routerMPC, getMpcErr := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if getMpcErr != nil {
		return nil, getMpcErr
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}
	mpcPubKey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		return nil, err
	}
	switch args.SwapType {
	case tokens.ERC20SwapType:
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, b.GetChainConfig().ChainID)
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return nil, tokens.ErrMissTokenConfig
	}

	token := b.GetTokenConfig(multichainToken)
	if token == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	extra, err := b.initExtra(args)

	if err != nil {
		return nil, err
	}

	index, err := b.GetAccountIndex(args.From, mpcPubKey)
	if err != nil {
		return nil, err
	}

	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	args.SwapValue = amount // SwapValue

	swapInArgs, err := CreateSwapInArgs(args.SwapID, multichainToken, sdk.HexToAddress(receiver), args.FromChainID, args.OriginValue, token.Extra)
	if err != nil {
		return nil, err
	}

	rawTx, err = CreateTransaction(sdk.HexToAddress(args.From), index, *extra.Sequence, *extra.Gas, *extra.BlockID, swapInArgs)
	if err != nil {
		return nil, err
	}
	return rawTx, nil
}

func CreateSwapInArgs(
	txHash string,
	tokenIdentifier string,
	receiver sdk.Address,
	fromChainID *big.Int,
	amount *big.Int,
	path string,
) (*SwapIn, error) {
	tx := cadence.String(txHash)
	token := cadence.String(tokenIdentifier)
	recipient := cadence.Address(receiver)
	id, err := common.GetUint64FromStr(fromChainID.String())
	if err != nil {
		return nil, err
	}
	fromChainId := cadence.NewUInt64(id)

	realValue := ParseFlowNumber(amount)
	if len(realValue) < 10 {
		return nil, tokens.ErrTxWithWrongValue
	}
	value, err := cadence.NewUFix64(realValue)
	if err != nil {
		return nil, err
	}

	receivePaths := strings.Split(path, ",")
	if len(receivePaths) != 2 {
		return nil, errors.New("receive path len error")
	}

	path_0, err := cadence.NewString(receivePaths[0])
	if err != nil {
		return nil, err
	}
	path_1, err := cadence.NewString(receivePaths[1])
	if err != nil {
		return nil, err
	}
	realPaths := cadence.NewArray([]cadence.Value{path_0, path_1})

	swapIn := &SwapIn{
		Tx:           tx,
		Token:        token,
		Receiver:     recipient,
		FromChainId:  fromChainId,
		Amount:       value,
		ReceivePaths: realPaths,
	}
	return swapIn, nil
}

func CreateTransaction(
	signerAddress sdk.Address,
	signerIndex int,
	signerSequence uint64,
	gas uint64,
	blockID string,
	swapInArgs *SwapIn,
) (*sdk.Transaction, error) {
	swapIn, errf := ioutil.ReadFile("tokens/flow/transaction/SwapIn.cdc")
	if errf != nil {
		return nil, errf
	}
	script := string(swapIn)
	script = fmt.Sprintf(script, signerAddress)
	fmtBlockID := sdk.HexToID(blockID)
	tx := sdk.NewTransaction().
		SetScript([]byte(script)).
		SetReferenceBlockID(fmtBlockID).
		SetProposalKey(signerAddress, signerIndex, signerSequence).
		SetPayer(signerAddress).
		AddAuthorizer(signerAddress)

	err := tx.AddArgument(swapInArgs.Tx)
	if err != nil {
		return nil, err
	}

	err = tx.AddArgument(swapInArgs.Token)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(swapInArgs.Receiver)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(swapInArgs.FromChainId)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(swapInArgs.Amount)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(swapInArgs.ReceivePaths)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (b *Bridge) initExtra(args *tokens.BuildTxArgs) (extra *tokens.AllExtras, err error) {
	extra = args.Extra
	if extra == nil {
		extra = &tokens.AllExtras{}
		args.Extra = extra
	}
	if extra.Sequence == nil {
		extra.Sequence, err = b.GetSeq(args)
		if err != nil {
			return nil, err
		}
	}
	if extra.Gas == nil {
		gas := defaultGasLimit
		extra.Gas = &gas
	}
	if extra.BlockID == nil {
		blockID, err := b.GetLatestBlockID()
		if err != nil {
			return nil, err
		}
		tem := blockID.Hex()
		extra.BlockID = &tem
	}
	return extra, nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if !b.IsValidAddress(receiver) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("swapout to invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, amount, err
}

// GetTxBlockInfo impl NonceSetter interface
func (b *Bridge) GetTxBlockInfo(txHash string) (blockHeight, blockTime uint64) {
	txStatus, err := b.GetTransactionStatus(txHash)
	if err != nil {
		return 0, 0
	}
	return txStatus.BlockHeight, txStatus.BlockTime
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	urls := b.GatewayConfig.AllGatewayURLs
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			return result.Keys[0].SequenceNumber, nil
		}
	}
	return 0, tokens.ErrGetAccount
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
		nonce = b.GetSwapNonce(args.From)
		return &nonce, nil
	}

	for i := 0; i < retryRPCCount; i++ {
		nonce, err = b.GetPoolNonce(args.From, "pending")
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}
