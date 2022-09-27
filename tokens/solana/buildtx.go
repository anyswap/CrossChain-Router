package solana

import (
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	routerprog "github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/system"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/token"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

// BuildRawTransaction impl
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.SwapType != tokens.ERC20SwapType {
		return nil, tokens.ErrSwapTypeNotSupported
	}
	if args.ERC20SwapInfo == nil || args.ERC20SwapInfo.TokenID == "" {
		return nil, tokens.ErrEmptyTokenID
	}

	erc20SwapInfo := args.ERC20SwapInfo
	tokenID := erc20SwapInfo.TokenID
	chainID := b.ChainConfig.ChainID

	multichainToken := router.GetCachedMultichainToken(tokenID, chainID)
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", tokenID, "chainID", chainID)
		return nil, tokens.ErrMissTokenConfig
	}

	tokenCfg := b.GetTokenConfig(multichainToken)
	if tokenCfg == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	var tx *types.Transaction
	if tokens.IsNativeCoin(multichainToken) {
		tx, err = b.BuildSwapinNativeTransaction(args, tokenCfg)
	} else if tokenCfg.ContractVersion == 0 {
		tx, err = b.BuildSwapinTransferTransaction(args, tokenCfg)
	} else {
		tx, err = b.BuildSwapinMintTransaction(args, tokenCfg)
	}
	if err != nil {
		return nil, err
	}

	_, err = b.SimulateTransaction(tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver types.PublicKey, amount uint64, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver, err = types.PublicKeyFromBase58(args.Bind)
	if err != nil {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind, "err", err)
		return receiver, amount, tokens.ErrWrongBindAddress
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
	swapValue := tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	if !swapValue.IsUint64() {
		return receiver, amount, tokens.ErrTxWithWrongValue
	}
	return receiver, swapValue.Uint64(), err
}

// BuildSwapinMintTransaction build swapin mint tx
func (b *Bridge) BuildSwapinMintTransaction(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) (*types.Transaction, error) {
	receiver, amount, err := b.getReceiverAndAmount(args, tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	routerInfo, err := router.GetTokenRouterInfo(tokenCfg.TokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	mpc, err := types.PublicKeyFromBase58(routerInfo.RouterMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerInfo.RouterPDA)
	if err != nil {
		return nil, err
	}
	tokenMint, err := types.PublicKeyFromBase58(tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	routerContract := b.GetRouterContract(tokenCfg.ContractAddress)
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewSwapinMintInstruction(
		args.SwapID, amount, args.FromChainID.Uint64(),
		mpc, routerAccount, receiver, tokenMint, token.TokenProgramID,
	)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	err = b.setExtraArgs(args)
	if err != nil {
		return nil, err
	}
	recentBlockHash, err := types.PublicKeyFromBase58(*args.Extra.BlockHash)
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(mpc))
}

// BuildSwapinTransferTransaction build swapin transfer tx
func (b *Bridge) BuildSwapinTransferTransaction(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) (*types.Transaction, error) {
	receiver, amount, err := b.getReceiverAndAmount(args, tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	routerInfo, err := router.GetTokenRouterInfo(tokenCfg.TokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	mpc, err := types.PublicKeyFromBase58(routerInfo.RouterMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerInfo.RouterPDA)
	if err != nil {
		return nil, err
	}
	tokenMint, err := types.PublicKeyFromBase58(tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	ata, err := types.FindAssociatedTokenAddress(routerAccount, tokenMint)
	if err != nil {
		return nil, err
	}
	routerContract := b.GetRouterContract(tokenCfg.ContractAddress)
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewSwapinTransferInstruction(
		args.SwapID, amount, args.FromChainID.Uint64(),
		mpc, routerAccount, ata, receiver, tokenMint, token.TokenProgramID,
	)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	err = b.setExtraArgs(args)
	if err != nil {
		return nil, err
	}
	recentBlockHash, err := types.PublicKeyFromBase58(*args.Extra.BlockHash)
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(mpc))
}

// BuildSwapinNativeTransaction build swapin native tx
func (b *Bridge) BuildSwapinNativeTransaction(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) (*types.Transaction, error) {
	receiver, amount, err := b.getReceiverAndAmount(args, tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	routerInfo, err := router.GetTokenRouterInfo(tokenCfg.TokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	mpc, err := types.PublicKeyFromBase58(routerInfo.RouterMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerInfo.RouterPDA)
	if err != nil {
		return nil, err
	}
	routerContract := b.GetRouterContract(tokenCfg.ContractAddress)
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewSwapinNativeInstruction(
		args.SwapID, amount, args.FromChainID.Uint64(),
		mpc, routerAccount, receiver, system.SystemProgramID,
	)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	err = b.setExtraArgs(args)
	if err != nil {
		return nil, err
	}
	blockHash, err := types.PublicKeyFromBase58(*args.Extra.BlockHash)
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, blockHash, types.TransactionPayer(mpc))
}

func (b *Bridge) setExtraArgs(args *tokens.BuildTxArgs) error {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
	extra.EthExtra = nil // clear this which may be set in replace job
	if extra.Sequence == nil && extra.BlockHash == nil {
		recentBlockHash, blockHeight, err := b.getRecentBlockhash()
		if err != nil {
			return err
		}
		extra.Sequence = &blockHeight
		var blockhash string = recentBlockHash.String()
		extra.BlockHash = &blockhash
	}
	return nil
}

// BuildMintSPLTransaction build mint spl token tx
func (b *Bridge) BuildMintSPLTransaction(amount uint64, mintAddr, toAddr, minterAddr string) (*types.Transaction, error) {
	mint, err := types.PublicKeyFromBase58(mintAddr)
	if err != nil {
		return nil, err
	}
	to, err := types.PublicKeyFromBase58(toAddr)
	if err != nil {
		return nil, err
	}
	minter, err := types.PublicKeyFromBase58(minterAddr)
	if err != nil {
		return nil, err
	}

	instruction := token.NewMintToInstruction(amount, mint, to, minter)
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(minter))
}

// BuildSendSPLTransaction build send spl token tx
func (b *Bridge) BuildSendSPLTransaction(amount uint64, sourceAddr, destAddr, fromAddr string) (*types.Transaction, error) {
	source, err := types.PublicKeyFromBase58(sourceAddr)
	if err != nil {
		return nil, err
	}
	destination, err := types.PublicKeyFromBase58(destAddr)
	if err != nil {
		return nil, err
	}
	from, err := types.PublicKeyFromBase58(fromAddr)
	if err != nil {
		return nil, err
	}

	instruction := token.NewTransferInstruction(amount, source, destination, from)
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}

	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(from))
}

// BuildSendSolanaTransaction build send solana tx
func (b *Bridge) BuildSendSolanaTransaction(lamports uint64, fromAddress, toAddress string) (*types.Transaction, error) {
	from, err := types.PublicKeyFromBase58(fromAddress)
	if err != nil {
		return nil, err
	}
	to, err := types.PublicKeyFromBase58(toAddress)
	if err != nil {
		return nil, err
	}

	instruction := system.NewTransferSolanaInstruction(from, to, lamports)
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}

	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(from))
}

func (b *Bridge) getRecentBlockhash() (types.Hash, uint64, error) {
	resp, err := b.GetLatestBlockhash()
	if err != nil {
		return types.Hash{}, 0, err
	}
	blockHash := resp.Value.Blockhash
	log.Info("getRecentBlockhash", "lastValidBlockHeight", resp.Value.LastValidBlockHeight, "blockHash", resp.Value.Blockhash)
	return blockHash, uint64(resp.Value.LastValidBlockHeight), nil
}

func (b *Bridge) BuildChangeMpcTransaction(routerContract, routerMPC, routerPDA, newMpcAddress string) (*types.Transaction, error) {
	mpc, err := types.PublicKeyFromBase58(routerMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerPDA)
	if err != nil {
		return nil, err
	}
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}
	newMpc, err := types.PublicKeyFromBase58(newMpcAddress)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewChangeMPCInstruction(mpc, routerAccount, newMpc)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(mpc))
}

func (b *Bridge) BuildApplyMpcTransaction(routerContract, routerMPC, routerPDA, newMpcAddress string) (*types.Transaction, error) {
	mpc, err := types.PublicKeyFromBase58(routerMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerPDA)
	if err != nil {
		return nil, err
	}
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}
	newMpc, err := types.PublicKeyFromBase58(newMpcAddress)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewApplyMPCInstruction(mpc, routerAccount, newMpc)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(mpc))
}

func (b *Bridge) BuildEnableSwapTransaction(routerContract, routerMPC, routerPDA string, enable bool) (*types.Transaction, error) {
	mpc, err := types.PublicKeyFromBase58(routerMPC)
	if err != nil {
		return nil, err
	}
	routerAccount, err := types.PublicKeyFromBase58(routerPDA)
	if err != nil {
		return nil, err
	}
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		return nil, err
	}

	instruction := routerprog.NewEnableSwapTradeInstruction(enable, mpc, routerAccount)
	instruction.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{instruction}

	recentBlockHash, _, err := b.getRecentBlockhash()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(instructions, recentBlockHash, types.TransactionPayer(mpc))
}
