package btc

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
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
	chainConfig := b.GetChainParams(b.ChainConfig.GetChainID())
	address, err = btcutil.DecodeAddress(addr, chainConfig)
	if err != nil {
		return
	}
	if !address.IsForNet(chainConfig) {
		err = fmt.Errorf("invalid address for net")
		return
	}
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

func (b *Bridge) FindUtxos(addr string) (result []*ElectUtxo, err error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err = FindUtxos(url, addr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return result, nil
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

// IsPayToScriptHash is p2sh
func (b *Bridge) IsPayToScriptHash(sigScript []byte) bool {
	return txscript.IsPayToScriptHash(sigScript)
}

func (b *Bridge) getRedeemScriptByOutputScrpit(preScript []byte) ([]byte, error) {
	// pkScript, err := b.ParsePkScript(preScript)
	// if err != nil {
	// return nil, err
	// }
	// p2shAddress, err := pkScript.Address(b.Inherit.GetChainParams())
	// if err != nil {
	// return nil, err
	// }
	// p2shAddr := p2shAddress.String()
	p2shAddr := ""
	bindAddr := GetP2shBindAddress(p2shAddr)
	if bindAddr == "" {
		return nil, fmt.Errorf("p2sh address %v is not registered", p2shAddr)
	}
	var address string
	address, redeemScript, _ := b.GetP2shAddress(bindAddr)
	if address != p2shAddr {
		return nil, fmt.Errorf("p2sh address mismatch for bind address %v, have %v want %v", bindAddr, p2shAddr, address)
	}
	return redeemScript, nil
}

// ParsePkScript parse pkScript
func (b *Bridge) ParsePkScript(pkScript []byte) (txscript.PkScript, error) {
	return txscript.ParsePkScript(pkScript)
}

// GetP2shBindAddress get p2sh bind address
func GetP2shBindAddress(p2shAddress string) (bindAddress string) {
	return ""
}

func (b *Bridge) GetP2shAddress(bindAddr string) (p2shAddress string, redeemScript []byte, err error) {
	if b.IsValidAddress(bindAddr) {
		return "", nil, fmt.Errorf("invalid bind address %v", bindAddr)
	}
	memo := common.FromHex(bindAddr)

	dcrmAddress, err := b.GetMPCAddress()
	if err != nil {
		return "", nil, fmt.Errorf("invalid dcrm address %v, %w", dcrmAddress, err)
	}
	address, err := b.DecodeAddress(dcrmAddress)
	if err != nil {
		return "", nil, fmt.Errorf("invalid dcrm address %v, %w", dcrmAddress, err)
	}
	pubKeyHash := address.ScriptAddress()
	return b.getP2shAddressWithMemo(memo, pubKeyHash)
}

func (b *Bridge) getP2shAddressWithMemo(memo, pubKeyHash []byte) (p2shAddress string, redeemScript []byte, err error) {
	redeemScript, err = b.GetP2shRedeemScript(memo, pubKeyHash)
	if err != nil {
		return
	}
	addressScriptHash, err := b.NewAddressScriptHash(redeemScript)
	if err != nil {
		return
	}
	p2shAddress = addressScriptHash.EncodeAddress()
	return
}

func (b *Bridge) GetP2shRedeemScript(memo, pubKeyHash []byte) (redeemScript []byte, err error) {
	return txscript.NewScriptBuilder().
		AddData(memo).AddOp(txscript.OP_DROP).
		AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).AddData(pubKeyHash).
		AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).
		Script()
}

// NewAddressScriptHash encap
func (b *Bridge) NewAddressScriptHash(redeemScript []byte) (*btcutil.AddressScriptHash, error) {
	// return btcutil.NewAddressScriptHash(redeemScript, b.Inherit.GetChainParams())
	return nil, tokens.ErrNotImplemented
}

// CalcSignatureHash calc sig hash
func (b *Bridge) CalcSignatureHash(sigScript []byte, tx *wire.MsgTx, i int) (sigHash []byte, err error) {
	return txscript.CalcSignatureHash(sigScript, txscript.SigHashAll, tx, i)
}

func (b *Bridge) getSigDataFromRSV(rsv string) ([]byte, bool) {
	rs := rsv[0 : len(rsv)-2]

	r := rs[:64]
	s := rs[64:]

	rr, ok := new(big.Int).SetString(r, 16)
	if !ok {
		return nil, false
	}

	ss, ok := new(big.Int).SetString(s, 16)
	if !ok {
		return nil, false
	}

	return b.SerializeSignature(rr, ss), true
}

func (b *Bridge) SerializeSignature(r, s *big.Int) []byte {
	sign := &btcec.Signature{R: r, S: s}
	return append(sign.Serialize(), byte(txscript.SigHashAll))
}

// GetSigScript get script
func (b *Bridge) GetSigScript(sigScripts [][]byte, prevScript, signData, cPkData []byte, i int) (sigScript []byte, err error) {
	scriptClass := txscript.GetScriptClass(prevScript)
	switch scriptClass {
	case txscript.PubKeyHashTy:
		sigScript, err = txscript.NewScriptBuilder().AddData(signData).AddData(cPkData).Script()
	case txscript.ScriptHashTy:
		if sigScripts == nil {
			err = fmt.Errorf("call MakeSignedTransaction spend p2sh without redeem scripts")
		} else {
			redeemScript := sigScripts[i]
			err = b.VerifyRedeemScript(prevScript, redeemScript)
			if err == nil {
				sigScript, err = txscript.NewScriptBuilder().AddData(signData).AddData(cPkData).AddData(redeemScript).Script()
			}
		}
	default:
		err = fmt.Errorf("unsupport to spend '%v' output", scriptClass.String())
	}
	return sigScript, err
}

// VerifyRedeemScript verify redeem script
func (b *Bridge) VerifyRedeemScript(prevScript, redeemScript []byte) error {
	p2shScript, err := b.GetP2shSigScript(redeemScript)
	if err != nil {
		return err
	}
	if !bytes.Equal(p2shScript, prevScript) {
		return fmt.Errorf("redeem script %x mismatch", redeemScript)
	}
	return nil
}

// GetP2shSigScript get p2sh signature script
func (b *Bridge) GetP2shSigScript(redeemScript []byte) ([]byte, error) {
	p2shAddr, err := b.GetP2shAddressByRedeemScript(redeemScript)
	if err != nil {
		return nil, err
	}
	return b.GetPayToAddrScript(p2shAddr)
}

// GetP2shAddressByRedeemScript get p2sh address by redeem script
func (b *Bridge) GetP2shAddressByRedeemScript(redeemScript []byte) (string, error) {
	addressScriptHash, err := b.NewAddressScriptHash(redeemScript)
	if err != nil {
		return "", err
	}
	return addressScriptHash.EncodeAddress(), nil
}

// GetCompressedPublicKey get compressed public key
func (b *Bridge) GetCompressedPublicKey(fromPublicKey string, needVerify bool) (cPkData []byte, err error) {
	if fromPublicKey == "" {
		return nil, nil
	}
	pkData := common.FromHex(fromPublicKey)
	cPkData, err = b.ToCompressedPublicKey(pkData)
	if err != nil {
		return nil, err
	}
	if needVerify {
		err = b.verifyPublickeyData(cPkData)
		if err != nil {
			return nil, err
		}
	}
	return cPkData, nil
}

// ToCompressedPublicKey convert to compressed public key if not
func (b *Bridge) ToCompressedPublicKey(pkData []byte) ([]byte, error) {
	pubKey, err := btcec.ParsePubKey(pkData, btcec.S256())
	if err != nil {
		return nil, err
	}
	return pubKey.SerializeCompressed(), nil
}

func (b *Bridge) verifyPublickeyData(pkData []byte) error {
	dcrmAddress, err := b.GetMPCAddress()
	if err != nil {
		return err
	}
	if dcrmAddress == "" {
		return nil
	}
	address, err := b.NewAddressPubKeyHash(pkData)
	if err != nil {
		return err
	}
	if address.EncodeAddress() != dcrmAddress {
		return fmt.Errorf("public key address %v is not the configed dcrm address %v", address, dcrmAddress)
	}
	return nil
}

// NewAddressPubKeyHash encap
func (b *Bridge) NewAddressPubKeyHash(pkData []byte) (*btcutil.AddressPubKeyHash, error) {
	// return btcutil.NewAddressPubKeyHash(btcutil.Hash160(pkData), b.Inherit.GetChainParams())
	return nil, tokens.ErrNotImplemented
}

// the rsv must have correct v (recovery id), otherwise will get wrong public key data.
func (b *Bridge) getPkDataFromSig(rsv, msgHash string, compressed bool) (pkData []byte, err error) {
	rsvData := common.FromHex(rsv)
	hashData := common.FromHex(msgHash)
	ecPub, err := crypto.SigToPub(hashData, rsvData)
	if err != nil {
		return nil, err
	}
	return b.SerializePublicKey(ecPub, compressed), nil
}

// SerializePublicKey serialize ecdsa public key
func (b *Bridge) SerializePublicKey(ecPub *ecdsa.PublicKey, compressed bool) []byte {
	if compressed {
		return (*btcec.PublicKey)(ecPub).SerializeCompressed()
	}
	return (*btcec.PublicKey)(ecPub).SerializeUncompressed()
}

// DecodeWIF decode wif
func DecodeWIF(wif string) (*btcutil.WIF, error) {
	return btcutil.DecodeWIF(wif)
}
