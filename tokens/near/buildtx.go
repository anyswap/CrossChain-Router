package near

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/mr-tron/base58"
)

const (
	defaultGasLimit uint64 = 70_000_000_000_000
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second
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

	switch args.SwapType {
	case tokens.ERC20SwapType:
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, tokens.ErrMissMPCPublicKey
	}

	nearPubKey, err := PublicKeyFromString(mpcPubkey)
	if err != nil {
		return nil, err
	}

	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return nil, tokens.ErrMissTokenConfig
	}

	tokenCfg := b.GetTokenConfig(multichainToken)
	if tokenCfg == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	args.SwapValue = amount // SwapValue

	if extra, err := b.initExtra(args); err != nil {
		return nil, err
	} else {
		if blockHashBytes, err := base58.Decode(*extra.BlockHash); err != nil {
			return nil, err
		} else {
			if to, actions, err := createFunctionCall(args.SwapID, receiver, amount.String(), args.FromChainID.String(), args.LogIndex, *extra.Gas, tokenCfg.ContractVersion); err != nil {
				return nil, err
			} else {
				var target string
				if to != "" {
					target = to
				} else {
					target = multichainToken
				}
				rawTx = CreateTransaction(args.From, nearPubKey, target, *extra.Sequence, blockHashBytes, actions)
			}
			return rawTx, nil
		}
	}
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
	if extra.BlockHash == nil {
		var blockHash string
		blockHash, err = b.GetLatestBlockHash()
		if err != nil {
			return nil, err
		}
		extra.BlockHash = &blockHash
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
	mpcPubkey := router.GetMPCPublicKey(address)
	return b.GetAccountNonce(address, mpcPubkey)
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

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

func CreateTransaction(
	signerID string,
	publicKey *PublicKey,
	receiverID string,
	nonce uint64,
	blockHash []byte,
	actions []Action,
) *RawTransaction {
	var tx RawTransaction
	tx.SignerID = signerID
	tx.PublicKey = *publicKey
	tx.ReceiverID = receiverID
	tx.Nonce = nonce
	copy(tx.BlockHash[:], blockHash)
	tx.Actions = actions
	return &tx
}

func createFunctionCall(txHash, to, amount, fromChainID string, logIndex int, gas, contractVersion uint64) (string, []Action, error) {
	log.Info("createFunctionCall", "txHash", txHash, "to", to, "amount", amount, "fromChainID", fromChainID)
	var methodName string
	var argsBytes []byte
	var deposit *big.Int
	switch contractVersion {
	case 666:
		argsBytes = buildTokenSwapInArgs(txHash, to, amount, fromChainID)
		methodName = "swap_in"
		deposit = big.NewInt(0)
	case 999:
		if value, err := common.GetBigIntFromStr(amount); err == nil {
			return to, []Action{{
				Enum: 3,
				Transfer: Transfer{
					Deposit: *value,
				},
			}}, nil
		} else {
			return "", nil, err
		}
	default:
		argsBytes = buildTokenTransferArgs(txHash, to, amount, fromChainID, logIndex)
		methodName = "ft_transfer"
		deposit = big.NewInt(1)
	}
	return "", []Action{{
		Enum: 2,
		FunctionCall: FunctionCall{
			MethodName: methodName,
			Args:       argsBytes,
			Gas:        gas,
			Deposit:    *deposit,
		},
	}}, nil
}

func buildTokenTransferArgs(txHash, to, amount, fromChainID string, logIndex int) []byte {
	memo := fmt.Sprintf("%s:%d:%s", txHash, logIndex, fromChainID)
	callArgs := &FtTransfer{
		ReceiverId: to,
		Amount:     amount,
		Memo:       memo,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}

func buildTokenSwapInArgs(txHash, to, amount, fromChainID string) []byte {
	callArgs := &FtSwapIn{
		TxHash:      txHash,
		ReceiverId:  to,
		Amount:      amount,
		FromChainId: fromChainID,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}
