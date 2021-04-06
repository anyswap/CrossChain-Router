package params

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/common/hexutil"
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/rpc/client"
)

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
	log.Info("check identifier pass", "identifier", config.Identifier, "isServer", isServer)
	if isServer {
		err = config.Server.CheckConfig()
		if err != nil {
			return err
		}
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

// CheckConfig of router server
func (s *RouterServerConfig) CheckConfig() error {
	if s.MongoDB == nil {
		return errors.New("server must config 'MongoDB'")
	}
	if s.APIServer == nil {
		return errors.New("server must config 'APIServer'")
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
	log.Info("check server config success")
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
	if c.Disable {
		if IsRouterSwap() {
			return errors.New("forbid disable mpc in router swap")
		}
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
