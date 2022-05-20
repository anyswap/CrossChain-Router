// Package mpc is a client of mpc server, doing the sign and accept tasks.
package mpc

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tools"
	"github.com/anyswap/CrossChain-Router/v3/tools/keystore"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

const (
	mpcToAddress       = "0x00000000000000000000000000000000000000dc"
	mpcWalletServiceID = 30400

	signTypeED25519 = "ED25519"
)

var (
	mpcSigner = types.MakeSigner("EIP155", big.NewInt(mpcWalletServiceID))
	mpcToAddr = common.HexToAddress(mpcToAddress)

	mpcConfig     *Config
	fastmpcConfig *Config
)

// Config mpc config
type Config struct {
	IsFastMPC bool

	mpcAPIPrefix     string
	mpcGroupID       string
	mpcThreshold     string
	mpcMode          string
	mpcNeededOracles uint32
	mpcTotalOracles  uint32

	signTypeEC256K1 string

	verifySignatureInAccept bool

	GetAcceptListLoopInterval  uint64
	GetAcceptListRetryInterval uint64
	MaxAcceptSignTimeInterval  int64
	PendingInvalidAccept       bool

	mpcRPCTimeout  int
	mpcSignTimeout time.Duration

	defaultMPCNode    *NodeInfo
	allInitiatorNodes []*NodeInfo // server only

	selfEnode string
	allEnodes []string

	// delete if fail too many times consecutively, 0 means disable checking
	maxSignGroupFailures      int
	minIntervalToAddSignGroup int64                   // seconds
	signGroupFailuresMap      map[string]signFailures // key is groupID
}

type signFailures struct {
	count    int
	lastTime int64
}

func newConfig() *Config {
	return &Config{
		mpcAPIPrefix:    "smpc_",
		mpcRPCTimeout:   10,
		mpcSignTimeout:  120 * time.Second,
		signTypeEC256K1: "EC256K1",

		GetAcceptListLoopInterval:  uint64(5),
		GetAcceptListRetryInterval: uint64(3),
		MaxAcceptSignTimeInterval:  int64(600),
		PendingInvalidAccept:       false,

		maxSignGroupFailures:      0,
		minIntervalToAddSignGroup: int64(3600),
		signGroupFailuresMap:      make(map[string]signFailures),
	}
}

// GetMPCConfig get mpc config
func GetMPCConfig(isFastMPC bool) *Config {
	if isFastMPC {
		return fastmpcConfig
	}
	return mpcConfig
}

func isEC(signType string) bool {
	return strings.HasPrefix(signType, "EC")
}

// NodeInfo mpc node info
type NodeInfo struct {
	keyWrapper             *keystore.Key
	mpcUser                common.Address
	mpcRPCAddress          string
	originSignGroups       []string // origin sub groups for sign
	usableSignGroupIndexes []int    // usable sign groups indexes

	signGroupsLock sync.RWMutex

	parent *Config
}

// Init init mpc
func Init(isServer bool) {
	mpcConfig = InitConfig(params.GetRouterConfig().MPC, isServer)

	if params.GetRouterConfig().FastMPC != nil {
		fastmpcConfig = InitConfig(params.GetRouterConfig().FastMPC, isServer)
		fastmpcConfig.IsFastMPC = true
	}
}

// InitConfig init mpc config
func InitConfig(mpcParams *params.MPCConfig, isServer bool) *Config {
	c := newConfig()

	if mpcParams.SignWithPrivateKey {
		log.Info("ignore mpc init as sign with private key")
		return c
	}

	if mpcParams.SignTypeEC256K1 != "" {
		c.signTypeEC256K1 = mpcParams.SignTypeEC256K1
	}

	if mpcParams.APIPrefix != "" {
		c.mpcAPIPrefix = mpcParams.APIPrefix
	}

	if mpcParams.RPCTimeout > 0 {
		c.mpcRPCTimeout = int(mpcParams.RPCTimeout)
	}
	if mpcParams.SignTimeout > 0 {
		c.mpcSignTimeout = time.Duration(mpcParams.SignTimeout * uint64(time.Second))
	}

	if mpcParams.GetAcceptListLoopInterval > 0 {
		c.GetAcceptListLoopInterval = mpcParams.GetAcceptListLoopInterval
	}
	if mpcParams.GetAcceptListRetryInterval > 0 {
		c.GetAcceptListRetryInterval = mpcParams.GetAcceptListRetryInterval
	}
	if mpcParams.MaxAcceptSignTimeInterval > 0 {
		c.MaxAcceptSignTimeInterval = mpcParams.MaxAcceptSignTimeInterval
	}
	c.PendingInvalidAccept = mpcParams.PendingInvalidAccept

	c.maxSignGroupFailures = mpcParams.MaxSignGroupFailures
	if mpcParams.MinIntervalToAddSignGroup > 0 {
		c.minIntervalToAddSignGroup = mpcParams.MinIntervalToAddSignGroup
	}

	c.verifySignatureInAccept = mpcParams.VerifySignatureInAccept

	c.setMPCGroup(*mpcParams.GroupID, mpcParams.Mode, *mpcParams.NeededOracles, *mpcParams.TotalOracles)
	c.setDefaultMPCNodeInfo(c.initMPCNodeInfo(mpcParams.DefaultNode, isServer))

	if isServer {
		for _, nodeCfg := range mpcParams.OtherNodes {
			c.initMPCNodeInfo(nodeCfg, isServer)
		}
	}

	c.initSelfEnode()
	c.initAllEnodes()

	c.verifyInitiators(mpcParams.Initiators)
	log.Info("init mpc success", "apiPrefix", c.mpcAPIPrefix, "isServer", isServer,
		"rpcTimeout", c.mpcRPCTimeout, "signTimeout", c.mpcSignTimeout.String(),
		"maxSignGroupFailures", c.maxSignGroupFailures,
		"minIntervalToAddSignGroup", c.minIntervalToAddSignGroup,
	)

	return c
}

// setDefaultMPCNodeInfo set default mpc node info
func (c *Config) setDefaultMPCNodeInfo(nodeInfo *NodeInfo) {
	c.defaultMPCNode = nodeInfo
}

// GetAllInitiatorNodes get all initiator mpc node info
func (c *Config) GetAllInitiatorNodes() []*NodeInfo {
	return c.allInitiatorNodes
}

// addInitiatorNode add initiator mpc node info
func (c *Config) addInitiatorNode(nodeInfo *NodeInfo) {
	if nodeInfo.mpcRPCAddress == "" {
		log.Fatal("initiator: empty mpc rpc address")
	}
	if nodeInfo.mpcUser == (common.Address{}) {
		log.Fatal("initiator: empty mpc user")
	}
	if len(nodeInfo.originSignGroups) == 0 {
		log.Fatal("initiator: empty sign groups")
	}
	for _, oldNode := range c.allInitiatorNodes {
		if oldNode.mpcRPCAddress == nodeInfo.mpcRPCAddress ||
			oldNode.mpcUser == nodeInfo.mpcUser {
			log.Fatal("duplicate initiator", "user", nodeInfo.mpcUser, "rpcAddr", nodeInfo.mpcRPCAddress)
		}
	}
	c.allInitiatorNodes = append(c.allInitiatorNodes, nodeInfo)
}

// IsSwapServer returns if this mpc user is the swap server
func (c *Config) IsSwapServer() bool {
	return len(c.allInitiatorNodes) > 0
}

// setMPCGroup set mpc group
func (c *Config) setMPCGroup(group string, mode, neededOracles, totalOracles uint32) {
	c.mpcGroupID = group
	c.mpcNeededOracles = neededOracles
	c.mpcTotalOracles = totalOracles
	c.mpcThreshold = fmt.Sprintf("%d/%d", neededOracles, totalOracles)
	c.mpcMode = fmt.Sprintf("%d", mode)
	log.Info("Init mpc group", "group", c.mpcGroupID, "threshold", c.mpcThreshold, "mode", c.mpcMode)
}

// GetGroupID return mpc group id
func (c *Config) GetGroupID() string {
	return c.mpcGroupID
}

// GetSelfEnode get self enode
func (c *Config) GetSelfEnode() string {
	return c.selfEnode
}

// GetAllEnodes get all enodes
func (c *Config) GetAllEnodes() []string {
	return c.allEnodes
}

// setMPCRPCAddress set mpc node rpc address
func (ni *NodeInfo) setMPCRPCAddress(url string) {
	ni.mpcRPCAddress = url
}

// GetMPCRPCAddress get mpc node rpc address
func (ni *NodeInfo) GetMPCRPCAddress() string {
	return ni.mpcRPCAddress
}

// setOriginSignGroups set origin sign subgroups
func (ni *NodeInfo) setOriginSignGroups(groups []string) {
	ni.originSignGroups = groups
	ni.usableSignGroupIndexes = make([]int, len(groups))
	for i := range groups {
		ni.usableSignGroupIndexes[i] = i
	}

	if ni.parent.maxSignGroupFailures > 0 {
		go ni.checkAndAddSignGroups()
	}
}

// getUsableSignGroupIndexes get usable sign group indexes (by copy in case of parallel)
func (ni *NodeInfo) getUsableSignGroupIndexes() []int {
	if ni.parent.maxSignGroupFailures == 0 {
		return ni.usableSignGroupIndexes
	}

	ni.signGroupsLock.RLock()
	defer ni.signGroupsLock.RUnlock()

	groupIndexes := make([]int, len(ni.usableSignGroupIndexes))
	for i, groupInd := range ni.usableSignGroupIndexes {
		groupIndexes[i] = groupInd
	}

	return groupIndexes
}

// deleteSignGroup delete sign group
func (ni *NodeInfo) deleteSignGroup(groupIndex int) {
	ni.signGroupsLock.Lock()
	defer ni.signGroupsLock.Unlock()

	for i, groupInd := range ni.usableSignGroupIndexes {
		if groupInd == groupIndex {
			ni.usableSignGroupIndexes = append(ni.usableSignGroupIndexes[:i], ni.usableSignGroupIndexes[i+1:]...)
			return
		}
	}
}

// checkAndAddSignGroups add sign group
func (ni *NodeInfo) checkAndAddSignGroups() {
	for {
		usableGroupIndexes := ni.getUsableSignGroupIndexes()
		for i := range ni.originSignGroups {
			usable := false
			for _, groupInd := range usableGroupIndexes {
				if groupInd == i {
					usable = true
					break
				}
			}
			if usable {
				continue
			}
			signGroup := ni.originSignGroups[i]
			signFailure := ni.parent.signGroupFailuresMap[signGroup]
			if signFailure.lastTime+ni.parent.minIntervalToAddSignGroup > time.Now().Unix() {
				continue
			}
			log.Info("check and add sign group", "signGroup", signGroup)
			ni.usableSignGroupIndexes = append(ni.usableSignGroupIndexes, i)
			// reset when readd
			ni.parent.signGroupFailuresMap[signGroup] = signFailures{
				count:    0,
				lastTime: time.Now().Unix(),
			}
		}
		time.Sleep(60 * time.Second)
	}
}

// GetMPCUser returns the mpc user of specified keystore
func (ni *NodeInfo) GetMPCUser() common.Address {
	return ni.mpcUser
}

// LoadKeyStore load keystore
func (ni *NodeInfo) LoadKeyStore(keyfile, passfile string) (common.Address, error) {
	key, err := tools.LoadKeyStore(keyfile, passfile)
	if err != nil {
		return common.Address{}, err
	}
	ni.keyWrapper = key
	ni.mpcUser = ni.keyWrapper.Address
	return ni.mpcUser, nil
}

func (c *Config) initSelfEnode() {
	for {
		enode, err := c.GetEnode(c.defaultMPCNode.mpcRPCAddress)
		if err == nil {
			c.selfEnode = enode
			log.Info("get mpc enode info success", "enode", enode)
			return
		}
		log.Error("can't get enode info", "rpcAddr", c.defaultMPCNode.mpcRPCAddress, "err", err)
		time.Sleep(10 * time.Second)
	}
}

func isEnodeExistIn(enode string, enodes []string) bool {
	sepIndex := strings.Index(enode, "@")
	if sepIndex == -1 {
		log.Fatal("wrong self enode, has no '@' char", "enode", enode)
	}
	cmpStr := enode[:sepIndex]
	for _, item := range enodes {
		if item[:sepIndex] == cmpStr {
			return true
		}
	}
	return false
}

func (c *Config) initAllEnodes() {
	c.allEnodes = c.verifySignGroupInfo(c.defaultMPCNode.mpcRPCAddress, c.mpcGroupID, false, true)
}

func (c *Config) verifySignGroupInfo(rpcAddr, groupID string, isSignGroup, includeSelf bool) []string {
	memberCount := c.mpcTotalOracles
	if isSignGroup {
		memberCount = c.mpcNeededOracles
	}
	for {
		groupInfo, err := c.GetGroupByID(groupID, rpcAddr)
		if err != nil {
			log.Error("get group info failed", "groupID", groupID, "err", err)
			time.Sleep(10 * time.Second)
			continue
		}
		log.Info("get mpc group info success", "groupInfo", groupInfo)
		if uint32(groupInfo.Count) != memberCount {
			log.Fatal("mpc group member count mismatch", "groupID", c.mpcGroupID, "have", groupInfo.Count, "want", memberCount)
		}
		if uint32(len(groupInfo.Enodes)) != memberCount {
			log.Fatal("get group info enodes count mismatch", "groupID", groupID, "have", len(groupInfo.Enodes), "want", memberCount)
		}
		exist := isEnodeExistIn(c.selfEnode, groupInfo.Enodes)
		if exist != includeSelf {
			log.Fatal("self enode's existence in group mismatch", "groupID", groupID, "groupInfo", groupInfo, "want", includeSelf, "have", exist)
		}
		if isSignGroup {
			for _, enode := range groupInfo.Enodes {
				if !isEnodeExistIn(enode, c.allEnodes) {
					log.Fatal("sign group has unrelated enode", "groupID", groupID, "enode", enode)
				}
			}
		}
		return groupInfo.Enodes
	}
}

func (c *Config) verifyInitiators(initiators []string) {
	allInitiatorNodes := c.allInitiatorNodes
	if len(allInitiatorNodes) == 0 {
		return
	}
	if len(initiators) != len(allInitiatorNodes) {
		log.Fatal("initiators count mismatch", "initiators", len(initiators), "initiatorNodes", len(allInitiatorNodes))
	}

	isInGroup := true
	for _, mpcNodeInfo := range allInitiatorNodes {
		exist := false
		mpcUser := mpcNodeInfo.mpcUser.String()
		for _, initiator := range initiators {
			if strings.EqualFold(initiator, mpcUser) {
				exist = true
			}
		}
		if !exist {
			log.Fatal("initiator misatch", "user", mpcUser)
		}
		for _, signGroupID := range mpcNodeInfo.originSignGroups {
			c.verifySignGroupInfo(mpcNodeInfo.mpcRPCAddress, signGroupID, true, isInGroup)
		}
		isInGroup = false
	}
}

func (c *Config) initMPCNodeInfo(mpcNodeCfg *params.MPCNodeConfig, isServer bool) *NodeInfo {
	mpcNodeInfo := &NodeInfo{parent: c}
	mpcNodeInfo.setMPCRPCAddress(*mpcNodeCfg.RPCAddress)
	log.Info("Init mpc rpc address", "rpcaddress", *mpcNodeCfg.RPCAddress)

	mpcUser, err := mpcNodeInfo.LoadKeyStore(*mpcNodeCfg.KeystoreFile, *mpcNodeCfg.PasswordFile)
	if err != nil {
		log.Fatalf("load keystore error %v", err)
	}
	log.Info("Init mpc, load keystore success", "user", mpcUser.String())

	if isServer {
		signGroups := mpcNodeCfg.SignGroups
		log.Info("Init mpc sign groups", "signGroups", signGroups)
		mpcNodeInfo.setOriginSignGroups(signGroups)
		c.addInitiatorNode(mpcNodeInfo)
	}

	return mpcNodeInfo
}

// IsMPCInitiator is initiator of mpc sign
func (c *Config) IsMPCInitiator(account string) bool {
	for _, mpcNodeInfo := range c.allInitiatorNodes {
		if strings.EqualFold(account, mpcNodeInfo.mpcUser.String()) {
			return true
		}
	}
	return false
}
