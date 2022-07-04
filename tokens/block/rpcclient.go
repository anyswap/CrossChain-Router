package block

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// PostTransaction impl
func (b *Bridge) PostTransaction(txHex string) (txHash string, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	var success bool
	for _, ccli := range cli.CClients {
		msgtx := DecodeTxHex(txHex, 0, false)
		hash, err0 := ccli.SendRawTransaction(msgtx, true)
		if err0 == nil && !success {
			success = true
			txHash = hash.String()
		}
		errs = append(errs, err0)
	}
	if success {
		return txHash, nil
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// GetTransactionByHash get tx response by hash
func (b *Bridge) GetTransactionByHash(txHash string) (etx *ElectTx, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		return
	}
	for _, ccli := range cli.CClients {
		tx, err0 := ccli.GetRawTransactionVerbose(hash)
		if err0 == nil {
			etx = ConvertTx(tx)
			return
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// EstimateFeePerKb impl
func (b *Bridge) EstimateFeePerKb(blocks int) (fee int64, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	for _, ccli := range cli.CClients {
		res, err0 := ccli.Client.EstimateSmartFee(int64(blocks), &btcjson.EstimateModeEconomical)
		if err0 == nil {
			if len(res.Errors) > 0 {
				errs = append(errs, fmt.Errorf("%+v", res.Errors))
				continue
			}
			return int64(*res.FeeRate * 1e8), nil
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// FindUtxos impl
func (b *Bridge) FindUtxos(addr string) (utxos []*ElectUtxo, err error) {
	// cloudchainsinc
	cli := b.GetClient()

	currentHeight, err := b.GetLatestBlockNumber()
	if err != nil {
		return nil, err
	}

	errs := make([]error, 0)
	for _, url := range cli.UTXOAPIAddresses {
		res := struct {
			Utxos []CloudchainUtxo `json:"utxos"`
		}{}

		reqdata := fmt.Sprintf(`{ "version": 2.0, "id": "lalala", "method": "getutxos", "params": [ "BLOCK", "[\"%s\"]" ] }`, addr)
		err0 := callCloudchains(url, reqdata, &res)

		if err0 == nil {
			for _, cutxo := range res.Utxos {
				value := uint64(cutxo.Value * 1e8)

				status := &ElectTxStatus{
					BlockHeight: &cutxo.BlockNumber,
				}

				confirmed := false
				if currentHeight-cutxo.BlockNumber > 6 {
					confirmed = true
				}
				status.Confirmed = &confirmed

				if blkhash, err1 := b.GetBlockHash(cutxo.BlockNumber); err1 != nil {
					status.BlockHash = &blkhash
					if blk, err2 := b.GetBlock(blkhash); err2 != nil {
						status.BlockTime = new(uint64)
						*status.BlockTime = uint64(*blk.Timestamp)
					}
				}

				utxo := &ElectUtxo{
					Txid:   new(string),
					Vout:   new(uint32),
					Value:  new(uint64),
					Status: new(ElectTxStatus),
				}
				*utxo.Txid = cutxo.Txhash
				*utxo.Vout = cutxo.Vout
				*utxo.Value = value
				utxo.Status = status
				utxos = append(utxos, utxo)
			}
			sort.Sort(SortableElectUtxoSlice(utxos))
			return utxos, err
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return utxos, err
}

// GetElectTransactionStatus impl
func (b *Bridge) GetElectTransactionStatus(txHash string) (txstatus *ElectTxStatus, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		return
	}
	for _, ccli := range cli.CClients {
		txraw, err0 := ccli.GetRawTransactionVerbose(hash)
		if err0 == nil {
			txstatus = TxStatus(txraw)
			if h := txstatus.BlockHash; h != nil {
				if blk, err1 := b.GetBlock(*h); err1 == nil {
					*txstatus.BlockHeight = uint64(*blk.Height)
				}
			}
			return
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// GetLatestBlockNumber impl
func (b *Bridge) GetLatestBlockNumber() (blocknumber uint64, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	for _, ccli := range cli.CClients {
		number, err0 := ccli.GetBlockCount()
		if err0 == nil {
			return uint64(number), nil
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// callCloudchains
func callCloudchains(url, reqdata string, result interface{}) error {
	client := &http.Client{}
	var data = strings.NewReader(reqdata)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	err = json.Unmarshal(bodyText, &result)
	return err
}

// GetBlockHash impl
func (b *Bridge) GetBlockHash(height uint64) (hash string, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	for _, ccli := range cli.CClients {
		bh, err0 := ccli.GetBlockHash(int64(height))
		if err0 == nil {
			hash = bh.String()
			return
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}

// GetBlock impl
func (b *Bridge) GetBlock(blockHash string) (eblock *ElectBlock, err error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	errs := make([]error, 0)
	hash, err := chainhash.NewHashFromStr(blockHash)
	if err != nil {
		return
	}
	for _, ccli := range cli.CClients {
		block, err0 := ccli.GetBlockVerbose(hash)
		if err0 == nil {
			eblock = ConvertBlock(block)
			return eblock, nil
		}
		errs = append(errs, err0)
	}
	err = fmt.Errorf("%+v", errs)
	return
}
