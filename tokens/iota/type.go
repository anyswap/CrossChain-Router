package iota

type Address struct {
	Type    uint64 `json:"type"`
	Address string `json:"address"`
}
type RawType struct {
	Type    uint64  `json:"type"`
	Address Address `json:"address"`
	Amount  uint64  `json:"amount"`
}
