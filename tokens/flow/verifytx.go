package flow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/onflow/flow-go-sdk"
	fcrypto "github.com/onflow/flow-go-sdk/crypto"
)

var (
	errTxResultType = errors.New("tx type is not TransactionResult")
	Event_Type      = "A.%s.Router.LogSwapOut"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	tx, ok := rawTx.(*sdk.Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	msgHash := msgHashes[0]

	message := tx.EnvelopeMessage()
	message = append(sdk.TransactionDomainTag[:], message...)
	hasher, _ := fcrypto.NewHasher(fcrypto.SHA3_256)
	sigHash := hasher.ComputeHash(message)

	if !strings.EqualFold(sigHash.String(), msgHash) {
		logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
		logFunc("message hash mismatch", "want", msgHash, "have", sigHash)
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	log.Info("verifySwapoutTx", "txhash", txHash, "logIndex", logIndex, "allowUnstable", allowUnstable)
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	receipts, err := b.getSwapTxReceipt(swapInfo, true)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipts) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	events, errv := b.fliterReceipts(&receipts[logIndex])
	if errv != nil {
		return swapInfo, tokens.ErrSwapoutLogNotFound
	}

	parseErr := b.parseSwapoutTxEvent(swapInfo, events)
	if parseErr != nil {
		return swapInfo, parseErr
	}

	checkErr := b.checkSwapoutInfo(swapInfo)
	if checkErr != nil {
		return swapInfo, checkErr
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return swapInfo, nil
}

func (b *Bridge) checkTxStatus(txres *sdk.TransactionResult, allowUnstable bool) error {
	if txres.Status.String() != Success_Status {
		return tokens.ErrTxIsNotValidated
	}
	if !allowUnstable {
		lastHeight, errh1 := b.GetLatestBlockNumber()
		if errh1 != nil {
			return errh1
		}

		txHeight, errh2 := b.GetBlockNumberByHash(txres.BlockID)
		if errh2 != nil {
			return errh2
		}

		if lastHeight < txHeight+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}

		if lastHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) parseSwapoutTxEvent(swapInfo *tokens.SwapTxInfo, event []interface{}) error {
	swapInfo.ERC20SwapInfo.Token = event[0].(string)
	swapInfo.Bind = event[1].(string)

	amount, erra := common.GetBigIntFromStr(fmt.Sprint(event[2].(uint64)))
	if erra != nil {
		return erra
	}
	swapInfo.Value = amount

	toChainID, errt := common.GetBigIntFromStr(fmt.Sprint(event[4].(uint64)))
	if errt != nil {
		return errt
	}
	swapInfo.ToChainID = toChainID

	tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID

	depositAddress := b.GetRouterContract(swapInfo.ERC20SwapInfo.Token)
	swapInfo.To = depositAddress
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if strings.EqualFold(swapInfo.From, swapInfo.To) {
		return tokens.ErrTxWithWrongSender
	}

	erc20SwapInfo := swapInfo.ERC20SwapInfo

	fromTokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil || erc20SwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}

	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID, "txid", swapInfo.Hash)
		return tokens.ErrMissTokenConfig
	}

	toBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	toTokenCfg := toBridge.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		log.Warn("get token config failed", "chainID", swapInfo.ToChainID, "token", multichainToken)
		return tokens.ErrMissTokenConfig
	}

	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}

	bindAddr := swapInfo.Bind
	if !toBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

func (b *Bridge) getSwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]sdk.Event, error) {
	tx, txErr := b.GetTransaction(swapInfo.Hash)
	if txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, tokens.ErrTxNotFound
	}
	txres, ok := tx.(*sdk.TransactionResult)
	if !ok {
		return nil, errTxResultType
	}

	statusErr := b.checkTxStatus(txres, allowUnstable)
	if statusErr != nil {
		return nil, statusErr
	}
	return txres.Events, nil
}

func (b *Bridge) fliterReceipts(receipt *sdk.Event) ([]interface{}, error) {
	mpc, err := b.GetMPCAddress()
	if err != nil {
		return nil, err
	}
	log.Warn("sssssssssss", "type", fmt.Sprintf(Event_Type, mpc[2:]), "receipt.Type", receipt.Type)
	if receipt.Type == fmt.Sprintf(Event_Type, mpc[2:]) {
		valut := receipt.Value.ToGoValue()
		return valut.([]interface{}), nil
	}
	return nil, tokens.ErrSwapoutLogNotFound
}
