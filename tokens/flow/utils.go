package flow

import (
	"fmt"
	"math/big"
)

var (
	FixLen = 8
)

func ParseFlowNumber(amount *big.Int) string {
	value := amount.String()
	if len(value) <= FixLen {
		return "0." + fmt.Sprintf("%08d", amount)
	}
	return value[0:len(value)-FixLen] + "." + value[len(value)-FixLen:]
}
