package params

import (
	"encoding/json"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/log"
)

// router swap constants
const (
	RouterSwapPrefixID = "routerswap"
)

var (
	routerConfig = &RouterConfig{}

	chainIDBlacklistMap = make(map[string]struct{})
	tokenIDBlacklistMap = make(map[string]struct{})
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
	ReplacePlusGasPricePercent uint64 `toml:",omitempty" json:",omitempty"`
	WaitTimeToReplace          int64  `toml:",omitempty" json:",omitempty"` // seconds
	MaxReplaceCount            int    `toml:",omitempty" json:",omitempty"`
	PlusGasPricePercentage     uint64 `toml:",omitempty" json:",omitempty"`
	MaxPlusGasPricePercentage  uint64 `toml:",omitempty" json:",omitempty"`
	MaxGasPriceFluctPercent    uint64 `toml:",omitempty" json:",omitempty"`
	DefaultGasLimit            uint64 `toml:",omitempty" json:",omitempty"`
	SwapDeadlineOffset         int64  `toml:",omitempty" json:",omitempty"` // seconds
}

// RouterConfig config
type RouterConfig struct {
	Server *RouterServerConfig `toml:",omitempty" json:",omitempty"`

	Identifier  string
	Onchain     *OnchainConfig
	Gateways    map[string][]string // key is chain ID
	GatewaysExt map[string][]string `toml:",omitempty" json:",omitempty"` // key is chain ID
	MPC         *MPCConfig
}

// OnchainConfig struct
type OnchainConfig struct {
	Contract   string
	APIAddress []string
	WSServers  []string
}

// MPCConfig mpc related config
type MPCConfig struct {
	APIPrefix     string
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

// GetIdentifier get identifier (to distiguish in mpc accept)
func GetIdentifier() string {
	return GetRouterConfig().Identifier
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

	routerConfig = config
	return routerConfig
}
