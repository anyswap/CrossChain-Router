package params

import (
	"encoding/json"
	"math/big"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
)

// router swap constants
const (
	RouterSwapPrefixID = "routerswap"
)

var (
	routerConfig = &RouterConfig{}
	locDataDir   string

	chainIDBlacklistMap = make(map[string]struct{})
	tokenIDBlacklistMap = make(map[string]struct{})
	fixedGasPriceMap    = make(map[string]*big.Int) // key is chainID
	maxGasPriceMap      = make(map[string]*big.Int) // key is chainID

	callByContractWhitelist   map[string]map[string]struct{} // chainID -> caller
	dynamicFeeTxEnabledChains map[string]struct{}
)

// RouterServerConfig only for server
type RouterServerConfig struct {
	Admins    []string
	MongoDB   *MongoDBConfig
	APIServer *APIServerConfig

	ChainIDBlackList []string `toml:",omitempty" json:",omitempty"`
	TokenIDBlackList []string `toml:",omitempty" json:",omitempty"`

	// extras
	EnableReplaceSwap          bool
	EnablePassBigValueSwap     bool
	ReplacePlusGasPricePercent uint64            `toml:",omitempty" json:",omitempty"`
	WaitTimeToReplace          int64             `toml:",omitempty" json:",omitempty"` // seconds
	MaxReplaceCount            int               `toml:",omitempty" json:",omitempty"`
	PlusGasPricePercentage     uint64            `toml:",omitempty" json:",omitempty"`
	MaxPlusGasPricePercentage  uint64            `toml:",omitempty" json:",omitempty"`
	MaxGasPriceFluctPercent    uint64            `toml:",omitempty" json:",omitempty"`
	SwapDeadlineOffset         int64             `toml:",omitempty" json:",omitempty"` // seconds
	DefaultGasLimit            map[string]uint64 `toml:",omitempty" json:",omitempty"` // key is chain ID
	FixedGasPrice              map[string]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	MaxGasPrice                map[string]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	NoncePassedConfirmInterval map[string]int64  `toml:",omitempty" json:",omitempty"` // key is chain ID

	DynamicFeeTx map[string]*DynamicFeeTxConfig `toml:",omitempty" json:",omitempty"` // key is chain ID
}

// RouterConfig config
type RouterConfig struct {
	Server *RouterServerConfig `toml:",omitempty" json:",omitempty"`

	Identifier  string
	Onchain     *OnchainConfig
	Gateways    map[string][]string // key is chain ID
	GatewaysExt map[string][]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	MPC         *MPCConfig
	Extra       *ExtraConfig `toml:",omitempty" json:",omitempty"`
}

// ExtraConfig extra config
type ExtraConfig struct {
	IsDebugMode   bool              `toml:",omitempty" json:",omitempty"`
	MinReserveFee map[string]uint64 `toml:",omitempty" json:",omitempty"`

	GetAcceptListInterval uint64 `toml:",omitempty" json:",omitempty"`

	AllowCallByContract     bool
	CallByContractWhitelist map[string][]string // chainID -> whitelist

	DynamicFeeTxEnabledChains []string
}

// OnchainConfig struct
type OnchainConfig struct {
	Contract    string
	APIAddress  []string
	WSServers   []string
	ReloadCycle uint64 // seconds
}

// MPCConfig mpc related config
type MPCConfig struct {
	APIPrefix     string
	RPCTimeout    uint64
	SignTimeout   uint64
	GroupID       *string
	NeededOracles *uint32
	TotalOracles  *uint32
	Mode          uint32 // 0:managed 1:private (default 0)
	Initiators    []string
	DefaultNode   *MPCNodeConfig
	OtherNodes    []*MPCNodeConfig `toml:",omitempty" json:",omitempty"`
}

// MPCNodeConfig mpc node config
type MPCNodeConfig struct {
	RPCAddress   *string
	SignGroups   []string `toml:",omitempty" json:",omitempty"`
	KeystoreFile *string  `json:"-"`
	PasswordFile *string  `json:"-"`
}

// APIServerConfig api service config
type APIServerConfig struct {
	Port           int
	AllowedOrigins []string
}

// MongoDBConfig mongodb config
type MongoDBConfig struct {
	DBURL    string
	DBName   string
	UserName string `json:"-"`
	Password string `json:"-"`
}

// DynamicFeeTxConfig dynamic fee tx config
type DynamicFeeTxConfig struct {
	PlusGasTipCapPercent uint64
	PlusGasFeeCapPercent uint64
	BlockCountFeeHistory int
	MaxGasTipCap         string
	MaxGasFeeCap         string

	// cached values
	maxGasTipCap *big.Int
	maxGasFeeCap *big.Int
}

// GetMaxGasTipCap get max gas tip cap
func (c *DynamicFeeTxConfig) GetMaxGasTipCap() *big.Int {
	return c.maxGasTipCap
}

// GetMaxGasFeeCap get max fee gas cap
func (c *DynamicFeeTxConfig) GetMaxGasFeeCap() *big.Int {
	return c.maxGasFeeCap
}

// GetIdentifier get identifier (to distiguish in mpc accept)
func GetIdentifier() string {
	return GetRouterConfig().Identifier
}

// GetAcceptListInterval get accept list interval (seconds)
func GetAcceptListInterval() uint64 {
	if GetExtraConfig() != nil {
		return GetExtraConfig().GetAcceptListInterval
	}
	return 0
}

// GetFixedGasPrice get fixed gas price of specified chain
func GetFixedGasPrice(chainID string) *big.Int {
	if fixedGasPrice, ok := fixedGasPriceMap[chainID]; ok {
		return new(big.Int).Set(fixedGasPrice)
	}
	return nil
}

// GetMaxGasPrice get max gas price of specified chain
func GetMaxGasPrice(chainID string) *big.Int {
	if maxGasPrice, ok := maxGasPriceMap[chainID]; ok {
		return new(big.Int).Set(maxGasPrice)
	}
	return nil
}

// GetNoncePassedConfirmInterval get nonce passed confirm interval
func GetNoncePassedConfirmInterval(chainID string) int64 {
	serverCfg := GetRouterServerConfig()
	if serverCfg != nil {
		return 0
	}
	if interval, exist := serverCfg.NoncePassedConfirmInterval[chainID]; exist {
		return interval
	}
	return 0
}

// GetMinReserveFee get min reserve fee
func GetMinReserveFee(chainID string) *big.Int {
	if GetExtraConfig() == nil {
		return nil
	}
	if minReserve, exist := GetExtraConfig().MinReserveFee[chainID]; exist {
		return new(big.Int).SetUint64(minReserve)
	}
	return nil
}

// IsDebugMode is debug mode, add more debugging log infos
func IsDebugMode() bool {
	return GetExtraConfig() != nil && GetExtraConfig().IsDebugMode
}

// AllowCallByContract allow call into router from contract
func AllowCallByContract() bool {
	return GetExtraConfig() != nil && GetExtraConfig().AllowCallByContract
}

// SetAllowCallByContract set allow call by contract flag
func SetAllowCallByContract(allow bool) {
	extraCfg := GetExtraConfig()
	if extraCfg == nil {
		extraCfg = &ExtraConfig{}
		routerConfig.Extra = extraCfg
	}
	extraCfg.AllowCallByContract = allow
}

func initCallByContractWhitelist() {
	callByContractWhitelist = make(map[string]map[string]struct{})
	if GetExtraConfig() == nil {
		return
	}
	for cid, whitelist := range GetExtraConfig().CallByContractWhitelist {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initCallByContractWhitelist wrong chainID", "chainID", cid, "err", err)
		}
		whitelistMap := make(map[string]struct{}, len(whitelist))
		for _, address := range whitelist {
			if !common.IsHexAddress(address) {
				log.Fatal("initCallByContractWhitelist wrong address", "chainID", cid, "address", address)
			}
			whitelistMap[strings.ToLower(address)] = struct{}{}
		}
		callByContractWhitelist[cid] = whitelistMap
	}
	log.Info("initCallByContractWhitelist success", "callByContractWhitelist", callByContractWhitelist)
}

// IsInCallByContractWhitelist is in call by contract whitelist
func IsInCallByContractWhitelist(chainID, caller string) bool {
	whitelist, exist := callByContractWhitelist[chainID]
	if !exist {
		return false
	}
	_, exist = whitelist[strings.ToLower(caller)]
	return exist
}

// IsMPCInitiator is initiator of mpc sign
func IsMPCInitiator(account string) bool {
	initiators := GetRouterConfig().MPC.Initiators
	for _, initiator := range initiators {
		if strings.EqualFold(account, initiator) {
			return true
		}
	}
	return false
}

// GetRouterConfig get router config
func GetRouterConfig() *RouterConfig {
	return routerConfig
}

// GetRouterServerConfig get router server config
func GetRouterServerConfig() *RouterServerConfig {
	return routerConfig.Server
}

// GetOnchainContract get onchain config contract address
func GetOnchainContract() string {
	return routerConfig.Onchain.Contract
}

// GetExtraConfig get extra config
func GetExtraConfig() *ExtraConfig {
	return routerConfig.Extra
}

// HasRouterAdmin has admin
func HasRouterAdmin() bool {
	return len(routerConfig.Server.Admins) != 0
}

// IsRouterAdmin is admin
func IsRouterAdmin(account string) bool {
	for _, admin := range routerConfig.Server.Admins {
		if strings.EqualFold(account, admin) {
			return true
		}
	}
	return false
}

// IsChainIDInBlackList is chain id in black list
func IsChainIDInBlackList(chainID string) bool {
	_, exist := chainIDBlacklistMap[chainID]
	return exist
}

// IsTokenIDInBlackList is token id in black list
func IsTokenIDInBlackList(tokenID string) bool {
	_, exist := tokenIDBlacklistMap[strings.ToLower(tokenID)]
	return exist
}

// IsSwapInBlacklist is chain or token blacklisted
func IsSwapInBlacklist(fromChainID, toChainID, tokenID string) bool {
	return IsChainIDInBlackList(fromChainID) ||
		IsChainIDInBlackList(toChainID) ||
		IsTokenIDInBlackList(tokenID)
}

func initDynamicFeeTxEnabledChains() {
	dynamicFeeTxEnabledChains = make(map[string]struct{})
	if GetExtraConfig() == nil {
		return
	}
	for _, cid := range GetExtraConfig().DynamicFeeTxEnabledChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initDynamicFeeTxEnabledChains wrong chainID", "chainID", cid, "err", err)
		}
		dynamicFeeTxEnabledChains[cid] = struct{}{}
	}
	log.Info("initDynamicFeeTxEnabledChains success", "dynamicFeeTxEnabledChains", dynamicFeeTxEnabledChains)
}

// IsDynamicFeeTxEnabled is dynamic fee tx enabled (EIP-1559)
func IsDynamicFeeTxEnabled(chainID string) bool {
	_, exist := dynamicFeeTxEnabledChains[chainID]
	return exist
}

// GetDynamicFeeTxConfig get dynamic fee tx config (EIP-1559)
func GetDynamicFeeTxConfig(chainID string) *DynamicFeeTxConfig {
	if !IsDynamicFeeTxEnabled(chainID) {
		return nil
	}
	serverCfg := GetRouterServerConfig()
	if serverCfg == nil {
		return nil
	}
	if cfg, exist := serverCfg.DynamicFeeTx[chainID]; exist {
		return cfg
	}
	return nil
}

// LoadRouterConfig load router swap config
func LoadRouterConfig(configFile string, isServer bool) *RouterConfig {
	if configFile == "" {
		log.Fatal("must specify config file")
	}
	log.Info("load router config file", "configFile", configFile, "isServer", isServer)
	if !common.FileExist(configFile) {
		log.Fatalf("LoadRouterConfig error: config file '%v' not exist", configFile)
	}
	config := &RouterConfig{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Fatalf("LoadRouterConfig error (toml DecodeFile): %v", err)
	}

	if !isServer {
		config.Server = nil
	}

	routerConfig = config

	var bs []byte
	if log.JSONFormat {
		bs, _ = json.Marshal(config)
	} else {
		bs, _ = json.MarshalIndent(config, "", "  ")
	}
	log.Println("LoadRouterConfig finished.", string(bs))

	if err := config.CheckConfig(isServer); err != nil {
		log.Fatalf("Check config failed. %v", err)
	}

	return routerConfig
}

// SetDataDir set data dir
func SetDataDir(dir string, isServer bool) {
	if dir == "" {
		if !isServer {
			log.Warn("suggest specify '--datadir' to enhance accept job")
		}
		return
	}
	currDir, err := common.CurrentDir()
	if err != nil {
		log.Fatal("get current dir failed", "err", err)
	}
	locDataDir = common.AbsolutePath(currDir, dir)
	log.Info("set data dir success", "datadir", locDataDir)
}

// GetDataDir get data dir
func GetDataDir() string {
	return locDataDir
}
