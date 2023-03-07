package cardano

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

// BuildRawTransaction build raw tx
//
//nolint:funlen,gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if extra, err := b.initExtra(args); err != nil {
		return nil, err
	} else {
		if extra.RawTx != nil {
			var tx RawTransaction
			if err := json.Unmarshal(extra.RawTx, &tx); err != nil {
				return nil, err
			}
			if err := b.VerifyRawTransaction(&tx, args); err != nil {
				return nil, err
			}
			return &tx, nil
		}
		if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
			return nil, tokens.ErrToChainIDMismatch
		}
		if args.Input != nil {
			return nil, fmt.Errorf("forbid build raw swap tx with input data")
		}
		if args.From == "" {
			return nil, fmt.Errorf("forbid empty sender")
		}

		routerMPC := b.GetRouterContract("")
		if !common.IsEqualIgnoreCase(args.From, routerMPC) {
			log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
			return nil, tokens.ErrSenderMismatch
		}

		mpcPubkey := router.GetMPCPublicKey(args.From)
		if mpcPubkey == "" {
			return nil, tokens.ErrMissMPCPublicKey
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

		if receiver, amount, err := b.getReceiverAndAmount(args, multichainToken); err != nil {
			return nil, err
		} else {
			args.SwapValue = amount // SwapValue
			if utxos, err := b.QueryUtxo(routerMPC, multichainToken, amount); err != nil {
				return nil, err
			} else {
				swapId := fmt.Sprintf("%s-%d", args.SwapID, args.LogIndex)
				rawTransaction, err := b.BuildTx(swapId, receiver, multichainToken, amount, utxos)
				if err != nil {
					return nil, err
				}

				txout := uint64(rawTransaction.Slot + b.ReSwapableBridgeBase.GetTimeoutConfig())
				b.ReSwapableBridgeBase.SetTxTimeout(args, &txout)

				if rawBytes, err := json.Marshal(rawTransaction); err != nil {
					return nil, err
				} else {
					extra.RawTx = rawBytes
					return rawTransaction, nil
				}
			}
		}
	}
}

func (b *Bridge) BuildTx(swapId, receiver, assetId string, amount *big.Int, utxos map[UtxoKey]AssetsMap) (*RawTransaction, error) {
	log.Infof("Cardano BuildTx:\nreceiver:%+v\nassetId:%+v\namount:%+v\nutxos:%+v\n", receiver, assetId, amount, utxos)

	txIns := []UtxoKey{}
	txInsAssets := []AssetsMap{}

	// max tx size fee + output min ada
	adaRequired := new(big.Int).SetUint64(b.calcMaxFee() + FixAdaAmount.Uint64())
	if assetId == AdaAsset {
		if amount.Cmp(FixAdaAmount) < 0 {
			return nil, fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, "ada not enough, below "+FixAdaAmount.String())
		}
		adaRequired.Add(adaRequired, amount)
	} else {
		adaRequired.Add(adaRequired, FixAdaAmount)
	}

	allAssetsMap := map[string]uint64{}
	for utxoKey, assetsMap := range utxos {
		for asset, assetAmount := range assetsMap {
			value, err := common.GetBigIntFromStr(assetAmount)
			if err != nil {
				log.Info("[Cardano]GetBigIntFromStr", "txhash", utxoKey.TxHash, "index", utxoKey.TxIndex, "err", err)
				return nil, err
			}
			allAssetsMap[asset] += value.Uint64()
		}

		txIns = append(txIns, utxoKey)
		txInsAssets = append(txInsAssets, assetsMap)
		if assetId == AdaAsset {
			if allAssetsMap[assetId] > adaRequired.Uint64() {
				break
			}
		} else {
			if allAssetsMap[assetId] > amount.Uint64() && allAssetsMap[AdaAsset] > adaRequired.Uint64() {
				break
			}
		}
	}
	if allAssetsMap[AdaAsset] <= adaRequired.Uint64() {
		return nil, fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, "ada not enough, below "+FixAdaAmount.String())
	}

	// pparams, err := b.RpcClient.ProtocolParams()
	// if err != nil {
	// 	return nil, err
	// }
	nodeTip, err := b.GetTip()
	if err != nil {
		return nil, err
	}
	rawTransaction := &RawTransaction{
		SwapId:           swapId,
		TxOuts:           make(map[string]AssetsMap),
		TxIns:            txIns,
		TxInsAssets:      txInsAssets,
		TxIndex:          uint64(0),
		Slot:             nodeTip.Slot,
		CoinsPerUTXOWord: uint64(b.ProtocolParams.CoinsPerUTXOWord),
		KeyDeposit:       uint64(b.ProtocolParams.KeyDeposit),
		MinFeeA:          uint64(b.ProtocolParams.MinFeeA),
		MinFeeB:          uint64(b.ProtocolParams.MinFeeB),
	}
	rawTransaction.TxOuts[receiver] = map[string]string{}
	routerMpc := b.GetRouterContract("")
	rawTransaction.TxOuts[routerMpc] = map[string]string{}
	var adaAmount *big.Int
	if assetId == AdaAsset {
		adaAmount = amount
	} else {
		adaAmount = FixAdaAmount
		if allAssetsMap[assetId] >= amount.Uint64() {
			rawTransaction.TxOuts[receiver][assetId] = amount.String()
			if allAssetsMap[assetId] > amount.Uint64() {
				rawTransaction.TxOuts[routerMpc][assetId] = fmt.Sprint((allAssetsMap[assetId] - amount.Uint64()))
			}
		} else {
			policy := strings.Split(assetId, ".")
			assetName := string(common.Hex2Bytes(policy[1]))
			_, _, policyId := b.GetAssetPolicy(assetName)
			if policy[0] != policyId.String() {
				return nil, fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, assetId+" not enough")
			} else {
				rawTransaction.Mint = map[string]string{
					assetName: fmt.Sprint(amount.Uint64() - allAssetsMap[assetId]),
				}
				rawTransaction.TxOuts[receiver][assetId] = amount.String()
			}
		}
	}
	rawTransaction.TxOuts[receiver][AdaAsset] = adaAmount.String()
	if allAssetsMap[AdaAsset] > adaAmount.Uint64() {
		rawTransaction.TxOuts[routerMpc][AdaAsset] = fmt.Sprint((allAssetsMap[AdaAsset] - adaAmount.Uint64()))
	}
	for assetIdWithName, assetAmount := range allAssetsMap {
		if assetIdWithName != AdaAsset && assetIdWithName != assetId {
			rawTransaction.TxOuts[routerMpc][assetIdWithName] = fmt.Sprint(assetAmount)
		}
	}

	return rawTransaction, nil
}

func (b *Bridge) calcMaxFee() uint64 {
	return uint64(b.ProtocolParams.MinFeeA)*uint64(b.ProtocolParams.MaxTxSize) + uint64(b.ProtocolParams.MinFeeB)
}

func (b *Bridge) CreateRawTx(rawTransaction *RawTransaction, mpcAddr string) (*cardanosdk.Tx, error) {
	// label, _ := strconv.Atoi(SwapInMetadataKey)
	// txBuilder, err := b.genTxBuilder(mpcAddr, rawTransaction, &cardanosdk.AuxiliaryData{
	// 	Metadata: cardanosdk.Metadata{
	// 		uint(label): map[string]interface{}{
	// 			"SwapId": rawTransaction.SwapId,
	// 		},
	// 	},
	// })
	txBuilder, err := b.genTxBuilder(mpcAddr, rawTransaction, nil)
	if err != nil {
		return nil, err
	}

	tx, err := buildTxAndAdjustRestAda(txBuilder, rawTransaction, mpcAddr)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (b *Bridge) CreateSwapoutRawTx(rawTransaction *RawTransaction, mpcAddr, bind, toChainId string) (*cardanosdk.Tx, error) {
	swapKey, _ := strconv.Atoi(MetadataKey)

	var data *cardanosdk.AuxiliaryData
	if bind != "" {
		data = &cardanosdk.AuxiliaryData{
			Metadata: cardanosdk.Metadata{
				uint(swapKey): map[string]interface{}{
					"bind":      bind,
					"toChainId": toChainId,
				},
			},
		}
	}

	txBuilder, err := b.genTxBuilder(mpcAddr, rawTransaction, data)
	if err != nil {
		return nil, err
	}

	tx, err := buildTxAndAdjustRestAda(txBuilder, rawTransaction, mpcAddr)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func buildTxAndAdjustRestAda(txBuilder *cardanosdk.TxBuilder, rawTransaction *RawTransaction, mpcAddr string) (*cardanosdk.Tx, error) {
	tx, err := txBuilder.Build()
	if err != nil {
		return nil, err
	}
	txhash, err := tx.Hash()
	if err != nil {
		return nil, err
	}

	amount, err := strconv.ParseUint(rawTransaction.TxOuts[mpcAddr][AdaAsset], 10, 64)
	if err != nil {
		return nil, err
	}
	rawTransaction.TxOuts[mpcAddr][AdaAsset] = fmt.Sprint((amount - uint64(tx.Body.Fee)))
	log.Info("[Cardano]RawTx", "txhash", txhash.String(), "rest[ADA]", rawTransaction.TxOuts[mpcAddr][AdaAsset])
	return tx, nil
}

func (b *Bridge) genTxBuilder(mpcAddr string, rawTransaction *RawTransaction, data *cardanosdk.AuxiliaryData) (*cardanosdk.TxBuilder, error) {
	mpc, _ := cardanosdk.NewAddress(mpcAddr)
	txBuilder := cardanosdk.NewTxBuilder(&cardanosdk.ProtocolParams{
		CoinsPerUTXOWord: cardanosdk.Coin(rawTransaction.CoinsPerUTXOWord),
		KeyDeposit:       cardanosdk.Coin(rawTransaction.KeyDeposit),
		MinFeeA:          cardanosdk.Coin(rawTransaction.MinFeeA),
		MinFeeB:          cardanosdk.Coin(rawTransaction.MinFeeB),
	})
	inputs := []*cardanosdk.TxInput{}
	for index, utxoKey := range rawTransaction.TxIns {
		txHash, err := cardanosdk.NewHash32(strings.TrimSpace(utxoKey.TxHash))
		if err != nil {
			return nil, err
		}
		adaAmount, err := common.GetBigIntFromStr(rawTransaction.TxInsAssets[index][AdaAsset])
		if err != nil {
			return nil, err
		}
		assetValue := cardanosdk.NewValue(cardanosdk.Coin(adaAmount.Uint64()))
		for assetkey, assetAmount := range rawTransaction.TxInsAssets[index] {
			if assetkey == AdaAsset {
				continue
			}
			tmp := strings.Split(assetkey, ".")

			value, err := common.GetBigIntFromStr(assetAmount)
			if err != nil {
				return nil, err
			}
			p := cardanosdk.NewPolicyIDFromHash(common.Hex2Bytes(tmp[0]))
			an := cardanosdk.NewAssetName(string(common.Hex2Bytes(tmp[1])))
			av := cardanosdk.BigNum(value.Uint64())
			if assetValue.MultiAsset.Get(p) == nil {
				assetValue.MultiAsset.Set(p, cardanosdk.NewAssets().Set(an, av))
			} else {
				assetValue.MultiAsset.Get(p).Set(an, av)
			}
			log.Info("TxInsAssets", "origin assetkey", assetkey, "policy", p.String(), "assetName", an.String(), "amount", value.Uint64())
		}

		log.Info("AddTxInsAssets", "txHash", txHash.String(), "TxIndex", utxoKey.TxIndex, "assetValue", assetValue.Coin)
		inputs = append(inputs, cardanosdk.NewTxInput(txHash, uint(utxoKey.TxIndex), assetValue))
	}
	if len(rawTransaction.TxOuts) > 2 {
		return nil, tokens.ErrOutputLength
	}
	var txOut *cardanosdk.TxOutput
	for address, assets := range rawTransaction.TxOuts {
		if address == mpcAddr {
			continue
		}
		receiver, err := cardanosdk.NewAddress(strings.TrimSpace(address))
		if err != nil {
			return nil, err
		}
		adaAmount, err := common.GetBigIntFromStr(assets[AdaAsset])
		if err != nil {
			return nil, err
		}
		outValue := cardanosdk.NewValue(cardanosdk.Coin(adaAmount.Uint64()))
		txOut = cardanosdk.NewTxOutput(receiver, outValue)
		for asset, amount := range assets {
			if asset == AdaAsset {
				continue
			}
			tmp := strings.Split(asset, ".")

			value, err := common.GetBigIntFromStr(amount)
			if err != nil {
				return nil, err
			}
			p := cardanosdk.NewPolicyIDFromHash(common.Hex2Bytes(tmp[0]))
			an := cardanosdk.NewAssetName(string(common.Hex2Bytes(tmp[1])))
			av := cardanosdk.BigNum(value.Uint64())
			if outValue.MultiAsset.Get(p) == nil {
				outValue.MultiAsset.Set(p, cardanosdk.NewAssets().Set(an, av))
			} else {
				outValue.MultiAsset.Get(p).Set(an, av)
			}
			log.Info("TxOutAssets", "origin assetkey", asset, "policy", p.String(), "assetName", an.String(), "amount", value.Uint64())
		}
		log.Info("AddTxOutAssets", "receiver", receiver.String(), "ada", adaAmount.Uint64())
	}
	if rawTransaction.Mint != nil {
		for assetNameStr, amount := range rawTransaction.Mint {
			mintAmount, err := common.GetBigIntFromStr(amount)
			if err != nil {
				return nil, err
			}
			policyKey, policyScript, policyID := b.GetAssetPolicy(assetNameStr)
			assetName := cardanosdk.NewAssetName(assetNameStr)
			newAsset := cardanosdk.NewMint().
				Set(
					policyID,
					cardanosdk.NewMintAssets().
						Set(assetName, mintAmount),
				)
			txBuilder.AddNativeScript(policyScript)
			txBuilder.Mint(newAsset)
			txBuilder.Sign(policyKey.PrvKey())
		}
	}

	txBuilder.AddInputs(inputs...)
	if txOut != nil {
		txBuilder.AddOutputs(txOut)
	}
	txBuilder.SetTTL(rawTransaction.Slot + b.ReSwapableBridgeBase.GetTimeoutConfig())
	txBuilder.AddChangeIfNeeded(mpc)
	txBuilder.Sign(b.FakePrikey)
	if data != nil {
		txBuilder.AddAuxiliaryData(data)
	}
	return txBuilder, nil
}

func (b *Bridge) GetAssetPolicy(name string) (crypto.XPrvKey, cardanosdk.NativeScript, cardanosdk.PolicyID) {
	assetPolicy := params.GetCustom(b.ChainConfig.ChainID, "AssetPolicyKey")
	isAppendName, _ := strconv.ParseBool(params.GetCustom(b.ChainConfig.ChainID, "AppendName"))
	if isAppendName {
		assetPolicy = assetPolicy + name
	}
	policyKey := crypto.NewXPrvKeyFromEntropy([]byte(assetPolicy), "")
	policyScript, err := cardanosdk.NewScriptPubKey(policyKey.PubKey())
	if err != nil {
		panic(err)
	}
	policyID, err := cardanosdk.NewPolicyID(policyScript)
	if err != nil {
		panic(err)
	}
	return policyKey, policyScript, policyID
}

// func CalcMinFee(rawTransaction *RawTransaction) (string, error) {
// 	txBodyPath := RawPath + rawTransaction.OutFile + RawSuffix
// 	cmdString := fmt.Sprintf(CalcMinFeeCmd, txBodyPath, len(rawTransaction.TxIns), len(rawTransaction.TxOuts))
// 	if execRes, err := ExecCmd(cmdString, " "); err != nil {
// 		return "", err
// 	} else {
// 		return execRes, nil
// 	}
// }

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
	return extra, nil
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	return 0, nil
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	// if params.IsParallelSwapEnabled() {
	// 	nonce, err = b.AllocateNonce(args)
	// 	return &nonce, err
	// }

	// if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
	// 	nonce = b.GetSwapNonce(args.From)
	// 	return &nonce, nil
	// }

	nonce, err = b.GetPoolNonce(args.From, "pending")
	if err != nil {
		return nil, err
	}
	// nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
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

func (b *Bridge) QueryUtxo(address, assetName string, amount *big.Int) (map[UtxoKey]AssetsMap, error) {
	if utxos, err := b.GetTransactionChainingMap(assetName, amount); err != nil {
		return b.QueryUtxoOnChain(address)
	} else {
		return utxos, nil
	}
}

func (b *Bridge) GetTransactionChainingMap(assetName string, amount *big.Int) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	needAmount := big.NewInt(amount.Int64())
	assetBalance := TransactionChaining.AssetsMap[assetName]
	if assetBalance != "" {
		if balance, err := common.GetBigIntFromStr(assetBalance); err != nil {
			return nil, err
		} else {
			maxfee := big.NewInt(0).SetUint64(b.calcMaxFee())
			if assetName == AdaAsset {
				needAmount.Add(needAmount, maxfee)
			} else {
				ada, err := common.GetBigIntFromStr(TransactionChaining.AssetsMap[AdaAsset])
				if err != nil || ada.Cmp(maxfee) < 0 {
					return nil, err
				}
			}
			if balance.Cmp(needAmount) >= 0 {
				utxoKey := UtxoKey{TxHash: TransactionChaining.InputKey.TxHash, TxIndex: TransactionChaining.InputKey.TxIndex}
				utxos[utxoKey] = TransactionChaining.AssetsMap
				log.Info("Cardano UseTransactionChainingUtxo", "TxHash", TransactionChaining.InputKey.TxHash, "TxIndex", TransactionChaining.InputKey.TxIndex)
				return utxos, nil
			}
		}
	}
	return nil, tokens.ErrTokenBalancesNotEnough
}

func (b *Bridge) QueryUtxoOnChain(address string) (map[UtxoKey]AssetsMap, error) {
	useAPI, _ := strconv.ParseBool(params.GetCustom(b.ChainConfig.ChainID, "UseAPI"))
	if useAPI {
		return b.QueryUtxoByAPI(address)
	} else {
		return b.QueryUtxoByGQL(address)
	}
}

func (b *Bridge) QueryUtxoByGQL(address string) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	if outputs, err := b.GetUtxosByAddress(address); err != nil {
		return nil, err
	} else {
		for _, output := range *outputs {
			utxoKey := UtxoKey{TxHash: output.TxHash, TxIndex: output.Index}
			if !TransactionChainingKeyCache.SpentUtxoMap[utxoKey] {
				utxos[utxoKey] = make(AssetsMap)
				utxos[utxoKey][AdaAsset] = output.Value
				for _, token := range output.Tokens {
					utxos[utxoKey][token.Asset.PolicyId+"."+token.Asset.AssetName] = token.Quantity
				}
			}
		}
		return utxos, nil
	}
}

func (b *Bridge) QueryUtxoByAPI(address string) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	addr, _ := cardanosdk.NewAddress(address)
	outputs, err := b.RpcClient.UTxOs(addr)
	if err != nil {
		return nil, err
	}
	for _, output := range outputs {
		utxoKey := UtxoKey{TxHash: output.TxHash.String(), TxIndex: output.Index}
		if !TransactionChainingKeyCache.SpentUtxoMap[utxoKey] {
			utxos[utxoKey] = make(AssetsMap)
			utxos[utxoKey][AdaAsset] = strconv.FormatUint(uint64(output.Amount.Coin), 10)
			for _, policyID := range output.Amount.MultiAsset.Keys() {
				for _, assetName := range output.Amount.MultiAsset.Get(policyID).Keys() {
					utxos[utxoKey][policyID.String()+"."+common.Bytes2Hex(assetName.Bytes())] = strconv.FormatUint(uint64(output.Amount.MultiAsset.Get(policyID).Get(assetName)), 10)
				}
			}
		}
	}
	return utxos, nil
}

func (b *Bridge) QueryUtxoOnChainByAsset(address string, asset string) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	if outputs, err := b.GetUtxosByAddress(address); err != nil {
		return nil, err
	} else {
		for _, output := range *outputs {
			utxoKey := UtxoKey{TxHash: output.TxHash, TxIndex: output.Index}
			if !TransactionChainingKeyCache.SpentUtxoMap[utxoKey] {
				if asset == AdaAsset {
					utxos[utxoKey] = make(AssetsMap)
					utxos[utxoKey][AdaAsset] = output.Value
					for _, token := range output.Tokens {
						utxos[utxoKey][token.Asset.PolicyId+"."+token.Asset.AssetName] = token.Quantity
					}
				} else {
					found := false
					for _, token := range output.Tokens {
						if token.Asset.PolicyId+"."+token.Asset.AssetName == asset {
							found = true
							break
						}
					}
					if found {
						utxos[utxoKey] = make(AssetsMap)
						utxos[utxoKey][AdaAsset] = output.Value
						for _, token := range output.Tokens {
							utxos[utxoKey][token.Asset.PolicyId+"."+token.Asset.AssetName] = token.Quantity
						}
					}
				}
			}
		}
		return utxos, nil
	}
}

func (b *Bridge) VerifyRawTransaction(raw *RawTransaction, args *tokens.BuildTxArgs) error {
	mpcAddr := b.GetRouterContract("")
	if len(raw.TxOuts) > 2 {
		return tokens.ErrOutputLength
	}

	mpcAssetsMap := raw.TxOuts[mpcAddr]
	receiverAssetsMap := raw.TxOuts[args.SwapArgs.Bind]
	if mpcAssetsMap == nil || receiverAssetsMap == nil {
		return tokens.ErrTxWithWrongReceiver
	}
	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	switch len(receiverAssetsMap) {
	case 1:
		adaAmount := receiverAssetsMap[AdaAsset]
		if value, err := common.GetBigIntFromStr(adaAmount); err != nil {
			return err
		} else {
			if value.Cmp(args.OriginValue) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}
	case 2:
		adaAmount := receiverAssetsMap[AdaAsset]
		if value, err := common.GetBigIntFromStr(adaAmount); err != nil {
			return err
		} else {
			if value.Cmp(DefaultAdaAmount) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}

		assetAmount := receiverAssetsMap[multichainToken]
		if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
			return err
		} else {
			if value.Cmp(args.OriginValue) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}
	default:
		return tokens.ErrTxWithWrongAssetLength
	}
	return nil
}
