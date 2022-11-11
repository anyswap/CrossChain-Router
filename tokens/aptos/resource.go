package aptos

import (
	"fmt"
	"strings"
)

func (b *Bridge) IsNative(resource string) bool {
	return strings.Split(resource, "::")[0] == "0x1"
}

func GetRouterModelId(address, modelname string) string {
	return fmt.Sprintf("%s::%s", address, modelname)
}

func GetRouterFunctionId(address, modelname, function string) string {
	return fmt.Sprintf("%s::%s::%s", address, modelname, function)
}
