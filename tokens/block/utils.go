package block

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
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
	pkScript, err := b.ParsePkScript(preScript)
	if err != nil {
		return nil, err
	}
	chainParams := b.GetChainParams(b.ChainConfig.GetChainID())
	p2shAddress, err := pkScript.Address(chainParams)
	if err != nil {
		return nil, err
	}
	p2shAddr := p2shAddress.String()
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

	mpcAddress := b.GetChainConfig().RouterContract // in btc routerMPC is routerContract

	address, err := b.DecodeAddress(mpcAddress)
	if err != nil {
		return "", nil, fmt.Errorf("invalid mpc address %v, %w", mpcAddress, err)
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
	chainParams := b.GetChainParams(b.ChainConfig.GetChainID())
	return btcutil.NewAddressScriptHash(redeemScript, chainParams)
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
	mpcAddress := b.GetChainConfig().RouterContract // in btc routerMPC is routerContract
	if mpcAddress == "" {
		return nil
	}
	address, err := b.NewAddressPubKeyHash(pkData)
	if err != nil {
		return err
	}
	if address.EncodeAddress() != mpcAddress {
		return fmt.Errorf("public key address %v is not the configed mpc address %v", address, mpcAddress)
	}
	return nil
}

// NewAddressPubKeyHash encap
func (b *Bridge) NewAddressPubKeyHash(pkData []byte) (*btcutil.AddressPubKeyHash, error) {
	chainParams := b.GetChainParams(b.ChainConfig.GetChainID())
	return btcutil.NewAddressPubKeyHash(btcutil.Hash160(pkData), chainParams)
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

// ConvertTx converts btcjson raw tx result to elect tx
func ConvertTx(tx *btcjson.TxRawResult) *ElectTx {
	etx := &ElectTx{
		Txid:     &tx.Txid,
		Version:  new(uint32),
		Locktime: new(uint32),
		Size:     new(uint32),
		Weight:   new(uint32),
		Fee:      new(uint64),
		Vin:      make([]*ElectTxin, 0),
		Vout:     make([]*ElectTxOut, 0),
		Status:   TxStatus(tx),
	}
	*etx.Version = uint32(tx.Version)
	*etx.Locktime = tx.LockTime
	*etx.Size = uint32(tx.Size)
	*etx.Weight = uint32(tx.Weight)
	for i := 0; i < len(tx.Vin); i++ {
		evin := ConvertVin(&tx.Vin[i])
		etx.Vin = append(etx.Vin, evin)
	}
	for j := 0; j < len(tx.Vout); j++ {
		evout := ConvertVout(&tx.Vout[j])
		etx.Vout = append(etx.Vout, evout)
	}
	return etx
}

// TxStatus make elect tx status from btcjson tx raw result
func TxStatus(tx *btcjson.TxRawResult) *ElectTxStatus {
	status := &ElectTxStatus{
		Confirmed:   new(bool),
		BlockHeight: new(uint64),
		BlockHash:   new(string),
		BlockTime:   new(uint64),
	}
	*status.Confirmed = tx.Confirmations > 6
	*status.BlockHash = tx.BlockHash
	*status.BlockTime = uint64(tx.Blocktime)
	return status
}

// ConvertVin converts btcjson vin to elect vin
func ConvertVin(vin *btcjson.Vin) *ElectTxin {
	evin := &ElectTxin{
		Txid:         &vin.Txid,
		Vout:         &vin.Vout,
		Scriptsig:    new(string),
		ScriptsigAsm: new(string),
		IsCoinbase:   new(bool),
		Sequence:     &vin.Sequence,
	}
	if vin.ScriptSig != nil {
		*evin.Scriptsig = vin.ScriptSig.Hex
		*evin.ScriptsigAsm = vin.ScriptSig.Asm
	}
	*evin.IsCoinbase = (vin.Coinbase != "")
	return evin
}

// ConvertVout converts btcjson vout to elect vout
func ConvertVout(vout *btcjson.Vout) *ElectTxOut {
	evout := &ElectTxOut{
		Scriptpubkey:        &vout.ScriptPubKey.Hex,
		ScriptpubkeyAsm:     &vout.ScriptPubKey.Asm,
		ScriptpubkeyType:    new(string),
		ScriptpubkeyAddress: new(string),
		Value:               new(uint64),
	}
	switch vout.ScriptPubKey.Type {
	case "pubkeyhash":
		*evout.ScriptpubkeyType = p2pkhType
	case "scripthash":
		*evout.ScriptpubkeyType = p2shType
	default:
		*evout.ScriptpubkeyType = opReturnType
	}
	if len(vout.ScriptPubKey.Addresses) == 1 {
		*evout.ScriptpubkeyAddress = vout.ScriptPubKey.Addresses[0]
	}
	if len(vout.ScriptPubKey.Addresses) > 1 {
		*evout.ScriptpubkeyAddress = fmt.Sprintf("%+v", vout.ScriptPubKey.Addresses)
	}
	*evout.Value = uint64(vout.Value * 1e8)
	return evout
}

// ConvertBlock converts btcjson block verbose result to elect block
func ConvertBlock(blk *btcjson.GetBlockVerboseResult) *ElectBlock {
	eblk := &ElectBlock{
		Hash:         new(string),
		Height:       new(uint32),
		Version:      new(uint32),
		Timestamp:    new(uint32),
		TxCount:      new(uint32),
		Size:         new(uint32),
		Weight:       new(uint32),
		MerkleRoot:   new(string),
		PreviousHash: new(string),
		Nonce:        new(uint32),
		Bits:         new(uint32),
		Difficulty:   new(uint64),
	}
	*eblk.Hash = blk.Hash
	*eblk.Height = uint32(blk.Height)
	*eblk.Version = uint32(blk.Version)
	*eblk.Timestamp = uint32(blk.Time)
	*eblk.TxCount = uint32(len(blk.Tx))
	*eblk.Size = uint32(blk.Size)
	*eblk.Weight = uint32(blk.Weight)
	*eblk.MerkleRoot = blk.MerkleRoot
	*eblk.PreviousHash = blk.PreviousHash
	*eblk.Nonce = blk.Nonce
	if bits, err := strconv.ParseUint(blk.Bits, 16, 32); err == nil {
		*eblk.Bits = uint32(bits)
	}
	*eblk.Difficulty = uint64(blk.Difficulty)
	return eblk
}

// DecodeTxHex decode tx hex to msgTx
func DecodeTxHex(txHex string, protocolversion uint32, isWitness bool) *wire.MsgTx {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil
	}

	msgtx := new(wire.MsgTx)

	if isWitness {
		_ = msgtx.BtcDecode(bytes.NewReader(txBytes), protocolversion, wire.WitnessEncoding)
	} else {
		_ = msgtx.BtcDecode(bytes.NewReader(txBytes), protocolversion, wire.BaseEncoding)
	}

	return msgtx
}
