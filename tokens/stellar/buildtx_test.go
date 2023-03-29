package stellar

import (
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func TestDecodeMemo(t *testing.T) {
	cases := []struct {
		Memo     string
		Expected *tokens.SwapTxInfo
	}{
		{"14c5107334a3ae117e3dad3570b419618c905aa5ec0000000000000000001691", &tokens.SwapTxInfo{
			Bind:      "0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC",
			ToChainID: big.NewInt(5777),
		}},

		{"141F05D743517471a4C4d2273F07B78Bfbd758e3c50000000000000000000005", &tokens.SwapTxInfo{
			Bind:      "0x1F05D743517471a4C4d2273F07B78Bfbd758e3c5",
			ToChainID: big.NewInt(5),
		}},
	}

	for _, c := range cases {
		t.Run(c.Memo, func(t *testing.T) {
			memo := base64.StdEncoding.EncodeToString(common.Hex2Bytes(c.Memo))
			if bind, tochainID := DecodeMemos(memo); !strings.EqualFold(bind, c.Expected.Bind) || tochainID.Cmp(c.Expected.ToChainID) != 0 {
				t.Fatalf("%s expected %v, but got %v %v", c.Memo, c.Expected, bind, tochainID)
			}
		})
	}
}

func TestEncodeMemo(t *testing.T) {
	cases := []struct {
		Expected string
		TxInfo   *tokens.SwapTxInfo
	}{
		{"14c5107334a3ae117e3dad3570b419618c905aa5ec0000000000000000001691", &tokens.SwapTxInfo{
			Bind:      "0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC",
			ToChainID: big.NewInt(5777),
		}},

		{"141F05D743517471a4C4d2273F07B78Bfbd758e3c50000000000000000000005", &tokens.SwapTxInfo{
			Bind:      "0x1F05D743517471a4C4d2273F07B78Bfbd758e3c5",
			ToChainID: big.NewInt(5),
		}},
	}

	for _, c := range cases {
		t.Run(c.Expected, func(t *testing.T) {
			if ans, err := EncodeMemo(c.TxInfo.ToChainID, c.TxInfo.Bind); err != nil || !strings.EqualFold(hex.EncodeToString(ans[:]), c.Expected) {
				t.Fatalf("%v expected %v, but got %v %v", c.TxInfo, c.Expected, hex.EncodeToString(ans[:]), err)
			}
		})
	}
}

func TestGetPaymentAmount(t *testing.T) {
	cases := []struct {
		Amount   string
		Decimals int
		Expected string
	}{
		{"1234567890123456789012345", 18, "1234567.8901234"},
		{"1234567890123456789", 6, "1234567890123.456789"},
		{"1234567890", 4, "123456.789"},
	}

	for _, c := range cases {
		t.Run(c.Amount, func(t *testing.T) {
			a, _ := big.NewInt(0).SetString(c.Amount, 10)
			if ans := getPaymentAmount(a, &tokens.TokenConfig{Decimals: uint8(c.Decimals)}); !strings.EqualFold(ans, c.Expected) {
				t.Fatalf("%s expected %v, but got %v", c.Amount, c.Expected, ans)
			}
		})
	}
}
