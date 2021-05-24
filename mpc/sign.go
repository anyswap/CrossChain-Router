package mpc

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/tools/crypto"
	"github.com/anyswap/CrossChain-Router/tools/keystore"
	"github.com/anyswap/CrossChain-Router/tools/rlp"
	"github.com/anyswap/CrossChain-Router/types"
)

const (
	pingCount                  = 3
	retrySignCount             = 3
	retryGetSignStatusCount    = 70
	retryGetSignStatusInterval = 10 * time.Second
)

func pingMPCNode(nodeInfo *NodeInfo) (err error) {
	rpcAddr := nodeInfo.mpcRPCAddress
	for j := 0; j < pingCount; j++ {
		_, err = GetEnode(rpcAddr)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	log.Error("pingMPCNode failed", "rpcAddr", rpcAddr, "pingCount", pingCount, "err", err)
	return err
}

// DoSignOne mpc sign single msgHash with context msgContext
func DoSignOne(signPubkey, msgHash, msgContext string) (keyID string, rsvs []string, err error) {
	return DoSign(signPubkey, []string{msgHash}, []string{msgContext})
}

// DoSign mpc sign msgHash with context msgContext
func DoSign(signPubkey string, msgHash, msgContext []string) (keyID string, rsvs []string, err error) {
	log.Debug("mpc DoSign", "msgHash", msgHash, "msgContext", msgContext)
	if signPubkey == "" {
		return "", nil, errors.New("mpc sign with empty public key")
	}
	var pingOk bool
	for retry := 0; retry < retrySignCount; retry++ {
		for _, mpcNode := range allInitiatorNodes {
			if err = pingMPCNode(mpcNode); err != nil {
				continue
			}
			pingOk = true
			signGroupsCount := int64(len(mpcNode.signGroups))
			// randomly pick first subgroup to sign
			randIndex, _ := rand.Int(rand.Reader, big.NewInt(signGroupsCount))
			startIndex := randIndex.Int64()
			i := startIndex
			for {
				keyID, rsvs, err = doSignImpl(mpcNode, i, signPubkey, msgHash, msgContext)
				if err == nil {
					return keyID, rsvs, nil
				}
				i = (i + 1) % signGroupsCount
				if i == startIndex {
					break
				}
			}
		}
	}
	if !pingOk {
		err = errors.New("mpc sign ping mpc node failed")
	}
	return "", nil, err
}

func doSignImpl(mpcNode *NodeInfo, signGroupIndex int64, signPubkey string, msgHash, msgContext []string) (keyID string, rsvs []string, err error) {
	nonce, err := GetSignNonce(mpcNode.mpcUser.String(), mpcNode.mpcRPCAddress)
	if err != nil {
		return "", nil, err
	}
	txdata := SignData{
		TxType:     "SIGN",
		PubKey:     signPubkey,
		MsgHash:    msgHash,
		MsgContext: msgContext,
		Keytype:    "ECDSA",
		GroupID:    mpcNode.signGroups[signGroupIndex],
		ThresHold:  mpcThreshold,
		Mode:       mpcMode,
		TimeStamp:  common.NowMilliStr(),
	}
	payload, _ := json.Marshal(txdata)
	rawTX, err := BuildMPCRawTx(nonce, payload, mpcNode.keyWrapper)
	if err != nil {
		return "", nil, err
	}

	rpcAddr := mpcNode.mpcRPCAddress
	keyID, err = Sign(rawTX, rpcAddr)
	if err != nil {
		return "", nil, err
	}

	time.Sleep(retryGetSignStatusInterval)
	var signStatus *SignStatus
	i := 0
	for ; i < retryGetSignStatusCount; i++ {
		signStatus, err = GetSignStatus(keyID, rpcAddr)
		if err == nil {
			rsvs = signStatus.Rsv
			break
		}
		switch {
		case errors.Is(err, ErrGetSignStatusFailed),
			errors.Is(err, ErrGetSignStatusTimeout):
			return "", nil, err
		}
		log.Warn("retry get sign status as error", "keyID", keyID, "err", err)
		time.Sleep(retryGetSignStatusInterval)
	}
	if i == retryGetSignStatusCount || len(rsvs) == 0 {
		return "", nil, errors.New("get sign status failed")
	}

	return keyID, rsvs, err
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
