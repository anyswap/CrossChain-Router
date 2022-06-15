package btc

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// GetPayToAddrScript get pay to address script
func (b *Bridge) GetPayToAddrScript(address string) ([]byte, error) {
	toAddr, err := b.DecodeAddress(address)
	if err != nil {
		return nil, fmt.Errorf("decode btc address '%v' failed. %w", address, err)
	}
	return txscript.PayToAddrScript(toAddr)
}

// DecodeAddress decode address
func (b *Bridge) DecodeAddress(addr string) (address btcutil.Address, err error) {
	// chainConfig := b.Inherit.GetChainParams()
	// address, err = btcutil.DecodeAddress(addr, chainConfig)
	// if err != nil {
	// 	return
	// }
	// if !address.IsForNet(chainConfig) {
	// 	err = fmt.Errorf("invalid address for net")
	// 	return
	// }
	return
}

// NullDataScript encap
func (b *Bridge) NullDataScript(memo string) ([]byte, error) {
	return txscript.NullDataScript([]byte(memo))
}

// NewTxOut new txout
func (b *Bridge) NewTxOut(amount int64, pkScript []byte) *wire.TxOut {
	return wire.NewTxOut(amount, pkScript)
}

func (b *Bridge) FindUtxos(addr string) ([]*ElectUtxo, error) {
	return b.FindUtxos(addr)
}

// EstimateFeePerKb impl
func (b *Bridge) EstimateFeePerKb(blocks int) (result int64, err error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err = EstimateFeePerKb(url, blocks)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, err
	}
	return result, nil
}

// NewTxIn new txin
func (b *Bridge) NewTxIn(txid string, vout uint32, pkScript []byte) (*wire.TxIn, error) {
	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, err
	}
	prevOutPoint := wire.NewOutPoint(txHash, vout)
	return wire.NewTxIn(prevOutPoint, pkScript, nil), nil
}

func isValidValue(value btcAmountType) bool {
	return value > 0 && value <= btcutil.MaxSatoshi
}
