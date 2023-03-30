package flow

import (
	"github.com/onflow/cadence"
)

type SwapIn struct {
	Tx           cadence.String  `json:"tx"`
	Token        cadence.String  `json:"token"`
	Receiver     cadence.Address `json:"Receiver"`
	FromChainId  cadence.UInt64  `json:"fromChainId"`
	Amount       cadence.UFix64  `json:"amount"`
	ReceivePaths cadence.Array   `json:"receivePaths"`
}
