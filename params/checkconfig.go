package params

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var blankOrCommaSepRegexp = regexp.MustCompile(`[\s,]+`) // blank or comma separated

func splitStringByBlankOrComma(str string) []string {
	return blankOrCommaSepRegexp.Split(strings.TrimSpace(str), -1)
}

// CallContractWithGateway call eth_call
func CallContractWithGateway(gateway, contract string, data hexutil.Bytes, blockNumber string) (result string, err error) {
	reqArgs := map[string]interface{}{
		"to":   contract,
		"data": data,
	}
	err = client.RPCPostWithTimeout(60, &result, gateway, "eth_call", reqArgs, blockNumber)
	if err == nil {
		return result, nil
	}
	return "", err
}

// CheckConfig check router config
func (config *RouterConfig) CheckConfig(isServer bool) (err error) {
	if !strings.HasPrefix(config.Identifier, RouterSwapPrefixID) || config.Identifier == RouterSwapPrefixID {
		return fmt.Errorf("wrong identifier '%v', missing prefix '%v'", config.Identifier, RouterSwapPrefixID)
	}
	if config.SwapType == "" {
		return errors.New("empty router swap type")
	}
	if config.SwapType == "anycallswap" && config.SwapSubType == "" {
		return errors.New("anycall must config 'SwapSubType'")
	}
	log.Info("check identifier pass", "identifier", config.Identifier, "swaptype", config.SwapType, "swapsubtype", config.SwapSubType, "isServer", isServer)

	err = config.CheckBlacklistConfig()
	if err != nil {
		return err
	}

	// check and init extra firstly
	if config.Extra != nil {
		err = config.Extra.CheckConfig()
		if err != nil {
			return err
		}
	}

	if isServer {
		err = config.Server.CheckConfig()
	} else {
		err = config.Oracle.CheckConfig()
	}
	if err != nil {
		return err
	}

	if config.MPC == nil {
		return errors.New("server must config 'MPC'")
	}
	err = config.MPC.CheckConfig(isServer)
	if err != nil {
		return err
	}

	if config.FastMPC != nil {
		err = config.FastMPC.CheckConfig(isServer)
		if err != nil {
			return err
		}
	}

	if config.Onchain == nil {
		return errors.New("server must config 'Onchain'")
	}
	err = config.Onchain.CheckConfig()
	if err != nil {
		return err
	}

	return nil
}

// CheckBlacklistConfig check black list config
func (config *RouterConfig) CheckBlacklistConfig() (err error) {
	tempCidMap := make(map[string]struct{})
	for _, chainID := range config.ChainIDBlackList {
		biChainID, ok := new(big.Int).SetString(chainID, 0)
		if !ok {
			return fmt.Errorf("wrong chain id '%v' in black list", chainID)
		}
		key := biChainID.String()
		if !IsReload {
			if _, exist := tempCidMap[key]; exist {
				return fmt.Errorf("duplicate chain id '%v' in black list", key)
			}
		}
		tempCidMap[key] = struct{}{}
	}
	chainIDBlacklistMap = tempCidMap
	if len(chainIDBlacklistMap) > 0 {
		log.Infof("chainID blacklist is %v (isReload: %v)", config.ChainIDBlackList, IsReload)
	}

	tempTCidMap := make(map[string]map[string]struct{})
	for cid, tokenIDs := range config.TokenIDBlackListOnChain {
		m := make(map[string]struct{})
		for _, tokenID := range tokenIDs {
			if tokenID == "" {
				return fmt.Errorf("empty token id in black list on chain %v", cid)
			}
			key := strings.ToLower(tokenID)
			if !IsReload {
				if _, exist := m[key]; exist {
					return fmt.Errorf("duplicate token id '%v' in black list on chain %v", key, cid)
				}
			}
			m[key] = struct{}{}
		}
		tempTCidMap[cid] = m
	}
	tokenIDBlacklistOnChainMap = tempTCidMap
	if len(tokenIDBlacklistOnChainMap) > 0 {
		log.Info("init tokenID blacklist on chains success", "isReload", IsReload)
	}

	tempTidMap := make(map[string]struct{})
	for _, tokenID := range config.TokenIDBlackList {
		if tokenID == "" {
			return errors.New("empty token id in black list")
		}
		key := strings.ToLower(tokenID)
		if !IsReload {
			if _, exist := tempTidMap[key]; exist {
				return fmt.Errorf("duplicate token id '%v' in black list", key)
			}
		}
		tempTidMap[key] = struct{}{}
	}
	tokenIDBlacklistMap = tempTidMap
	if len(tokenIDBlacklistMap) > 0 {
		log.Info("init tokenID blacklist success", "isReload", IsReload)
	}

	tempAccMap := make(map[string]struct{})
	for _, account := range config.AccountBlackList {
		if account == "" {
			return errors.New("empty account in black list")
		}
		key := strings.ToLower(account)
		if !IsReload {
			if _, exist := tempAccMap[key]; exist {
				return fmt.Errorf("duplicate account '%v' in black list", key)
			}
		}
		tempAccMap[key] = struct{}{}
	}
	accountBlacklistMap = tempAccMap
	if len(accountBlacklistMap) > 0 {
		log.Info("init account blacklist success", "isReload", IsReload)
	}
	return nil
}

// CheckConfig of router oracle
func (c *RouterOracleConfig) CheckConfig() (err error) {
	if c == nil {
		return errors.New("router oracle must config 'Oracle'")
	}
	if c.ServerAPIAddress == "" {
		return errors.New("oracle must config 'ServerAPIAddress'")
	}
	if IsReload {
		return nil
	}
	if c.NoCheckServerConnection {
		log.Info("oracle ignore check server connection")
		return nil
	}
	var version string
	for i := 0; i < 3; i++ {
		err = client.RPCPostWithTimeout(60, &version, c.ServerAPIAddress, "swap.GetVersionInfo")
		if err == nil {
			log.Info("oracle get server version info succeed", "version", version)
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Warn("oracle connect ServerAPIAddress failed", "ServerAPIAddress", c.ServerAPIAddress, "err", err)
	}
	return err
}

// CheckConfig of router server
//nolint:funlen,gocyclo // ok
func (s *RouterServerConfig) CheckConfig() error {
	if s == nil {
		return errors.New("router server must config 'Server'")
	}
	if s.APIServer == nil {
		return errors.New("server must config 'APIServer'")
	}
	if s.MongoDB == nil {
		return errors.New("server must config 'MongoDB'")
	}
	if err := s.MongoDB.CheckConfig(); err != nil {
		return err
	}
	for cid, defGasLimit := range s.DefaultGasLimit {
		masGasLimit := s.MaxGasLimit[cid]
		if masGasLimit > 0 && defGasLimit > masGasLimit {
			return fmt.Errorf("chain %v default gas limit %v is greater than its max gas limit %v", cid, defGasLimit, masGasLimit)
		}
	}

	initAutoSwapNonceEnabledChains()

	tempFixGasPriceMap := make(map[string]*big.Int)
	for chainID, fixedGasPriceStr := range s.FixedGasPrice {
		biChainID, ok := new(big.Int).SetString(chainID, 0)
		if !ok {
			return fmt.Errorf("wrong chain id '%v' in 'FixedGasPrice'", chainID)
		}
		fixedGasPrice, err := common.GetBigIntFromStr(fixedGasPriceStr)
		if err != nil {
			return fmt.Errorf("wrong gas price '%v' in 'FixedGasPrice'", fixedGasPriceStr)
		}
		key := biChainID.String()
		if !IsReload {
			if _, exist := tempFixGasPriceMap[key]; exist {
				return fmt.Errorf("duplicate chain id '%v' in 'FixedGasPrice'", key)
			}
		}
		tempFixGasPriceMap[key] = fixedGasPrice
	}
	fixedGasPriceMap = tempFixGasPriceMap
	log.Info("init FixedGasPrice success", "isReload", IsReload)

	tempMaxGasPriceMap := make(map[string]*big.Int)
	for chainID, maxGasPriceStr := range s.MaxGasPrice {
		biChainID, ok := new(big.Int).SetString(chainID, 0)
		if !ok {
			return fmt.Errorf("wrong chain id '%v' in 'MaxGasPrice'", chainID)
		}
		maxGasPrice, err := common.GetBigIntFromStr(maxGasPriceStr)
		if err != nil {
			return fmt.Errorf("wrong gas price '%v' in 'MaxGasPrice'", maxGasPriceStr)
		}
		key := biChainID.String()
		if !IsReload {
			if _, exist := tempMaxGasPriceMap[key]; exist {
				return fmt.Errorf("duplicate chain id '%v' in 'MaxGasPrice'", key)
			}
		}
		tempMaxGasPriceMap[key] = maxGasPrice
	}
	maxGasPriceMap = tempMaxGasPriceMap
	log.Info("init MaxGasPrice success", "isReload", IsReload)

	err := s.CheckDynamicFeeTxConfig()
	if err != nil {
		return err
	}
	err = s.CheckExtra()
	if err != nil {
		return err
	}
	log.Info("check server config success",
		"defaultGasLimit", s.DefaultGasLimit,
		"maxGasLimit", s.MaxGasLimit,
		"maxTokenGasLimit", s.MaxTokenGasLimit,
		"fixedGasPrice", fixedGasPriceMap,
		"maxGasPrice", maxGasPriceMap,
		"noncePassedConfirmInterval", s.NoncePassedConfirmInterval,
	)
	return nil
}

// CheckDynamicFeeTxConfig check dynamic fee tx config
func (s *RouterServerConfig) CheckDynamicFeeTxConfig() error {
	for _, c := range s.DynamicFeeTx {
		if c.MaxGasTipCap != "" {
			bi, err := common.GetBigIntFromStr(c.MaxGasTipCap)
			if err != nil {
				return errors.New("wrong 'MaxGasTipCap'")
			}
			c.maxGasTipCap = bi
		}
		if c.MaxGasFeeCap != "" {
			bi, err := common.GetBigIntFromStr(c.MaxGasFeeCap)
			if err != nil {
				return errors.New("wrong 'MaxGasFeeCap'")
			}
			c.maxGasFeeCap = bi
		}
		if c.PlusGasTipCapPercent > 100 {
			return errors.New("too large 'PlusGasTipCapPercent'")
		}
		if c.PlusGasFeeCapPercent > 100 {
			return errors.New("too large 'PlusGasFeeCapPercent'")
		}
		if c.BlockCountFeeHistory > 1024 {
			return errors.New("too large 'BlockCountFeeHistory'")
		}

		if c.maxGasTipCap == nil {
			return errors.New("server must config 'MaxGasTipCap'")
		}
		if c.maxGasFeeCap == nil {
			return errors.New("server must config 'MaxGasFeeCap'")
		}
		if c.maxGasTipCap.Cmp(c.maxGasFeeCap) > 0 {
			return errors.New("must satisfy 'MaxGasTipCap <= MaxGasFeeCap'")
		}
	}
	for cid := range dynamicFeeTxEnabledChains {
		if _, exist := s.DynamicFeeTx[cid]; !exist {
			return fmt.Errorf("chain %v enabled dynamic fee but without concrete config", cid)
		}
	}
	log.Info("check server dynamic fee tx config success")
	return nil
}

// CheckExtra check extra server config
func (s *RouterServerConfig) CheckExtra() error {
	if s.MaxPlusGasPricePercentage == 0 {
		s.MaxPlusGasPricePercentage = 100 // default value
	}
	if s.PlusGasPricePercentage > s.MaxPlusGasPricePercentage {
		return errors.New("too large 'PlusGasPricePercentage' value")
	}
	if s.MaxGasPriceFluctPercent > 100 {
		return errors.New("too large 'MaxGasPriceFluctPercent' value")
	}
	return nil
}

// CheckConfig check mongodb config
func (c *MongoDBConfig) CheckConfig() error {
	if c.DBName == "" {
		return errors.New("mongodb must config 'DBName'")
	}
	if c.DBURL == "" && len(c.DBURLs) == 0 {
		return errors.New("mongodb must config 'DBURL' or 'DBURLs'")
	}
	if c.DBURL != "" {
		if len(c.DBURLs) != 0 {
			return errors.New("mongodb can not config both 'DBURL' and 'DBURLs'")
		}
		c.DBURLs = splitStringByBlankOrComma(c.DBURL)
	}
	return nil
}

// CheckConfig check onchain config storing chain and token configs
func (c *OnchainConfig) CheckConfig() error {
	if c.IgnoreCheck {
		log.Info("ignore check onchain config")
		return nil
	}
	log.Info("start check onchain config connection")
	if c.Contract == "" {
		return errors.New("onchain must config 'Contract'")
	}
	if len(c.APIAddress) == 0 {
		return errors.New("onchain must config 'APIAddress'")
	}
	if c.ReloadCycle > 0 && c.ReloadCycle < 600 {
		return errors.New("onchain config wrong 'ReloadCycle' value (must be 0 or >= 600)")
	}
	if IsReload {
		return nil
	}
	callGetAllChainIDs := common.FromHex("0xe27112d5")
	for _, apiAddress := range c.APIAddress {
		_, err := CallContractWithGateway(apiAddress, c.Contract, callGetAllChainIDs, "latest")
		if err != nil {
			log.Warn("call getAllChainIDs failed", "gateway", apiAddress, "contract", c.Contract, "err", err)
			continue
		}
		log.Info("check onchain config connection success", "contract", c.Contract)
		return nil
	}
	log.Error("check onchain config connection failed", "gateway", c.APIAddress, "contract", c.Contract)
	return errors.New("check onchain config connection failed")
}

// CheckConfig check mpc config
//nolint:funlen,gocyclo // ok
func (c *MPCConfig) CheckConfig(isServer bool) (err error) {
	if c.SignWithPrivateKey {
		return nil
	}
	if c.GroupID == nil {
		return errors.New("mpc must config 'GroupID'")
	}
	if c.NeededOracles == nil {
		return errors.New("mpc must config 'NeededOracles'")
	}
	if c.TotalOracles == nil {
		return errors.New("mpc must config 'TotalOracles'")
	}
	if !(c.Mode == 0 || c.Mode == 1) {
		return errors.New("mpc must config 'Mode' to 0 (managed) or 1 (private)")
	}
	if len(c.Initiators) == 0 {
		return errors.New("mpc must config 'Initiators'")
	}
	if c.DefaultNode == nil {
		return errors.New("mpc must config 'DefaultNode'")
	}
	err = c.DefaultNode.CheckConfig(isServer)
	if err != nil {
		return err
	}
	for _, mpcNode := range c.OtherNodes {
		err = mpcNode.CheckConfig(isServer)
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckConfig check mpc node config
func (c *MPCNodeConfig) CheckConfig(isServer bool) (err error) {
	if c.RPCAddress == nil || *c.RPCAddress == "" {
		return errors.New("mpc node must config 'RPCAddress'")
	}
	if c.KeystoreFile == nil || *c.KeystoreFile == "" {
		return errors.New("mpc node must config 'KeystoreFile'")
	}
	if c.PasswordFile == nil {
		return errors.New("mpc node must config 'PasswordFile'")
	}
	if isServer && len(c.SignGroups) == 0 {
		return errors.New("swap server mpc node must config 'SignGroups'")
	}
	log.Info("check mpc config pass", "isServer", isServer)
	return nil
}

// CheckConfig check extra config
func (c *ExtraConfig) CheckConfig() (err error) {
	initCallByContractWhitelist()
	initCallByContractCodeHashWhitelist()
	initBigValueWhitelist()
	initDynamicFeeTxEnabledChains()
	initEnableCheckTxBlockHashChains()
	initEnableCheckTxBlockIndexChains()
	initDisableUseFromChainIDInReceiptChains()
	initUseFastMPCChains()
	initDontCheckReceivedTokenIDs()
	initDontCheckBalanceTokenIDs()
	initDontCheckTotalSupplyTokenIDs()
	initCheckTokenBalanceEnabledChains()
	initIgnoreAnycallFallbackAppIDs()

	for cid, cfg := range c.LocalChainConfig {
		if err = cfg.CheckConfig(); err != nil {
			log.Warn("check local chain config failed", "chainID", cid, "err", err)
			return err
		}
	}

	if c.UsePendingBalance {
		GetBalanceBlockNumberOpt = "pending"
	}

	for chainID, baseFeePercent := range c.BaseFeePercent {
		if _, ok := new(big.Int).SetString(chainID, 0); !ok {
			return fmt.Errorf("wrong chain id '%v' in 'BaseFeePercent'", chainID)
		}
		if baseFeePercent < -90 || baseFeePercent > 500 {
			return errors.New("'BaseFeePercent' must be in range [-90, 500]")
		}
	}

	log.Info("check extra config success",
		"minReserveFee", c.MinReserveFee,
		"allowCallByContract", c.AllowCallByContract,
		"baseFeePercent", c.BaseFeePercent,
		"usePendingBalance", c.UsePendingBalance,
	)
	return nil
}

// CheckConfig check local chain config
func (c *LocalChainConfig) CheckConfig() (err error) {
	if c.BigValueDiscount > 100 {
		return errors.New("'BigValueDiscount' is larger than 100")
	}
	return nil
}
