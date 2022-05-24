package reef

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/types"

	"github.com/itering/scale.go"
	scaletypes "github.com/itering/scale.go/types"
	"github.com/itering/scale.go/utiles"
	"github.com/itering/scale.go/source"
	substratetypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

type BlockResult struct {
	*Block `json:"block"`
	Justifications interface{} `json:"justifications"`
}

type Block struct {
	*Header `json:"header"`
	Extrinsics []string `json:"extrinsics"`
}

type Header struct {
	Digest Digest `json:"digest"`
	ExtrinsicsRoot string `json:"extrinsicsRoot"`
	Number string `json:"number"`
	ParentHash string `json:"parentHash"`
	StateRoot string `json:"stateRoot"`
}

type Digest struct {
	Logs []string `json:"logs"`
}

type ExtrinsicInfo struct {
	ExtrinsicLength int
	ExtrinsicHash string
	VersionInfo string
	ContainTransaction bool
	Nonce int
	Era string
	CallIndex string
	Params *ExtParams
	Signature string
	SignedExtensions interface{}
}

type ExtParams struct {
	Address string
	Input []byte
	Value *big.Int
	GasLimit uint64
	StorageLimit uint32
}

const (
	EventID_ExtrinsicSuccess string = "ExtrinsicSuccess"
	Phase_ApplyExtrinsic = 0
)

type ExtrinsicResult struct {
	*ExtrinsicInfo
	EvmLogs []*types.RPCLog
}

func (b *Bridge) MakeExtrinsicResult(blockRes *BlockResult, extIdx int, events string) (res *ExtrinsicResult, err error) {
	defer func() { if r := recover(); r != nil {
		err = fmt.Errorf("%v", r)
	} }()

	if len(blockRes.Extrinsics) < extIdx + 1 {
		return nil, errors.New("extrinsic index overflow")
	}
	m := scalecodec.MetadataDecoder{}
	m.Init(utiles.HexToBytes(*b.GetMetadata()))
	_ = m.Process()
	c, err := ioutil.ReadFile(fmt.Sprintf("reef.json"))
	if err != nil {
		return nil, err
	}

	res = new(ExtrinsicResult)

	scaletypes.RegCustomTypes(source.LoadTypeRegistry(c))
	e := scalecodec.ExtrinsicDecoder{}
	option := scaletypes.ScaleDecoderOption{Metadata: &m.Metadata, Spec: 9111}
	extrinsicRaw := blockRes.Extrinsics[extIdx]
	e.Init(scaletypes.ScaleBytes{Data: utiles.HexToBytes(extrinsicRaw)}, &option)
	e.Process()

	res.ExtrinsicLength = e.ExtrinsicLength
	res.ExtrinsicHash = e.ExtrinsicHash
	res.VersionInfo = e.VersionInfo
	res.ContainTransaction = e.ContainsTransaction
	res.Era = e.Era
	res.Nonce = e.Nonce
	res.CallIndex = e.CallIndex
	res.Signature = e.Signature
	res.SignedExtensions = e.SignedExtensions
	res.Params = new(ExtParams)
	res.EvmLogs = make([]*types.RPCLog, 0)

	for _, p := range e.Params {
		switch p.Name {
		case "target":
			res.Params.Address, _ = p.Value.(string)
		case "input":
			val, _ := p.Value.(string)
			res.Params.Input, _ = hex.DecodeString(val)
		case "value":
			val, _ := p.Value.(uint64)
			res.Params.Value = new(big.Int).SetUint64(val)
		case "gas_limit":
			val, _ := p.Value.(uint64)
			res.Params.GasLimit = val
		case "storage_limit":
			val, _ := p.Value.(uint32)
			res.Params.StorageLimit = val
		default:
			continue
		}
	}

	e2 := scalecodec.EventsDecoder{}
	e2.Init(scaletypes.ScaleBytes{Data: utiles.HexToBytes(events)}, &option)
	e2.Process()
	evts, _ := e2.Vec.Value.([]interface{})
	for _, event := range evts {
		evt, ok := event.(map[string]interface{})
		if !ok { continue }
		if evt["event_idx"] == extIdx && evt["module_id"] == "EVM" {
			evtParams, ok := evt["params"].([]scalecodec.EventParam)
			if !ok { continue }
			for _, evtParam := range evtParams {
				if evtParam.Type == "Log" {
					rpclog := &types.RPCLog{
						Address: new(common.Address),
						Topics: []common.Hash{},
						Data: new(hexutil.Bytes),
						Removed: new(bool),
					}
					evmlog, ok := evtParam.Value.(map[string]interface{})
					if !ok { continue }
					addrStr, _ := evmlog["address"].(string)
					address := common.HexToAddress(addrStr)
					*rpclog.Address = address
					topics, _ := evmlog["topics"].([]interface{})
					for _, topic := range topics {
						topicStr, _ := topic.(string)
						rpclog.Topics = append(rpclog.Topics, common.HexToHash(topicStr))
					}
					dataStr, _ := evmlog["data"].(string)
					*rpclog.Data = utiles.HexToBytes(dataStr)
					*rpclog.Removed = false
					res.EvmLogs = append(res.EvmLogs, (rpclog))
				}
			}
		}
	}
	return res, nil
}


type Extrinsic struct {
	*substratetypes.Extrinsic
	*substratetypes.SignatureOptions
}
