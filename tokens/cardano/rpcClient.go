package cardano

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/blockfrost/blockfrost-go"
	cardanosdk "github.com/echovl/cardano-go"
)

const (
	rpcTimeout     = 60
	CardanoMainNet = "https://cardano-mainnet.blockfrost.io/api/v0"
	CardanoTestNet = "https://cardano-testnet.blockfrost.io/api/v0"
	CardanoPreProd = "https://cardano-preprod.blockfrost.io/api/v0"
	CardanoPreview = "https://cardano-preview.blockfrost.io/api/v0"
	IPFSNet        = "https://ipfs.blockfrost.io/api/v0"
)

func GetTransactionByHash(url, txHash string) (*Transaction, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryTransaction, txHash)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	if len(result.Transactions) == 0 {
		return nil, tokens.ErrTxNotFound
	}
	return &result.Transactions[0], nil
}

func GetUtxosByAddress(url, address string) (*[]Output, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryOutputs, address)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result OutputsResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	if len(result.Outputs) == 0 {
		return nil, tokens.ErrOutputLength
	}
	return &result.Outputs, nil
}

func GetCardanoTip(url string) (*TipResponse, error) {
	request := &client.Request{}
	request.Params = QueryTIPAndProtocolParams
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TipResponse
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// func queryTipCmd() (*Tip, error) {
// 	if execRes, err := ExecCmd(QueryTipCmd, " "); err != nil {
// 		return nil, err
// 	} else {
// 		var tip Tip
// 		if err := json.Unmarshal([]byte(execRes), &tip); err != nil {
// 			return nil, err
// 		}
// 		return &tip, nil
// 	}
// }

// BlockfrostNode implements Node using the blockfrost API.
type BlockfrostNode struct {
	client    blockfrost.APIClient
	projectID string
	network   cardanosdk.Network
	url       string
}

// NewNode returns a new instance of BlockfrostNode.
func NewNode(network cardanosdk.Network, networkUrl string, projectID string) cardanosdk.Node {
	return &BlockfrostNode{
		network:   network,
		url:       networkUrl,
		projectID: projectID,
		client: blockfrost.NewAPIClient(blockfrost.APIClientOptions{
			ProjectID: projectID,
			Server:    networkUrl,
		}),
	}
}

func (b *BlockfrostNode) UTxOs(addr cardanosdk.Address) ([]cardanosdk.UTxO, error) {
	butxos, err := b.client.AddressUTXOs(context.Background(), addr.Bech32(), blockfrost.APIQueryParams{})
	if err != nil {
		// Addresses without UTXOs return NotFound error
		if err, ok := err.(*blockfrost.APIError); ok {
			if _, ok := err.Response.(blockfrost.NotFound); ok {
				return []cardanosdk.UTxO{}, nil
			}
		}
		return nil, err
	}

	utxos := make([]cardanosdk.UTxO, len(butxos))

	for i, butxo := range butxos {
		txHash, err := cardanosdk.NewHash32(butxo.TxHash)
		if err != nil {
			return nil, err
		}

		amount := cardanosdk.NewValue(0)
		for _, a := range butxo.Amount {
			if a.Unit == "lovelace" {
				lovelace, err := strconv.ParseUint(a.Quantity, 10, 64)
				if err != nil {
					return nil, err
				}
				amount.Coin += cardanosdk.Coin(lovelace)
			} else {
				unitBytes, err := hex.DecodeString(a.Unit)
				if err != nil {
					return nil, err
				}
				policyID := cardanosdk.NewPolicyIDFromHash(unitBytes[:28])
				assetName := string(unitBytes[28:])
				assetValue, err := strconv.ParseUint(a.Quantity, 10, 64)
				if err != nil {
					return nil, err
				}
				currentAssets := amount.MultiAsset.Get(policyID)
				if currentAssets != nil {
					currentAssets.Set(
						cardanosdk.NewAssetName(assetName),
						cardanosdk.BigNum(assetValue),
					)
				} else {
					amount.MultiAsset.Set(
						policyID,
						cardanosdk.NewAssets().
							Set(
								cardanosdk.NewAssetName(string(assetName)),
								cardanosdk.BigNum(assetValue),
							),
					)
				}
			}
		}

		utxos[i] = cardanosdk.UTxO{
			Spender: addr,
			TxHash:  txHash,
			Amount:  amount,
			Index:   uint64(butxo.OutputIndex),
		}
	}

	return utxos, nil
}

func (b *BlockfrostNode) Tip() (*cardanosdk.NodeTip, error) {
	block, err := b.client.BlockLatest(context.Background())
	if err != nil {
		return nil, err
	}

	return &cardanosdk.NodeTip{
		Block: uint64(block.Height),
		Epoch: uint64(block.Epoch),
		Slot:  uint64(block.Slot),
	}, nil
}

func (b *BlockfrostNode) SubmitTx(tx *cardanosdk.Tx) (*cardanosdk.Hash32, error) {
	url := fmt.Sprintf("%s/tx/submit", b.url)
	txBytes := tx.Bytes()

	req, err := http.NewRequest("POST", url, bytes.NewReader(txBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Add("project_id", b.projectID)
	req.Header.Add("Content-Type", "application/cbor")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(respBody))
	}

	txHash, err := tx.Hash()
	if err != nil {
		return nil, err
	}

	return &txHash, nil
}

func (b *BlockfrostNode) ProtocolParams() (*cardanosdk.ProtocolParams, error) {
	eparams, err := b.client.LatestEpochParameters(context.Background())
	if err != nil {
		return nil, err
	}

	minUTXO, err := strconv.ParseUint(eparams.MinUtxo, 10, 64)
	if err != nil {
		return nil, err
	}

	poolDeposit, err := strconv.ParseUint(eparams.PoolDeposit, 10, 64)
	if err != nil {
		return nil, err
	}
	keyDeposit, err := strconv.ParseUint(eparams.KeyDeposit, 10, 64)
	if err != nil {
		return nil, err
	}

	pparams := &cardanosdk.ProtocolParams{
		MinFeeA:            cardanosdk.Coin(eparams.MinFeeA),
		MinFeeB:            cardanosdk.Coin(eparams.MinFeeB),
		MaxBlockBodySize:   uint(eparams.MaxBlockSize),
		MaxTxSize:          uint(eparams.MaxTxSize),
		MaxBlockHeaderSize: uint(eparams.MaxBlockHeaderSize),
		KeyDeposit:         cardanosdk.Coin(keyDeposit),
		PoolDeposit:        cardanosdk.Coin(poolDeposit),
		MaxEpoch:           uint(eparams.Epoch),
		NOpt:               uint(eparams.NOpt),
		CoinsPerUTXOWord:   cardanosdk.Coin(minUTXO),
	}

	return pparams, nil
}

func (b *BlockfrostNode) Network() cardanosdk.Network {
	return b.network
}

func (b *BlockfrostNode) GetTransactionByHash(txHash string) (*blockfrost.TransactionContent, error) {
	tx, err := b.client.Transaction(context.Background(), txHash)
	return &tx, err
}

func (b *BlockfrostNode) GetTransactionMetadataByHash(txHash string) (*[]blockfrost.TransactionMetadata, error) {
	txm, err := b.client.TransactionMetadata(context.Background(), txHash)
	return &txm, err
}

func (b *BlockfrostNode) GetTransactionUtxoByHash(txHash string) (*blockfrost.TransactionUTXOs, error) {
	utxos, err := b.client.TransactionUTXOs(context.Background(), txHash)
	return &utxos, err
}
