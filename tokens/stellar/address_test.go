package stellar

import (
	"testing"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

func TestIsValidAddress(t *testing.T) {
	cases := []struct {
		Addr     string
		Expected bool
	}{
		{"GCHH22AXHDDXET47Q3YKSANZ74GAAWEIMECQ3ZNFVM3SJY2LGYYPFLUH", true},
		{"gchh22axhddxet47q3yksanz74gaaweimecq3znfvm3sjy2lgyypfluh", false},
		{"0xEDbe0d03d8022012a03d5535e8677681dbbd9bbd", false},
	}
	b := NewCrossChainBridge("1000005786703")

	for _, c := range cases {
		t.Run(c.Addr, func(t *testing.T) {
			if ans := b.IsValidAddress(c.Addr); ans != c.Expected {
				t.Fatalf("%s expected %v, but %v got", c.Addr, c.Expected, ans)
			}
		})
	}
}

func TestPublicKeyToAddress(t *testing.T) {
	cases := []struct {
		Pubkey   string
		Expected string
	}{
		{"be0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294", "GC7A2A6YAIQBFIB5KU26QZ3WQHN33G55CMFDLEZYRJQUKQJJ6XBJJPJJ"},
		{"146f71db711bc259176f9bcba1756308d2a7af0f1c0b90deece65997a84c8f56", "GAKG64O3OEN4EWIXN6N4XILVMMENFJ5PB4OAXEG65TTFTF5IJSHVMBIC"},
	}
	for _, c := range cases {
		t.Run(c.Pubkey, func(t *testing.T) {
			if ans, err := PublicKeyToAddress(common.Hex2Bytes(c.Pubkey)); err != nil || ans != c.Expected {
				t.Fatalf("%s expected %v, but %v got err:%v", c.Pubkey, c.Expected, ans, err)
			}
		})
	}
}

func TestVerifyMPCPubKey(t *testing.T) {
	cases := []struct {
		Pubkey string
		Addr   string
	}{
		{"EDbe0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294", "GC7A2A6YAIQBFIB5KU26QZ3WQHN33G55CMFDLEZYRJQUKQJJ6XBJJPJJ"},
		{"146f71db711bc259176f9bcba1756308d2a7af0f1c0b90deece65997a84c8f56", "GAKG64O3OEN4EWIXN6N4XILVMMENFJ5PB4OAXEG65TTFTF5IJSHVMBIC"},
	}
	for _, c := range cases {
		t.Run(c.Pubkey, func(t *testing.T) {
			if ans := VerifyMPCPubKey(c.Addr, c.Pubkey); ans != nil {
				t.Fatalf("%s expected %v, but got err:%v", c.Pubkey, c.Addr, ans)
			}
		})
	}
}
