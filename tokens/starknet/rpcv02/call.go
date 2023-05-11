package rpcv02

import (
	"context"
	"strings"

	"github.com/dontpanicdao/caigo/types"
)

// Call a starknet function without creating a StarkNet transaction.
func (provider *Provider) Call(ctx context.Context, request types.FunctionCall, blockID BlockID) ([]string, error) {
	request.EntryPointSelector = types.BigToHex(types.GetSelectorFromName(request.EntryPointSelector))
	if len(request.Calldata) == 0 {
		request.Calldata = make([]string, 0)
	}
	request.EntryPointSelector = addPrefixToEntryPointSelector(request.EntryPointSelector)
	var result []string
	if err := do(ctx, provider.c, "starknet_call", &result, request, blockID); err != nil {
		// TODO: Bind Pathfinder/Devnet Error to
		// CONTRACT_NOT_FOUND, INVALID_MESSAGE_SELECTOR, INVALID_CALL_DATA, CONTRACT_ERROR, BLOCK_NOT_FOUND
		return nil, err
	}
	return result, nil
}

func addPrefixToEntryPointSelector(s string) string {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	if len(s) == 63 {
		s = "0" + s
	}
	s = "0x" + s
	return s
}
