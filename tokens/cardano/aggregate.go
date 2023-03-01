package cardano

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func (b *Bridge) BuildAggregateTx(swapId string, utxos map[UtxoKey]AssetsMap) (*RawTransaction, error) {
	log.Infof("BuildAggregateTx:\nswapId:%+v\nutxos:%+v\n", swapId, utxos)
	routerMpc := b.GetRouterContract("")
	nodeTip, err := b.GetTip()
	if err != nil {
		return nil, err
	}
	rawTransaction := &RawTransaction{
		// Fee:     "0",
		SwapId:           swapId,
		TxOuts:           make(map[string]AssetsMap),
		TxIns:            []UtxoKey{},
		TxInsAssets:      []AssetsMap{},
		Slot:             nodeTip.Slot,
		CoinsPerUTXOWord: uint64(b.ProtocolParams.CoinsPerUTXOWord),
		KeyDeposit:       uint64(b.ProtocolParams.KeyDeposit),
		MinFeeA:          uint64(b.ProtocolParams.MinFeeA),
		MinFeeB:          uint64(b.ProtocolParams.MinFeeB),
	}
	allAssetsMap := map[string]uint64{}
	for utxoKey, assetsMap := range utxos {
		rawTransaction.TxIns = append(rawTransaction.TxIns, utxoKey)
		rawTransaction.TxInsAssets = append(rawTransaction.TxInsAssets, assetsMap)
		for asset, assetAmount := range assetsMap {
			if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
				return nil, err
			} else {
				allAssetsMap[asset] += value.Uint64()
			}
		}
	}

	rawTransaction.TxOuts[routerMpc] = map[string]string{}
	for assetIdWithName, assetAmount := range allAssetsMap {
		if assetIdWithName == AdaAsset {
			rawTransaction.TxOuts[routerMpc][assetIdWithName] = fmt.Sprint(assetAmount)
		} else {
			policy := strings.Split(assetIdWithName, ".")
			assetName := string(common.Hex2Bytes(policy[1]))
			_, _, policyId := b.GetAssetPolicy(assetName)
			if policy[0] != policyId.String() {
				rawTransaction.TxOuts[routerMpc][assetIdWithName] = fmt.Sprint(assetAmount)
			} else {
				rawTransaction.Mint = map[string]string{
					assetName: fmt.Sprintf("-%d", assetAmount),
				}
			}
		}
	}

	if rawTransaction.Mint == nil || len(rawTransaction.Mint) == 0 {
		return nil, errors.New("no need to Aggregate")
	}
	return rawTransaction, nil
}

func (b *Bridge) SignAggregateTx(swapId string, rawTx interface{}) (string, error) {
	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
		return "", tokens.ErrWrongRawTx
	} else {
		txdata, err := json.Marshal(rawTransaction)
		if err != nil {
			return "", err
		}
		chainId := b.ChainConfig.GetChainID()
		mpcAddress := b.GetRouterContract("")
		args := &tokens.BuildTxArgs{
			SwapArgs: tokens.SwapArgs{
				Identifier:  tokens.AggregateIdentifier,
				SwapID:      swapId,
				FromChainID: chainId,
				ToChainID:   chainId,
			},
			From: mpcAddress,
			Extra: &tokens.AllExtras{
				RawTx: txdata,
			},
		}
		if signTx, _, err := b.MPCSignTransaction(rawTransaction, args); err != nil {
			return "", err
		} else {
			if txHash, err := b.SendTransaction(signTx); err != nil {
				return "", err
			} else {
				return txHash, nil
			}
		}
	}
}

func (b *Bridge) AggregateTx() (txHash string, err error) {
	mpcAddress := b.GetRouterContract("")
	swapId := fmt.Sprintf("doAggregateJob_%d", time.Now().Unix())
	utxo, err := b.QueryUtxoOnChain(mpcAddress)
	if err != nil {
		return "", err
	}
	rawTransaction, err := b.BuildAggregateTx(swapId, utxo)
	if err != nil {
		return "", err
	}
	txhash, err := b.SignAggregateTx(swapId, rawTransaction)
	if err != nil {
		return "", err
	}
	log.Info("CardanoAggregateTx", "txHash", txhash, "success", true)
	return txhash, nil
}

func (b *Bridge) VerifyAggregate(msgHash []string, args *tokens.BuildTxArgs) error {
	if args == nil || args.Extra == nil || len(args.Extra.RawTx) == 0 {
		return errors.New("invalid aggregate args")
	}
	var rawtx RawTransaction
	err := json.Unmarshal(args.Extra.RawTx, &rawtx)
	if err != nil {
		return err
	}
	routerMpc := b.GetRouterContract("")
	for acc := range rawtx.TxOuts {
		if !strings.EqualFold(acc, routerMpc) {
			return errors.New("invalid aggregate receiver")
		}
	}
	return nil
}
