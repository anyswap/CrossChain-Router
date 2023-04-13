package iota

import iotago "github.com/iotaledger/iota.go/v2"

type Address struct {
	Type    uint64 `json:"type"`
	Address string `json:"address"`
}

type RawType struct {
	Type    uint64  `json:"type"`
	Address Address `json:"address"`
	Amount  uint64  `json:"amount"`
}

type MessagePayload struct {
	Type         uint64        `json:"type"`
	Essence      Essence       `json:"essence"`
	UnlockBlocks []UnlockBlock `json:"unlockBlocks"`
}

type Essence struct {
	Type    uint64   `json:"type"`
	Inputs  []Input  `json:"inputs"`
	Outputs []Output `json:"outputs"`
	Payload Payload  `json:"payload"`
}

type UnlockBlock struct {
	Type      uint64    `json:"type"`
	Signature Signature `json:"signature"`
}

type Signature struct {
	Type      uint64 `json:"type"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}

type Input struct {
	Type                   uint64 `json:"type"`
	TransactionId          string `json:"transactionId"`
	TransactionOutputIndex uint64 `json:"transactionOutputIndex"`
}

type Output struct {
	Type    uint64  `json:"type"`
	Address Address `json:"address"`
	Amount  uint64  `json:"amount"`
}

type Payload struct {
	Type  uint64 `json:"type"`
	Index string `json:"index"`
	Data  string `json:"data"`
}

type MessageBuilder struct {
	TransactionBuilder *iotago.TransactionBuilder `json:"transactionBuilder"`
	Essence            *iotago.TransactionEssence `json:"essence"`
}
