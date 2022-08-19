package aptos

import "strings"

func (b *Bridge) IsNative(resource string) bool {
	return strings.Split(resource, "::")[0] == "0x1"
}
