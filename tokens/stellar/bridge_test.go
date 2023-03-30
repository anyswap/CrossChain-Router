//go:build ignore

package stellar

import (
	"testing"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func PrepareTestBridge() *Bridge {
	bridge := NewCrossChainBridge("1000005786703")
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress: []string{"https://horizon-testnet.stellar.org"},
	})
	bridge.InitRemotes()
	return bridge
}

func TestGetFee(t *testing.T) {
	bridge := PrepareTestBridge()
	if ans := bridge.GetFee(); ans == 0 {
		t.Fatalf("GetFee expected >0 , but got %v", ans)
	}
}

func TestGetAccount(t *testing.T) {
	bridget := PrepareTestBridge()
	if _, err := bridget.GetAccount("GC4Y6G2KHMKOVTQLVBKF4MDAXX2BWOYBRLZWQ2H6POLTVOLMWTMDP7BE"); err != nil {
		t.Fatalf("GetAccount expected not err, but got %v", err)
	}
}

func TestGetLatestBlockNumber(t *testing.T) {
	bridge := PrepareTestBridge()
	if num, err := bridge.GetLatestBlockNumber(); num == 0 || err != nil {
		t.Fatalf("GetLatestBlockNumber expected not err, but got %v %v", num, err)
	}
}

func TestGetTx(t *testing.T) {
	bridge := PrepareTestBridge()
	if _, err := bridge.GetTransaction("6fc1c449b740fd937b435e8414c1892fe9e5c5aacded3b4ca4ba0e5519862588"); err != nil {
		t.Fatalf("GetTx expected not err, but got %v", err)
	}
}

func TestGetAccountBalance(t *testing.T) {
	bridget := PrepareTestBridge()
	if _, err := bridget.GetBalance("GC4Y6G2KHMKOVTQLVBKF4MDAXX2BWOYBRLZWQ2H6POLTVOLMWTMDP7BE"); err != nil {
		t.Fatalf("TestGetAccountBalance expected not err, but got %v", err)
	}
}

func TestGetAsset(t *testing.T) {
	bridget := PrepareTestBridge()
	if _, err := bridget.GetAsset("ZUSD", "GACP35XDWYIP6IS5IL22CHOZKJOXICQIK4CFJ65Q4O2IIUNTKUGZY2KI"); err != nil {
		t.Fatalf("GetAsset expected not err, but got %v", err)
	}
}
