package starknet

import (
	"fmt"
	"github.com/dontpanicdao/caigo"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

const INVOKE = "invoke"

func (b *Bridge) TransactionHash(calls FunctionCall, maxFee, nonce *big.Int) (*big.Int, error) {
	switch {
	case b.account.version == DefaultAccountVersion:
		callArray := fmtCalldata([]FunctionCall{calls})
		cdHash, err := caigo.Curve.ComputeHashOnElements(callArray)
		if err != nil {
			return nil, err
		}
		version, _ := big.NewInt(0).SetString("0x1", 0)
		multiHashData := []*big.Int{
			UTF8StrToBig(INVOKE),
			version, //big.NewInt(int64(b.account.version)),
			SNValToBN(b.account.Address),
			big.NewInt(0),
			cdHash,
			maxFee,
			//UTF8StrToBig(b.account.chainId),
			UTF8StrToBig("SN_GOERLI"),
			nonce,
		}
		return caigo.Curve.ComputeHashOnElements(multiHashData)
	default:
		return nil, fmt.Errorf("starknet version %d unsupported", b.account.version)
	}
}

func HexToHash(s string) Hash { return BytesToHash(ethcommon.FromHex(s)) }

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

func fmtCalldataStrings(calls []FunctionCall) (calldataStrings []string) {
	callArray := fmtCalldata(calls)
	for _, data := range callArray {
		calldataStrings = append(calldataStrings, fmt.Sprintf("0x%s", data.Text(16)))
	}
	return calldataStrings
}

// Formats the multicall transactions in a format which can be signed and verified
// by the network and OpenZeppelin account contracts
func fmtCalldata(calls []FunctionCall) (calldataArray []*big.Int) {
	callArray := []*big.Int{big.NewInt(int64(len(calls)))}

	for _, tx := range calls {
		address, _ := big.NewInt(0).SetString(tx.ContractAddress.Hex(), 0)
		callArray = append(callArray, address, GetSelectorFromName(tx.EntryPointSelector))

		if len(tx.Calldata) == 0 {
			callArray = append(callArray, big.NewInt(0), big.NewInt(0))

			continue
		}

		callArray = append(callArray, big.NewInt(int64(len(calldataArray))), big.NewInt(int64(len(tx.Calldata))))
		for _, cd := range tx.Calldata {
			calldataArray = append(calldataArray, SNValToBN(cd))
		}
	}

	callArray = append(callArray, big.NewInt(int64(len(calldataArray))))
	callArray = append(callArray, calldataArray...)
	return callArray
}
