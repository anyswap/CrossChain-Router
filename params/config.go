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

	callByContractWhitelist map[string]map[string]struct{} // chainID -> caller
	exclueFeeWhitelist      map[string]map[string]struct{} // tokenID -> caller
	bigValueWhitelist       map[string]map[string]struct{} // tokenID -> caller

	dynamicFeeTxEnabledChains            map[string]struct{}
	enableCheckTxBlockHashChains         map[string]struct{}
	enableCheckTxBlockIndexChains        map[string]struct{}
	disableUseFromChainIDInReceiptChains map[string]struct{}

	isDebugMode           *bool
	isAllowCallByContract *bool
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

// RouterOracleConfig only for oracle
type RouterOracleConfig struct {
	ServerAPIAddress string
}

// RouterConfig config
type RouterConfig struct {
	Server *RouterServerConfig `toml:",omitempty" json:",omitempty"`
	Oracle *RouterOracleConfig `toml:",omitempty" json:",omitempty"`

	Identifier  string
	SwapType    string
	Onchain     *OnchainConfig
	Gateways    map[string][]string // key is chain ID
	GatewaysExt map[string][]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	MPC         *MPCConfig
	Extra       *ExtraConfig `toml:",omitempty" json:",omitempty"`
}

// ExtraConfig extra config
type ExtraConfig struct {
	IsDebugMode     bool `toml:",omitempty" json:",omitempty"`
	EnableSwapTrade bool `toml:",omitempty" json:",omitempty"`

	MinReserveFee  map[string]uint64 `toml:",omitempty" json:",omitempty"`
	BaseFeePercent map[string]int64  `toml:",omitempty" json:",omitempty"` // key is chain ID

	GetAcceptListInterval uint64 `toml:",omitempty" json:",omitempty"`

	AllowCallByContract     bool
	CallByContractWhitelist map[string][]string // chainID -> whitelist
	ExclueFeeWhitelist      map[string][]string // tokenID -> whitelist
	BigValueWhitelist       map[string][]string // tokenID -> whitelist

	DynamicFeeTxEnabledChains            []string
	EnableCheckTxBlockHashChains         []string
	EnableCheckTxBlockIndexChains        []string
	DisableUseFromChainIDInReceiptChains []string
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
	Port             int
	AllowedOrigins   []string
	MaxRequestsLimit int
}

// MongoDBConfig mongodb config
type MongoDBConfig struct {
	DBURL    string   `toml:",omitempty" json:",omitempty"`
	DBURLs   []string `toml:",omitempty" json:",omitempty"`
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

// GetSwapType get router swap type
func GetSwapType() string {
	return GetRouterConfig().SwapType
}

// IsSwapTradeEnabled is swap trade enabled
func IsSwapTradeEnabled() bool {
	return GetExtraConfig() != nil && GetExtraConfig().EnableSwapTrade
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

// GetBaseFeePercent get base fee percent
func GetBaseFeePercent(chainID string) int64 {
	extraCfg := GetExtraConfig()
	if extraCfg == nil {
		return 0
	}
	if baseFeePercent, exist := GetExtraConfig().BaseFeePercent[chainID]; exist {
		return baseFeePercent
	}
	return 0
}

// IsDebugMode is debug mode, add more debugging log infos
func IsDebugMode() bool {
	if isDebugMode == nil {
		flag := GetExtraConfig() != nil && GetExtraConfig().IsDebugMode
		isDebugMode = &flag
	}
	return *isDebugMode
}

// AllowCallByContract allow call into router from contract
func AllowCallByContract() bool {
	if isAllowCallByContract == nil {
		flag := GetExtraConfig() != nil && GetExtraConfig().AllowCallByContract
		isAllowCallByContract = &flag
	}
	return *isAllowCallByContract
}

// SetAllowCallByContract set allow call by contract flag (used in testing)
func SetAllowCallByContract(allow bool) {
	if isAllowCallByContract == nil {
		isAllowCallByContract = &allow
	} else {
		*isAllowCallByContract = allow
	}
}

func initCallByContractWhitelist() {
	callByContractWhitelist = make(map[string]map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().CallByContractWhitelist) == 0 {
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
	log.Info("initCallByContractWhitelist success")
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

func initExclueFeeWhitelist() {
	exclueFeeWhitelist = make(map[string]map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().ExclueFeeWhitelist) == 0 {
		return
	}
	for tid, whitelist := range GetExtraConfig().ExclueFeeWhitelist {
		whitelistMap := make(map[string]struct{}, len(whitelist))
		for _, address := range whitelist {
			if !common.IsHexAddress(address) {
				log.Fatal("initExclueFeeWhitelist wrong address", "tokenID", tid, "address", address)
			}
			whitelistMap[strings.ToLower(address)] = struct{}{}
		}
		exclueFeeWhitelist[tid] = whitelistMap
	}
	log.Info("initExclueFeeWhitelist success")
}

// IsInExclueFeeWhitelist is in call by contract whitelist
func IsInExclueFeeWhitelist(tokenID, caller string) bool {
	whitelist, exist := exclueFeeWhitelist[tokenID]
	if !exist {
		return false
	}
	_, exist = whitelist[strings.ToLower(caller)]
	return exist
}

func initBigValueWhitelist() {
	bigValueWhitelist = make(map[string]map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().BigValueWhitelist) == 0 {
		return
	}
	for tid, whitelist := range GetExtraConfig().BigValueWhitelist {
		whitelistMap := make(map[string]struct{}, len(whitelist))
		for _, address := range whitelist {
			if !common.IsHexAddress(address) {
				log.Fatal("initBigValueWhitelist wrong address", "tokenID", tid, "address", address)
			}
			whitelistMap[strings.ToLower(address)] = struct{}{}
		}
		bigValueWhitelist[tid] = whitelistMap
	}
	log.Info("initBigValueWhitelist success")
}

// IsInBigValueWhitelist is in call by contract whitelist
func IsInBigValueWhitelist(tokenID, caller string) bool {
	whitelist, exist := bigValueWhitelist[tokenID]
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

// GetRouterOracleConfig get router oracle config
func GetRouterOracleConfig() *RouterOracleConfig {
	return routerConfig.Oracle
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
	if GetExtraConfig() == nil || len(GetExtraConfig().DynamicFeeTxEnabledChains) == 0 {
		return
	}
	for _, cid := range GetExtraConfig().DynamicFeeTxEnabledChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initDynamicFeeTxEnabledChains wrong chainID", "chainID", cid, "err", err)
		}
		dynamicFeeTxEnabledChains[cid] = struct{}{}
	}
	log.Info("initDynamicFeeTxEnabledChains success")
}

// IsDynamicFeeTxEnabled is dynamic fee tx enabled (EIP-1559)
func IsDynamicFeeTxEnabled(chainID string) bool {
	_, exist := dynamicFeeTxEnabledChains[chainID]
	return exist
}

func initEnableCheckTxBlockHashChains() {
	enableCheckTxBlockHashChains = make(map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().EnableCheckTxBlockHashChains) == 0 {
		return
	}
	for _, cid := range GetExtraConfig().EnableCheckTxBlockHashChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initEnableCheckTxBlockHashChains wrong chainID", "chainID", cid, "err", err)
		}
		enableCheckTxBlockHashChains[cid] = struct{}{}
	}
	log.Info("initEnableCheckTxBlockHashChains success")
}

// IsCheckTxBlockHashEnabled check tx block hash
func IsCheckTxBlockHashEnabled(chainID string) bool {
	_, exist := enableCheckTxBlockHashChains[chainID]
	return exist
}

func initEnableCheckTxBlockIndexChains() {
	enableCheckTxBlockIndexChains = make(map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().EnableCheckTxBlockIndexChains) == 0 {
		return
	}
	for _, cid := range GetExtraConfig().EnableCheckTxBlockIndexChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initEnableCheckTxBlockIndexChains wrong chainID", "chainID", cid, "err", err)
		}
		enableCheckTxBlockIndexChains[cid] = struct{}{}
	}
	log.Info("initEnableCheckTxBlockIndexChains success")
}

// IsCheckTxBlockIndexEnabled check tx block and index
func IsCheckTxBlockIndexEnabled(chainID string) bool {
	_, exist := enableCheckTxBlockIndexChains[chainID]
	return exist
}

func initDisableUseFromChainIDInReceiptChains() {
	disableUseFromChainIDInReceiptChains = make(map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().DisableUseFromChainIDInReceiptChains) == 0 {
		return
	}
	for _, cid := range GetExtraConfig().DisableUseFromChainIDInReceiptChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initDisableUseFromChainIDInReceiptChains wrong chainID", "chainID", cid, "err", err)
		}
		disableUseFromChainIDInReceiptChains[cid] = struct{}{}
	}
	log.Info("initDisableUseFromChainIDInReceiptChains success")
}

// IsUseFromChainIDInReceiptDisabled if use fromChainID from receipt log
func IsUseFromChainIDInReceiptDisabled(chainID string) bool {
	_, exist := disableUseFromChainIDInReceiptChains[chainID]
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
	} else {
		config.Oracle = nil
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
