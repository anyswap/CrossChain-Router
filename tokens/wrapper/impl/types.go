package impl

import (
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

type RegisterSwapResult struct {
	SwapTxInfos []*tokens.SwapTxInfo
	Errs        []error
}

type locRegisterSwapResult struct {
	SwapTxInfos []*tokens.SwapTxInfo
	Errs        []string
}

func (r *RegisterSwapResult) MarshalJSON() ([]byte, error) {
	lr := &locRegisterSwapResult{
		SwapTxInfos: r.SwapTxInfos,
		Errs:        make([]string, len(r.Errs)),
	}
	for i, err := range r.Errs {
		if err != nil {
			lr.Errs[i] = err.Error()
		}
	}
	return json.Marshal(lr)
}

func (r *RegisterSwapResult) UnmarshalJSON(input []byte) error {
	var lr locRegisterSwapResult
	err := json.Unmarshal(input, &lr)
	if err != nil {
		return err
	}
	r.SwapTxInfos = lr.SwapTxInfos
	r.Errs = make([]error, len(lr.Errs))
	for i, err := range lr.Errs {
		if err != "" {
			r.Errs[i] = errors.New(err)
		}
	}
	return nil
}

func (r *RegisterSwapResult) String() string {
	data, _ := json.Marshal(r)
	return string(data)
}
