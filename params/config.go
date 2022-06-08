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

// IsTestMode used for testing
var IsTestMode bool

var (
	routerConfig = &RouterConfig{Extra: &ExtraConfig{}, MPC: &MPCConfig{}}

	routerConfigFile string
	locDataDir       string

	// IsSwapServer is swap server
	IsSwapServer bool

	chainIDBlacklistMap = make(map[string]struct{})
	tokenIDBlacklistMap = make(map[string]struct{})
	accountBlacklistMap = make(map[string]struct{})
	fixedGasPriceMap    = make(map[string]*big.Int) // key is chainID
	maxGasPriceMap      = make(map[string]*big.Int) // key is chainID

	callByContractWhitelist         map[string]map[string]struct{} // chainID -> caller
	callByContractCodeHashWhitelist map[string]map[string]struct{} // chainID -> codehash
	bigValueWhitelist               map[string]map[string]struct{} // tokenID -> caller

	autoSwapNonceEnabledChains map[string]struct{}

	dynamicFeeTxEnabledChains            map[string]struct{}
	enableCheckTxBlockHashChains         map[string]struct{}
	enableCheckTxBlockIndexChains        map[string]struct{}
	disableUseFromChainIDInReceiptChains map[string]struct{}
	useFastMPCChains                     map[string]struct{}
	dontCheckReceivedTokenIDs            map[string]struct{}

	isDebugMode           *bool
	isNFTSwapWithData     *bool
	isAllowCallByContract *bool
	isCheckEIP1167Master  *bool
)

// exported variables
var (
	GetBalanceBlockNumberOpt = "latest" // latest or pending
)

// RouterServerConfig only for server
type RouterServerConfig struct {
	Admins     []string
	Assistants []string
	MongoDB    *MongoDBConfig
	APIServer  *APIServerConfig

	ChainIDBlackList []string `toml:",omitempty" json:",omitempty"`
	TokenIDBlackList []string `toml:",omitempty" json:",omitempty"`
	AccountBlackList []string `toml:",omitempty" json:",omitempty"`

	AutoSwapNonceEnabledChains []string `toml:",omitempty" json:",omitempty"`

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
	CalcGasPriceMethod         map[string]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	RetrySendTxLoopCount       map[string]int    `toml:",omitempty" json:",omitempty"` // key is chain ID
	SendTxLoopCount            map[string]int    `toml:",omitempty" json:",omitempty"` // key is chain ID
	SendTxLoopInterval         map[string]int    `toml:",omitempty" json:",omitempty"` // key is chain ID

	DynamicFeeTx map[string]*DynamicFeeTxConfig `toml:",omitempty" json:",omitempty"` // key is chain ID
}

// RouterOracleConfig only for oracle
type RouterOracleConfig struct {
	ServerAPIAddress        string
	NoCheckServerConnection bool
}

// RouterConfig config
type RouterConfig struct {
	Server *RouterServerConfig `toml:",omitempty" json:",omitempty"`
	Oracle *RouterOracleConfig `toml:",omitempty" json:",omitempty"`

	Identifier  string
	SwapType    string
	SwapSubType string
	Onchain     *OnchainConfig
	Gateways    map[string][]string // key is chain ID
	GatewaysExt map[string][]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	MPC         *MPCConfig
	FastMPC     *MPCConfig   `toml:",omitempty" json:",omitempty"`
	Extra       *ExtraConfig `toml:",omitempty" json:",omitempty"`
}

// ExtraConfig extra config
type ExtraConfig struct {
	IsDebugMode           bool `toml:",omitempty" json:",omitempty"`
	EnableSwapTrade       bool `toml:",omitempty" json:",omitempty"`
	EnableSwapWithPermit  bool `toml:",omitempty" json:",omitempty"`
	ForceAnySwapInAuto    bool `toml:",omitempty" json:",omitempty"`
	IsNFTSwapWithData     bool `toml:",omitempty" json:",omitempty"`
	EnableParallelSwap    bool `toml:",omitempty" json:",omitempty"`
	UsePendingBalance     bool `toml:",omitempty" json:",omitempty"`
	DontPanicInInitRouter bool `toml:",omitempty" json:",omitempty"`

	MinReserveFee    map[string]uint64 `toml:",omitempty" json:",omitempty"`
	BaseFeePercent   map[string]int64  `toml:",omitempty" json:",omitempty"` // key is chain ID
	MinReserveBudget map[string]uint64 `toml:",omitempty" json:",omitempty"`

	AllowCallByConstructor          bool                `toml:",omitempty" json:",omitempty"`
	AllowCallByContract             bool                `toml:",omitempty" json:",omitempty"`
	CheckEIP1167Master              bool                `toml:",omitempty" json:",omitempty"`
	CallByContractWhitelist         map[string][]string `toml:",omitempty" json:",omitempty"` // chainID -> whitelist
	CallByContractCodeHashWhitelist map[string][]string `toml:",omitempty" json:",omitempty"` // chainID -> whitelist
	BigValueWhitelist               map[string][]string `toml:",omitempty" json:",omitempty"` // tokenID -> whitelist

	DynamicFeeTxEnabledChains            []string `toml:",omitempty" json:",omitempty"`
	EnableCheckTxBlockHashChains         []string `toml:",omitempty" json:",omitempty"`
	EnableCheckTxBlockIndexChains        []string `toml:",omitempty" json:",omitempty"`
	DisableUseFromChainIDInReceiptChains []string `toml:",omitempty" json:",omitempty"`
	UseFastMPCChains                     []string `toml:",omitempty" json:",omitempty"`
	DontCheckReceivedTokenIDs            []string `toml:",omitempty" json:",omitempty"`

	RPCClientTimeout map[string]int `toml:",omitempty" json:",omitempty"` // key is chainID
	// chainID,customKey => customValue
	Customs map[string]map[string]string `toml:",omitempty" json:",omitempty"`
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
	SignTypeEC256K1 string `toml:",omitempty" json:",omitempty"`

	APIPrefix                 string
	RPCTimeout                uint64 `toml:",omitempty" json:",omitempty"`
	SignTimeout               uint64 `toml:",omitempty" json:",omitempty"`
	MaxSignGroupFailures      int    `toml:",omitempty" json:",omitempty"`
	MinIntervalToAddSignGroup int64  `toml:",omitempty" json:",omitempty"`

	VerifySignatureInAccept bool `toml:",omitempty" json:",omitempty"`

	GetAcceptListLoopInterval  uint64 `toml:",omitempty" json:",omitempty"`
	GetAcceptListRetryInterval uint64 `toml:",omitempty" json:",omitempty"`
	MaxAcceptSignTimeInterval  int64  `toml:",omitempty" json:",omitempty"`
	PendingInvalidAccept       bool   `toml:",omitempty" json:",omitempty"`

	GroupID       *string
	NeededOracles *uint32
	TotalOracles  *uint32
	Mode          uint32 // 0:managed 1:private (default 0)
	Initiators    []string
	DefaultNode   *MPCNodeConfig
	OtherNodes    []*MPCNodeConfig `toml:",omitempty" json:",omitempty"`

	SignWithPrivateKey bool              // use private key instead (use for testing)
	SignerPrivateKeys  map[string]string `json:"-"` // key is chain ID (use for testing)
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

// GetSwapSubType get router swap sub type
func GetSwapSubType() string {
	return GetRouterConfig().SwapSubType
}

// IsSwapTradeEnabled is swap trade enabled
func IsSwapTradeEnabled() bool {
	return GetExtraConfig() != nil && GetExtraConfig().EnableSwapTrade
}

// IsSwapWithPermitEnabled is swap with permit enabled
func IsSwapWithPermitEnabled() bool {
	return GetExtraConfig() != nil && GetExtraConfig().EnableSwapWithPermit
}

// IsForceAnySwapInAuto is forcely call anySwapinAuto
func IsForceAnySwapInAuto() bool {
	return GetExtraConfig() != nil && GetExtraConfig().ForceAnySwapInAuto
}

// IsParallelSwapEnabled is parallel swap enabled
func IsParallelSwapEnabled() bool {
	return GetExtraConfig() != nil && GetExtraConfig().EnableParallelSwap
}

// IsFixedGasPrice is fixed gas price of specified chain
func IsFixedGasPrice(chainID string) bool {
	_, exist := fixedGasPriceMap[chainID]
	return exist
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
	if serverCfg == nil {
		return 0
	}
	if interval, exist := serverCfg.NoncePassedConfirmInterval[chainID]; exist {
		return interval
	}
	return 0
}

// GetCalcGasPriceMethod get calc gas price method eg. median (default), first, max, etc.
func GetCalcGasPriceMethod(chainID string) string {
	serverCfg := GetRouterServerConfig()
	if serverCfg != nil {
		if method, exist := serverCfg.CalcGasPriceMethod[chainID]; exist {
			return method
		}
	}
	return "median" // default value
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

// HasMinReserveBudgetConfig has min reserve budget config
func HasMinReserveBudgetConfig() bool {
	return GetExtraConfig() != nil && len(GetExtraConfig().MinReserveBudget) > 0
}

// GetMinReserveBudget get min reserve budget
func GetMinReserveBudget(chainID string) *big.Int {
	if GetExtraConfig() == nil {
		return nil
	}
	if minReserve, exist := GetExtraConfig().MinReserveBudget[chainID]; exist {
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

// GetRPCClientTimeout get rpc client timeout
func GetRPCClientTimeout(chainID string) int {
	extraCfg := GetExtraConfig()
	if extraCfg == nil {
		return 0
	}
	return extraCfg.RPCClientTimeout[chainID]
}

// GetCustom get custom
func GetCustom(chainID, key string) string {
	extraCfg := GetExtraConfig()
	if extraCfg == nil {
		return ""
	}
	mapping, exist := extraCfg.Customs[chainID]
	if !exist || len(mapping) == 0 {
		return ""
	}
	if value, exist := mapping[key]; exist {
		return value
	}
	return ""
}

// GetSignerPrivateKey get signer private key (use for testing)
func (c *MPCConfig) GetSignerPrivateKey(chainID string) string {
	if prikey, exist := c.SignerPrivateKeys[chainID]; exist {
		return prikey
	}
	return ""
}

// SetSignerPrivateKey set signer private key (use for testing)
func (c *MPCConfig) SetSignerPrivateKey(chainID, prikey string) {
	c.SignWithPrivateKey = true
	if len(c.SignerPrivateKeys) == 0 {
		c.SignerPrivateKeys = make(map[string]string)
	}
	c.SignerPrivateKeys[chainID] = prikey
}

// IsDebugMode is debug mode, add more debugging log infos
func IsDebugMode() bool {
	if isDebugMode == nil {
		flag := GetExtraConfig() != nil && GetExtraConfig().IsDebugMode
		isDebugMode = &flag
	}
	return *isDebugMode
}

// SetDebugMode set debug mode
func SetDebugMode(flag bool) {
	if isDebugMode == nil {
		isDebugMode = &flag
	} else {
		*isDebugMode = flag
	}
}

// IsNFTSwapWithData is nft swap with data, add data in swapout log and swapin argument
func IsNFTSwapWithData() bool {
	if isNFTSwapWithData == nil {
		flag := GetExtraConfig() != nil && GetExtraConfig().IsNFTSwapWithData
		isNFTSwapWithData = &flag
	}
	return *isNFTSwapWithData
}

// AllowCallByConstructor allow call by constructor
func AllowCallByConstructor() bool {
	return GetExtraConfig() != nil && GetExtraConfig().AllowCallByConstructor
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

// CheckEIP1167Master whether check eip1167 master call by contract
func CheckEIP1167Master() bool {
	if isCheckEIP1167Master == nil {
		flag := GetExtraConfig() != nil && GetExtraConfig().CheckEIP1167Master
		isCheckEIP1167Master = &flag
	}
	return *isCheckEIP1167Master
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

// AddOrRemoveCallByContractWhitelist add or remove call by contract whitelist
//nolint:dupl // allow duplicate
func AddOrRemoveCallByContractWhitelist(chainID string, callers []string, isAdd bool) {
	whitelist, exist := callByContractWhitelist[chainID]
	if !exist {
		if !isAdd {
			return
		}
		callByContractWhitelist = make(map[string]map[string]struct{})
		callByContractWhitelist[chainID] = make(map[string]struct{})
		whitelist = callByContractWhitelist[chainID]
	}
	for _, caller := range callers {
		key := strings.ToLower(caller)
		if isAdd {
			whitelist[key] = struct{}{}
		} else {
			delete(whitelist, key)
		}
	}
	if GetExtraConfig() != nil {
		chainWhitelist := make([]string, 0, len(whitelist))
		for caller := range whitelist {
			chainWhitelist = append(chainWhitelist, caller)
		}
		if GetExtraConfig().CallByContractWhitelist == nil {
			GetExtraConfig().CallByContractWhitelist = make(map[string][]string)
		}
		GetExtraConfig().CallByContractWhitelist[chainID] = chainWhitelist
	}
}

func initCallByContractCodeHashWhitelist() {
	callByContractCodeHashWhitelist = make(map[string]map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().CallByContractCodeHashWhitelist) == 0 {
		return
	}
	for cid, whitelist := range GetExtraConfig().CallByContractCodeHashWhitelist {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initCallByContractCodeHashWhitelist wrong chainID", "chainID", cid, "err", err)
		}
		whitelistMap := make(map[string]struct{}, len(whitelist))
		for _, codehash := range whitelist {
			if !common.IsHexHash(codehash) {
				log.Fatal("initCallByContractCodeHashWhitelist wrong code hash", "chainID", cid, "codehash", codehash)
			}
			whitelistMap[codehash] = struct{}{}
		}
		callByContractCodeHashWhitelist[cid] = whitelistMap
	}
	log.Info("initCallByContractCodeHashWhitelist success")
}

// HasCallByContractCodeHashWhitelist has call by contract code hash whitelist
func HasCallByContractCodeHashWhitelist(chainID string) bool {
	whitelist, exist := callByContractCodeHashWhitelist[chainID]
	return exist && len(whitelist) > 0
}

// IsInCallByContractCodeHashWhitelist is in call by contract code hash whitelist
func IsInCallByContractCodeHashWhitelist(chainID, codehash string) bool {
	whitelist, exist := callByContractCodeHashWhitelist[chainID]
	if !exist {
		return false
	}
	_, exist = whitelist[codehash]
	return exist
}

// AddOrRemoveCallByContractCodeHashWhitelist add or remove call by contract code hash whitelist
func AddOrRemoveCallByContractCodeHashWhitelist(chainID string, codehashes []string, isAdd bool) {
	whitelist, exist := callByContractCodeHashWhitelist[chainID]
	if !exist {
		if !isAdd {
			return
		}
		callByContractCodeHashWhitelist = make(map[string]map[string]struct{})
		callByContractCodeHashWhitelist[chainID] = make(map[string]struct{})
		whitelist = callByContractCodeHashWhitelist[chainID]
	}
	for _, codehash := range codehashes {
		key := codehash
		if isAdd {
			whitelist[key] = struct{}{}
		} else {
			delete(whitelist, key)
		}
	}
	if GetExtraConfig() != nil {
		chainWhitelist := make([]string, 0, len(whitelist))
		for codehash := range whitelist {
			chainWhitelist = append(chainWhitelist, codehash)
		}
		if GetExtraConfig().CallByContractCodeHashWhitelist == nil {
			GetExtraConfig().CallByContractCodeHashWhitelist = make(map[string][]string)
		}
		GetExtraConfig().CallByContractCodeHashWhitelist[chainID] = chainWhitelist
	}
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

// AddOrRemoveBigValueWhitelist add or remove big value whitelist
//nolint:dupl // allow duplicate
func AddOrRemoveBigValueWhitelist(tokenID string, callers []string, isAdd bool) {
	whitelist, exist := bigValueWhitelist[tokenID]
	if !exist {
		if !isAdd {
			return
		}
		bigValueWhitelist = make(map[string]map[string]struct{})
		bigValueWhitelist[tokenID] = make(map[string]struct{})
		whitelist = bigValueWhitelist[tokenID]
	}
	for _, caller := range callers {
		key := strings.ToLower(caller)
		if isAdd {
			whitelist[key] = struct{}{}
		} else {
			delete(whitelist, key)
		}
	}
	if GetExtraConfig() != nil {
		tokenWhitelist := make([]string, 0, len(whitelist))
		for caller := range whitelist {
			tokenWhitelist = append(tokenWhitelist, caller)
		}
		if GetExtraConfig().BigValueWhitelist == nil {
			GetExtraConfig().BigValueWhitelist = make(map[string][]string)
		}
		GetExtraConfig().BigValueWhitelist[tokenID] = tokenWhitelist
	}
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

// GetMPCConfig get mpc config
func GetMPCConfig(isFastMPC bool) *MPCConfig {
	if isFastMPC {
		return routerConfig.FastMPC
	}
	return routerConfig.MPC
}

// GetOnchainContract get onchain config contract address
func GetOnchainContract() string {
	return routerConfig.Onchain.Contract
}

// GetExtraConfig get extra config
func GetExtraConfig() *ExtraConfig {
	return routerConfig.Extra
}

// SetExtraConfig set extra config (used by testing)
func SetExtraConfig(extra *ExtraConfig) error {
	if err := extra.CheckConfig(); err != nil {
		return err
	}
	routerConfig.Extra = extra
	return nil
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

// IsRouterAssistant is router assistants
func IsRouterAssistant(account string) bool {
	for _, assistant := range routerConfig.Server.Assistants {
		if strings.EqualFold(account, assistant) {
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

// AddOrRemoveChainIDBlackList add or remove chainID blacklist
func AddOrRemoveChainIDBlackList(chainIDs []string, isAdd bool) {
	for _, chainID := range chainIDs {
		if isAdd {
			chainIDBlacklistMap[chainID] = struct{}{}
		} else {
			delete(chainIDBlacklistMap, chainID)
		}
	}
	if GetRouterServerConfig() != nil {
		blacklist := make([]string, 0, len(chainIDBlacklistMap))
		for chainID := range chainIDBlacklistMap {
			blacklist = append(blacklist, chainID)
		}
		GetRouterServerConfig().ChainIDBlackList = blacklist
	}
}

// IsTokenIDInBlackList is token id in black list
func IsTokenIDInBlackList(tokenID string) bool {
	_, exist := tokenIDBlacklistMap[strings.ToLower(tokenID)]
	return exist
}

// AddOrRemoveTokenIDBlackList add or remove tokenID blacklist
func AddOrRemoveTokenIDBlackList(tokenIDs []string, isAdd bool) {
	for _, tokenID := range tokenIDs {
		key := strings.ToLower(tokenID)
		if isAdd {
			tokenIDBlacklistMap[key] = struct{}{}
		} else {
			delete(tokenIDBlacklistMap, key)
		}
	}
	if GetRouterServerConfig() != nil {
		blacklist := make([]string, 0, len(tokenIDBlacklistMap))
		for tokenID := range tokenIDBlacklistMap {
			blacklist = append(blacklist, tokenID)
		}
		GetRouterServerConfig().TokenIDBlackList = blacklist
	}
}

// IsAccountInBlackList is account in black list
func IsAccountInBlackList(account string) bool {
	_, exist := accountBlacklistMap[strings.ToLower(account)]
	return exist
}

// AddOrRemoveAccountBlackList add or remove account blacklist
func AddOrRemoveAccountBlackList(accounts []string, isAdd bool) {
	for _, account := range accounts {
		key := strings.ToLower(account)
		if isAdd {
			accountBlacklistMap[key] = struct{}{}
		} else {
			delete(accountBlacklistMap, key)
		}
	}
	if GetRouterServerConfig() != nil {
		blacklist := make([]string, 0, len(accountBlacklistMap))
		for account := range accountBlacklistMap {
			blacklist = append(blacklist, account)
		}
		GetRouterServerConfig().AccountBlackList = blacklist
	}
}

func initAutoSwapNonceEnabledChains() {
	autoSwapNonceEnabledChains = make(map[string]struct{})
	serverCfg := GetRouterServerConfig()
	if serverCfg == nil || len(serverCfg.AutoSwapNonceEnabledChains) == 0 {
		return
	}
	for _, cid := range serverCfg.AutoSwapNonceEnabledChains {
		if _, err := common.GetBigIntFromStr(cid); err != nil {
			log.Fatal("initAutoSwapNonceEnabledChains wrong chainID", "chainID", cid, "err", err)
		}
		autoSwapNonceEnabledChains[cid] = struct{}{}
	}
	log.Info("initAutoSwapNonceEnabledChains success", "chains", serverCfg.AutoSwapNonceEnabledChains)
}

// IsAutoSwapNonceEnabled is auto swap nonce enabled
func IsAutoSwapNonceEnabled(chainID string) bool {
	_, exist := autoSwapNonceEnabledChains[chainID]
	return exist
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

func initUseFastMPCChains() {
	useFastMPCChains = make(map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().UseFastMPCChains) == 0 {
		return
	}
	for _, cid := range GetExtraConfig().UseFastMPCChains {
		useFastMPCChains[cid] = struct{}{}
	}
	log.Info("initUseFastMPCChains success")
}

// IsUseFastMPC is use fast mpc
func IsUseFastMPC(chainID string) bool {
	_, exist := useFastMPCChains[chainID]
	return exist
}

func initDontCheckReceivedTokenIDs() {
	dontCheckReceivedTokenIDs = make(map[string]struct{})
	if GetExtraConfig() == nil || len(GetExtraConfig().DontCheckReceivedTokenIDs) == 0 {
		return
	}
	for _, tid := range GetExtraConfig().DontCheckReceivedTokenIDs {
		dontCheckReceivedTokenIDs[strings.ToLower(tid)] = struct{}{}
	}
	log.Info("initDontCheckReceivedTokenIDs success")
}

// DontCheckTokenReceived do not check token received (a security enhance checking)
func DontCheckTokenReceived(tokenID string) bool {
	_, exist := dontCheckReceivedTokenIDs[strings.ToLower(tokenID)]
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
func LoadRouterConfig(configFile string, isServer, check bool) *RouterConfig {
	if configFile == "" {
		log.Fatal("must specify config file")
	}
	log.Info("load router config file", "configFile", configFile, "isServer", isServer)
	if !common.FileExist(configFile) {
		log.Fatalf("LoadRouterConfig error: config file '%v' not exist", configFile)
	}
	config := &RouterConfig{Extra: &ExtraConfig{}, MPC: &MPCConfig{}}
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

	if check {
		if err := config.CheckConfig(isServer); err != nil {
			log.Fatalf("Check config failed. %v", err)
		}
	}

	routerConfigFile = configFile
	return routerConfig
}

// ReloadRouterConfig reload config
func ReloadRouterConfig() {
	configFile := routerConfigFile
	isServer := IsSwapServer

	log.Info("reload router config file", "configFile", configFile, "isServer", isServer)

	config := &RouterConfig{Extra: &ExtraConfig{}, MPC: &MPCConfig{}}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Errorf("ReloadRouterConfig error (toml DecodeFile): %v", err)
		return
	}

	if !isServer {
		config.Server = nil
	} else {
		config.Oracle = nil
	}

	if err := config.CheckConfig(isServer); err != nil {
		log.Errorf("ReloadRouterConfig check config failed. %v", err)
		return
	}

	var bs []byte
	if log.JSONFormat {
		bs, _ = json.Marshal(config)
	} else {
		bs, _ = json.MarshalIndent(config, "", "  ")
	}
	log.Println("ReloadRouterConfig finished.", string(bs))

	routerConfig = config
}

// SetDataDir set data dir
func SetDataDir(dir string, isServer bool) {
	if dir == "" {
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
