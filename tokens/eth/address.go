package eth

import (
	"regexp"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
)

var (
	eip1167Proxies        = make(map[common.Address]common.Address) // proxy -> master
	maxEip1167ProxiesSize = 10000

	eip1167ProxyCodePattern = regexp.MustCompile("^0x363d3d373d3d3d363d73([0-9a-fA-F]{40})5af43d82803e903d91602b57fd5bf3$")
	eip1167ProxyCodeLen     = 45 // bytes
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}
	if b.DontCheckAddressMixedCase {
		return true
	}
	unprefixedHex, ok, hasUpperChar := common.GetUnprefixedHex(address)
	if hasUpperChar {
		// valid checksum
		if unprefixedHex != common.HexToAddress(address).Hex()[2:] {
			return false
		}
	}
	return ok
}

// IsContractAddress is contract address
func (b *Bridge) IsContractAddress(address string) (bool, error) {
	var code []byte
	var err error
	for i := 0; i < retryRPCCount; i++ {
		code, err = b.GetCode(address)
		if err == nil {
			return len(code) > 1, nil // unexpect RSK getCode return 0x00
		}
		time.Sleep(retryRPCInterval)
	}
	return false, err
}

// GetEIP1167Master get eip1167 master address
func (b *Bridge) GetEIP1167Master(proxy common.Address) (master common.Address) {
	master, exist := eip1167Proxies[proxy]
	if exist {
		return master
	}
	if len(eip1167Proxies) > maxEip1167ProxiesSize {
		eip1167Proxies = nil
	}

	proxyAddr := proxy.String()

	var code []byte
	var err error
	for i := 0; i < retryRPCCount; i++ {
		code, err = b.GetCode(proxyAddr)
		if err == nil && len(code) > 1 {
			break
		}
		log.Warn("GetEIP1167Master call getCode failed", "address", proxy, "err", err)
		time.Sleep(retryRPCInterval)
	}
	if len(code) != eip1167ProxyCodeLen {
		return master
	}

	matches := eip1167ProxyCodePattern.FindStringSubmatch(common.ToHex(code))
	if len(matches) == 2 {
		master = common.HexToAddress(matches[1])
		eip1167Proxies[proxy] = master
	}
	return master
}
