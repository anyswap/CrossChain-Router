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
	err = client.RPCPost(&result, gateway, "eth_call", reqArgs, blockNumber)
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
	log.Info("check identifier pass", "identifier", config.Identifier, "swaptype", config.SwapType, "isServer", isServer)

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

	if config.Onchain == nil {
		return errors.New("server must config 'Onchain'")
	}
	err = config.Onchain.CheckConfig()
	if err != nil {
		return err
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
	for _, chainID := range s.ChainIDBlackList {
		biChainID, ok := new(big.Int).SetString(chainID, 0)
		if !ok {
			return fmt.Errorf("wrong chain id '%v' in black list", chainID)
		}
		key := biChainID.String()
		if _, exist := chainIDBlacklistMap[key]; exist {
			return fmt.Errorf("duplicate chain id '%v' in black list", key)
		}
		chainIDBlacklistMap[key] = struct{}{}
	}
	for _, tokenID := range s.TokenIDBlackList {
		if tokenID == "" {
			return errors.New("empty token id in black list")
		}
		key := strings.ToLower(tokenID)
		if _, exist := tokenIDBlacklistMap[key]; exist {
			return fmt.Errorf("duplicate token id '%v' in black list", key)
		}
		tokenIDBlacklistMap[key] = struct{}{}
	}
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
		if _, exist := fixedGasPriceMap[key]; exist {
			return fmt.Errorf("duplicate chain id '%v' in 'FixedGasPrice'", key)
		}
		fixedGasPriceMap[key] = fixedGasPrice
	}
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
		if _, exist := maxGasPriceMap[key]; exist {
			return fmt.Errorf("duplicate chain id '%v' in 'MaxGasPrice'", key)
		}
		maxGasPriceMap[key] = maxGasPrice
	}
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
		"fixedGasPriceMap", fixedGasPriceMap,
		"maxGasPriceMap", maxGasPriceMap,
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
	if len(c.WSServers) == 0 {
		log.Warn("onchain does not config web socket server, so do not support reload config.")
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
func (c *MPCConfig) CheckConfig(isServer bool) (err error) {
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
	initExclueFeeWhitelist()
	initBigValueWhitelist()
	initDynamicFeeTxEnabledChains()
	initEnableCheckTxBlockHashChains()
	initEnableCheckTxBlockIndexChains()
	initDisableUseFromChainIDInReceiptChains()

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
		"callByContractWhitelist", c.CallByContractWhitelist,
		"exclueFeeWhitelist", c.ExclueFeeWhitelist,
		"bigValueWhitelist", c.BigValueWhitelist,
		"dynamicFeeTxEnabledChains", c.DynamicFeeTxEnabledChains,
		"enableCheckTxBlockHashChains", c.EnableCheckTxBlockHashChains,
		"enableCheckTxBlockIndexChains", c.EnableCheckTxBlockIndexChains,
		"initDisableUseFromChainIDInReceiptChains", c.DisableUseFromChainIDInReceiptChains,
		"baseFeePercent", c.BaseFeePercent,
	)
	return nil
}
