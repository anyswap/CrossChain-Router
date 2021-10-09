// Package mpc is a client of mpc server, doing the sign and accept tasks.
package mpc

import (
	"fmt"
	"math/big"
	"strings"
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
)

var (
	mpcSigner = types.MakeSigner("EIP155", big.NewInt(mpcWalletServiceID))
	mpcToAddr = common.HexToAddress(mpcToAddress)

	mpcAPIPrefix     = "smpc_" // default prefix
	mpcGroupID       string
	mpcThreshold     string
	mpcMode          string
	mpcNeededOracles uint32
	mpcTotalOracles  uint32

	mpcRPCTimeout  = 10                // default to 10 seconds
	mpcSignTimeout = 120 * time.Second // default to 120 seconds

	defaultMPCNode    *NodeInfo
	allInitiatorNodes []*NodeInfo // server only

	selfEnode string
	allEnodes []string
)

// NodeInfo mpc node info
type NodeInfo struct {
	keyWrapper    *keystore.Key
	mpcUser       common.Address
	mpcRPCAddress string
	signGroups    []string // sub groups for sign
}

// Init init mpc
func Init(mpcConfig *params.MPCConfig, isServer bool) {
	if mpcConfig.APIPrefix != "" {
		mpcAPIPrefix = mpcConfig.APIPrefix
	}

	if mpcConfig.RPCTimeout > 0 {
		mpcRPCTimeout = int(mpcConfig.RPCTimeout)
	}
	if mpcConfig.SignTimeout > 0 {
		mpcSignTimeout = time.Duration(mpcConfig.SignTimeout * uint64(time.Second))
	}

	setMPCGroup(*mpcConfig.GroupID, mpcConfig.Mode, *mpcConfig.NeededOracles, *mpcConfig.TotalOracles)
	setDefaultMPCNodeInfo(initMPCNodeInfo(mpcConfig.DefaultNode, isServer))

	if isServer {
		for _, nodeCfg := range mpcConfig.OtherNodes {
			initMPCNodeInfo(nodeCfg, isServer)
		}
	}

	initSelfEnode()
	initAllEnodes()

	verifyInitiators(mpcConfig.Initiators)
	log.Info("init mpc success", "apiPrefix", mpcAPIPrefix, "signTimeout", mpcSignTimeout.String(), "isServer", isServer)
}

// setDefaultMPCNodeInfo set default mpc node info
func setDefaultMPCNodeInfo(nodeInfo *NodeInfo) {
	defaultMPCNode = nodeInfo
}

// GetAllInitiatorNodes get all initiator mpc node info
func GetAllInitiatorNodes() []*NodeInfo {
	return allInitiatorNodes
}

// addInitiatorNode add initiator mpc node info
func addInitiatorNode(nodeInfo *NodeInfo) {
	if nodeInfo.mpcRPCAddress == "" {
		log.Fatal("initiator: empty mpc rpc address")
	}
	if nodeInfo.mpcUser == (common.Address{}) {
		log.Fatal("initiator: empty mpc user")
	}
	if len(nodeInfo.signGroups) == 0 {
		log.Fatal("initiator: empty sign groups")
	}
	for _, oldNode := range allInitiatorNodes {
		if oldNode.mpcRPCAddress == nodeInfo.mpcRPCAddress ||
			oldNode.mpcUser == nodeInfo.mpcUser {
			log.Fatal("duplicate initiator", "user", nodeInfo.mpcUser, "rpcAddr", nodeInfo.mpcRPCAddress)
		}
	}
	allInitiatorNodes = append(allInitiatorNodes, nodeInfo)
}

// IsSwapServer returns if this mpc user is the swap server
func IsSwapServer() bool {
	return len(allInitiatorNodes) > 0
}

// setMPCGroup set mpc group
func setMPCGroup(group string, mode, neededOracles, totalOracles uint32) {
	mpcGroupID = group
	mpcNeededOracles = neededOracles
	mpcTotalOracles = totalOracles
	mpcThreshold = fmt.Sprintf("%d/%d", neededOracles, totalOracles)
	mpcMode = fmt.Sprintf("%d", mode)
	log.Info("Init mpc group", "group", mpcGroupID, "threshold", mpcThreshold, "mode", mpcMode)
}

// GetGroupID return mpc group id
func GetGroupID() string {
	return mpcGroupID
}

// GetSelfEnode get self enode
func GetSelfEnode() string {
	return selfEnode
}

// GetAllEnodes get all enodes
func GetAllEnodes() []string {
	return allEnodes
}

// setMPCRPCAddress set mpc node rpc address
func (ni *NodeInfo) setMPCRPCAddress(url string) {
	ni.mpcRPCAddress = url
}

// GetMPCRPCAddress get mpc node rpc address
func (ni *NodeInfo) GetMPCRPCAddress() string {
	return ni.mpcRPCAddress
}

// setSignGroups set sign subgroups
func (ni *NodeInfo) setSignGroups(groups []string) {
	ni.signGroups = groups
}

// GetSignGroups get sign subgroups
func (ni *NodeInfo) GetSignGroups() []string {
	return ni.signGroups
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

func initSelfEnode() {
	for {
		enode, err := GetEnode(defaultMPCNode.mpcRPCAddress)
		if err == nil {
			selfEnode = enode
			log.Info("get mpc enode info success", "enode", enode)
			return
		}
		log.Error("can't get enode info", "rpcAddr", defaultMPCNode.mpcRPCAddress, "err", err)
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

func initAllEnodes() {
	allEnodes = verifySignGroupInfo(defaultMPCNode.mpcRPCAddress, mpcGroupID, false, true)
}

func verifySignGroupInfo(rpcAddr, groupID string, isSignGroup, includeSelf bool) []string {
	memberCount := mpcTotalOracles
	if isSignGroup {
		memberCount = mpcNeededOracles
	}
	for {
		groupInfo, err := GetGroupByID(groupID, rpcAddr)
		if err != nil {
			log.Error("get group info failed", "groupID", groupID, "err", err)
			time.Sleep(10 * time.Second)
			continue
		}
		log.Info("get mpc group info success", "groupInfo", groupInfo)
		if uint32(groupInfo.Count) != memberCount {
			log.Fatal("mpc group member count mismatch", "groupID", mpcGroupID, "have", groupInfo.Count, "want", memberCount)
		}
		if uint32(len(groupInfo.Enodes)) != memberCount {
			log.Fatal("get group info enodes count mismatch", "groupID", groupID, "have", len(groupInfo.Enodes), "want", memberCount)
		}
		exist := isEnodeExistIn(selfEnode, groupInfo.Enodes)
		if exist != includeSelf {
			log.Fatal("self enode's existence in group mismatch", "groupID", groupID, "groupInfo", groupInfo, "want", includeSelf, "have", exist)
		}
		if isSignGroup {
			for _, enode := range groupInfo.Enodes {
				if !isEnodeExistIn(enode, allEnodes) {
					log.Fatal("sign group has unrelated enode", "groupID", groupID, "enode", enode)
				}
			}
		}
		return groupInfo.Enodes
	}
}

func verifyInitiators(initiators []string) {
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
		for _, signGroupID := range mpcNodeInfo.GetSignGroups() {
			verifySignGroupInfo(mpcNodeInfo.mpcRPCAddress, signGroupID, true, isInGroup)
		}
		isInGroup = false
	}
}

func initMPCNodeInfo(mpcNodeCfg *params.MPCNodeConfig, isServer bool) *NodeInfo {
	mpcNodeInfo := &NodeInfo{}
	mpcNodeInfo.setMPCRPCAddress(*mpcNodeCfg.RPCAddress)
	log.Info("Init mpc rpc address", "rpcaddress", *mpcNodeCfg.RPCAddress)

	mpcUser, err := mpcNodeInfo.LoadKeyStore(*mpcNodeCfg.KeystoreFile, *mpcNodeCfg.PasswordFile)
	if err != nil {
		log.Fatalf("load keystore error %v", err)
	}
	log.Info("Init mpc, load keystore success", "user", mpcUser.String())

	if isServer {
		if !params.IsMPCInitiator(mpcUser.String()) {
			log.Fatalf("server mpc user %v is not in configed initiators", mpcUser.String())
		}

		signGroups := mpcNodeCfg.SignGroups
		log.Info("Init mpc sign groups", "signGroups", signGroups)
		mpcNodeInfo.setSignGroups(signGroups)
		addInitiatorNode(mpcNodeInfo)
	}

	return mpcNodeInfo
}
