package eth

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	br = NewCrossChainBridge()
)

type consArgs struct {
	args    []string
	wantErr error
}

type verifyTxTest struct {
	wantErr error

	receipt *types.RPCTxReceipt

	allowCallByContract bool
}

const (
	tRouterAddress = "0x5555555555555555555555555555555555555555"
	tTokenAddress  = "0x6666666666666666666666666666666666666666"
	tWrongContract = "0x7777777777777777777777777777777777777777"
)

var consArgsSlice = []*consArgs{
	{ // 0
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: nil,
	},
	{ // 1
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			"0x2222222222222222222222222222222222222222", // to
			"true",         // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: nil,
	},
	{ // 2
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			"0x2222222222222222222222222222222222222222", // to
			"true",         // allowCallByContract
			tWrongContract, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrTxWithWrongContract,
	},
	{ // 3
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tWrongContract, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrTxWithWrongContract,
	},
	{ // 4
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tWrongContract, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrTxWithWrongContract,
	},
	{ // 5
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000007777777777777777777777777777777777777777",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrMissTokenConfig,
	},
	{ // 6
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"true", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrTxWithRemovedLog,
	},
	{ // 7
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
			"0x0000000000000000000000001111111111111111111111111111111111111111",
			"0x0000000000000000000000009999999999999999999999999999999999999999",
		},
		wantErr: tokens.ErrSwapoutLogNotFound,
	},
	{ // 8
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b280000000000000000000000000000000000000000000000000000000000000000890000000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: tokens.ErrTxWithWrongTopics,
	},
	{ // 9
		args: []string{
			"0x1111111111111111111111111111111111111111", // from
			tRouterAddress, // to
			"false",        // allowCallByContract
			tRouterAddress, // log address
			"0x00000000000000000000000000000000000000000000000000000000ee6b28000000000000000000000000000000000000000000000000000000000000000089000000000000000000000000000000000000000000000000000000000038", // log data
			"false", // log removed
			// log topics
			"0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a",
			"0x0000000000000000000000006666666666666666666666666666666666666666",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
			"0x000000000000000000000000c8e50e55aeac372572ea4512f718189784720c39",
		},
		wantErr: abicoder.ErrParseDataError,
	},
}

// TestVerifyTx test verify tx, compare the verify error with the wanted error
func TestVerifyTx(t *testing.T) {
	log.SetLogger(3, false, true)
	br.ChainConfig = &tokens.ChainConfig{
		BlockChain:     "testBlockChain",
		RouterContract: tRouterAddress,
	}
	br.SetTokenConfig(tTokenAddress,
		&tokens.TokenConfig{
			TokenID:         "testTokenID",
			ContractAddress: tTokenAddress,
		},
	)

	allPassed := true
	tests := constructTests(t, consArgsSlice)
	for i, test := range tests {
		err := verifyTestTx(test)
		if !errors.Is(err, test.wantErr) {
			receiptJs, _ := json.Marshal(test.receipt)
			allPassed = false
			t.Errorf("verify tx failed, index %v, allowCallByContract %v, receipt %v, want error '%v', real error '%v'",
				i, test.allowCallByContract, string(receiptJs), test.wantErr, err)
		}
	}
	if allPassed {
		t.Logf("test verify tx all passed with %v test cases", len(tests))
	}
}

func verifyTestTx(test *verifyTxTest) (err error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType                                             // SwapType
	swapInfo.Hash = "0x0000000000000000000000000000000000000000000000000000000000000000" // Hash

	params.SetAllowCallByContract(test.allowCallByContract)

	err = br.verifySwapTxReceipt(swapInfo, test.receipt)
	if err != nil {
		return err
	}
	return br.verifyERC20SwapTxLog(swapInfo, test.receipt.Logs[0])
}

func constructTests(t *testing.T, argsSlice []*consArgs) []*verifyTxTest {
	tests := make([]*verifyTxTest, 0, len(argsSlice))
	for _, args := range argsSlice {
		test := constructTest(t, args.args, args.wantErr)
		if test != nil {
			tests = append(tests, test)
		}
	}
	return tests
}

func constructTest(t *testing.T, args []string, wantErr error) *verifyTxTest {
	if len(args) == 0 {
		return nil
	}
	return constructERC20SwapTxTest(t, args, wantErr)
}

func constructERC20SwapTxTest(t *testing.T, args []string, wantErr error) *verifyTxTest {
	test := &verifyTxTest{
		wantErr: wantErr,
	}

	from, to := getFromToAddress(t, args)
	test.allowCallByContract = strings.EqualFold(args[2], "true")
	logAddr, logData, removed, topics := getLogInfo(t, args[3:])

	log := &types.RPCLog{
		Address: logAddr,
		Topics:  topics,
		Data:    &logData,
		Removed: &removed,
	}

	test.receipt = &types.RPCTxReceipt{
		From:      from,
		Recipient: to,
		Logs:      []*types.RPCLog{log},
	}
	return test
}

func getFromToAddress(t *testing.T, args []string) (from, to *common.Address) {
	t.Helper()
	if len(args) < 2 {
		t.Errorf("getFromToAddress with less args: %v", args)
		return
	}
	fromAddr := common.HexToAddress(args[0])
	toAddr := common.HexToAddress(args[1])
	return &fromAddr, &toAddr
}

func getLogInfo(t *testing.T, args []string) (logAddr *common.Address, logData hexutil.Bytes, removed bool, topics []common.Hash) {
	t.Helper()
	if len(args) < 4 {
		t.Errorf("getLogInfo with less args: %v", args)
		return
	}
	addr := common.HexToAddress(args[0])
	logData = common.FromHex(args[1])
	removed = strings.EqualFold(args[2], "true")

	topics = make([]common.Hash, len(args)-3)
	for i := 3; i < len(args); i++ {
		topics[i-3] = common.HexToHash(args[i])
	}
	return &addr, logData, removed, topics
}
