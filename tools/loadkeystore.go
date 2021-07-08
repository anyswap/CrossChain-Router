package tools

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/tools/keystore"
)

// LoadKeyStore load keystore from keyfile and passfile
func LoadKeyStore(keyfile, passfile string) (*keystore.Key, error) {
	keyjson, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, fmt.Errorf("read keystore fail %w", err)
	}
	passdata, err := ioutil.ReadFile(passfile)
	if err != nil {
		return nil, fmt.Errorf("read password fail %w", err)
	}
	passwd := strings.TrimSpace(string(passdata))
	key, err := keystore.DecryptKey(keyjson, passwd)
	if err != nil {
		return nil, fmt.Errorf("decrypt key fail %w", err)
	}
	return key, nil
}
