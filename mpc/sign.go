package mpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/tools/crypto"
	"github.com/anyswap/CrossChain-Router/tools/keystore"
	"github.com/anyswap/CrossChain-Router/tools/rlp"
	"github.com/anyswap/CrossChain-Router/types"
)

func getMPCNode() *NodeInfo {
	countOfInitiators := len(allInitiatorNodes)
	if countOfInitiators < 2 {
		return defaultMPCNode
	}
	i, pingCount := 0, 3
	for {
		nodeInfo := allInitiatorNodes[i]
		rpcAddr := nodeInfo.mpcRPCAddress
		for j := 0; j < pingCount; j++ {
			_, err := GetEnode(rpcAddr)
			if err == nil {
				return nodeInfo
			}
			log.Error("GetEnode of initiator failed", "rpcAddr", rpcAddr, "times", j+1, "err", err)
			time.Sleep(1 * time.Second)
		}
		i = (i + 1) % countOfInitiators
		if i == 0 {
			log.Error("GetEnode of initiator failed all")
			time.Sleep(60 * time.Second)
		}
	}
}

// DoSignOne mpc sign single msgHash with context msgContext
func DoSignOne(signPubkey, msgHash, msgContext string) (rpcAddr, result string, err error) {
	return DoSign(signPubkey, []string{msgHash}, []string{msgContext})
}

// DoSign mpc sign msgHash with context msgContext
func DoSign(signPubkey string, msgHash, msgContext []string) (rpcAddr, result string, err error) {
	log.Debug("mpc DoSign", "msgHash", msgHash, "msgContext", msgContext)
	if signPubkey == "" {
		return "", "", fmt.Errorf("mpc sign with empty public key")
	}
	mpcNode := getMPCNode()
	if mpcNode == nil {
		return "", "", fmt.Errorf("mpc sign with nil node info")
	}
	nonce, err := GetSignNonce(mpcNode.mpcUser.String(), mpcNode.mpcRPCAddress)
	if err != nil {
		return "", "", err
	}
	// randomly pick sub-group to sign
	signGroups := mpcNode.signGroups
	randIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(signGroups))))
	signGroup := signGroups[randIndex.Int64()]
	txdata := SignData{
		TxType:     "SIGN",
		PubKey:     signPubkey,
		MsgHash:    msgHash,
		MsgContext: msgContext,
		Keytype:    "ECDSA",
		GroupID:    signGroup,
		ThresHold:  mpcThreshold,
		Mode:       mpcMode,
		TimeStamp:  common.NowMilliStr(),
	}
	payload, _ := json.Marshal(txdata)
	rawTX, err := BuildMPCRawTx(nonce, payload, mpcNode.keyWrapper)
	if err != nil {
		return "", "", err
	}
	rpcAddr = mpcNode.mpcRPCAddress
	result, err = Sign(rawTX, rpcAddr)
	return rpcAddr, result, err
}

// BuildMPCRawTx build mpc raw tx
func BuildMPCRawTx(nonce uint64, payload []byte, keyWrapper *keystore.Key) (string, error) {
	tx := types.NewTransaction(
		nonce,             // nonce
		mpcToAddr,         // to address
		big.NewInt(0),     // value
		100000,            // gasLimit
		big.NewInt(80000), // gasPrice
		payload,           // data
	)
	signature, err := crypto.Sign(mpcSigner.Hash(tx).Bytes(), keyWrapper.PrivateKey)
	if err != nil {
		return "", err
	}
	sigTx, err := tx.WithSignature(mpcSigner, signature)
	if err != nil {
		return "", err
	}
	txdata, err := rlp.EncodeToBytes(sigTx)
	if err != nil {
		return "", err
	}
	rawTX := common.ToHex(txdata)
	return rawTX, nil
}
