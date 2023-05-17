package eth_test

import (
	"testing"

	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
)

func TestIsValidAddress(t *testing.T) {

	cases := []struct {
		Addr     string
		Expected bool
	}{
		{"0x90529C2CEA2C77856E777C993C6392CC60C5D5A2", true},
		{"0X90529C2CEA2C77856E777C993C6392CC60C5D5A2", true},
		{"0x90529c2cEA2c77856E777c993c6392cC60C5d5A2", true},
		{"0X90529C2CEa2c77856e777c993c6392cc60c5d5a2", false},
		{"0x90529C2CEa2c77856e777c993c6392cc60c5d5a2", false},
	}

	b := eth.NewCrossChainBridge()

	for _, v := range cases {
		rst := b.IsValidAddress(v.Addr)
		if rst != v.Expected {
			t.Fatalf("%s expected %v, but %v got", v.Addr, v.Expected, rst)
		}
	}
}
