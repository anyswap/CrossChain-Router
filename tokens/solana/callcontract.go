package solana

import (
	"encoding/base64"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	errDocedeAccountData = errors.New("docode account data failed")
)

// GetMPCAddress query
func (b *Bridge) GetMPCAddress(programID string) (types.PublicKey, error) {
	acc, err := b.GetRouterAccount(programID)
	if err != nil {
		return types.PublicKey{}, err
	}
	return acc.MPC, nil
}

// GetRouterAccount query
func (b *Bridge) GetRouterAccount(programID string) (*RouterAccount, error) {
	filters := []map[string]interface{}{
		{"dataSize": 41},
	}
	res, err := b.GetProgramAccounts(programID, "base64", filters)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, errors.New("router should have one and only one router account")
	}
	account := res[0]
	data, ok := account.Account.Data.([]interface{})
	if !ok {
		return nil, errDocedeAccountData
	}
	base64Data, ok := data[0].(string)
	if !ok {
		return nil, errDocedeAccountData
	}
	accData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil || len(accData) != 41 {
		return nil, errDocedeAccountData
	}

	routerMPC := types.PublicKeyFromBytes(accData[8:40])
	routerBump := uint8(accData[40])

	routerAcc := RouterAccount{
		MPC:  routerMPC,
		Bump: routerBump,
	}
	return &routerAcc, nil
}

// RouterAccount struct
type RouterAccount struct {
	MPC  types.PublicKey `json:"mpc"`
	Bump uint8           `json:"bump"`
}

// GetTokenDecimals query
func (b *Bridge) GetTokenDecimals(tokenMint string) (uint8, error) {
	res, err := b.GetTokenSupply(tokenMint)
	if err != nil {
		return 0, err
	}
	return res.Value.Decimals, nil
}

// IsMinter query
func (b *Bridge) IsMinter(tokenMint, minterAddr string) (bool, error) {
	res, err := b.GetAccountInfo(tokenMint, "jsonParsed")
	if err != nil {
		return false, err
	}
	parsedData, ok := res.Value.Data.(map[string]interface{})
	if !ok {
		return false, errDocedeAccountData
	}
	parsedInfo, ok := parsedData["parsed"].(map[string]interface{})
	if !ok {
		return false, errDocedeAccountData
	}
	innerInfo, ok := parsedInfo["info"].(map[string]interface{})
	if !ok {
		return false, errDocedeAccountData
	}
	mintAuthority, ok := innerInfo["mintAuthority"].(string)
	if !ok {
		return false, errDocedeAccountData
	}
	return mintAuthority == minterAddr, nil
}

// GetTokenBalance query
func (b *Bridge) GetTokenBalance(tokenAccount string) (result *types.GetTokenAmountResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getTokenAccountBalance"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, tokenAccount, obj)
	return result, err
}

// GetTokenSupply query
func (b *Bridge) GetTokenSupply(tokenMint string) (result *types.GetTokenAmountResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getTokenSupply"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, tokenMint, obj)
	return result, err
}
