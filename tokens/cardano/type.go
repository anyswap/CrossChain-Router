package cardano

type Tip struct {
	Block        uint64 `json:"block"`
	Epoch        uint64 `json:"epoch"`
	Era          string `json:"era"`
	Hash         string `json:"hash"`
	Slot         uint64 `json:"slot"`
	SyncProgress string `json:"syncProgress"`
}

type TransactionResult struct {
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	Block         Block      `json:"block"`
	Hash          string     `json:"hash"`
	Metadata      []Metadata `json:"metadata"`
	Inputs        []Input    `json:"inputs"`
	Outputs       []Output   `json:"outputs"`
	ValidContract bool       `json:"validContract"`
}

type Output struct {
	Address string  `json:"address"`
	Tokens  []Token `json:"tokens"`
	Value   string  `json:"value"`
}

type Input struct {
	Tokens []Token `json:"tokens"`
	Value  string  `json:"value"`
}

type Token struct {
	Asset    Asset  `json:"asset"`
	Quantity string `json:"quantity"`
}

type Asset struct {
	AssetId   string `json:"assetId"`
	AssetName string `json:"assetName"`
}

type Block struct {
	EpochNo uint64 `json:"epochNo"`
	Number  uint64 `json:"number"`
}

type Metadata struct {
	Key   string        `json:"key"`
	Value MetadataValue `json:"value"`
}

type MetadataValue struct {
	Bind      string `json:"bind"`
	ToChainId uint64 `json:"toChainId"`
}

type UtxoMap struct {
	Index  string            `json:"index"`
	Assets map[string]string `json:"assets"`
}

type RawTransaction struct {
	Fee     string                       `json:"fee"`
	TxInts  map[string]string            `json:"txInts"`
	TxOuts  map[string]map[string]string `json:"txOuts"`
	Mint    map[string]string            `json:"mint"`
	OutFile string                       `json:"outFile"`
}
