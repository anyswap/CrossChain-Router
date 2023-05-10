package params

import (
	"encoding/hex"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tools"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

// KeystoreConfig keystore config
type KeystoreConfig struct {
	Address        string
	KeystoreFile   string `json:"-"`
	PasswordFile   string `json:"-"`
	PrivateKeyFile string `json:"-"`

	privKey string
}

type KeystoreConfigs []*KeystoreConfig

func (c *KeystoreConfig) String() string {
	return c.Address
}

func (cs KeystoreConfigs) String() string {
	s := "["
	for i, c := range cs {
		s += c.Address
		if i < len(cs)-1 {
			s += ", "
		}
	}
	s += "]"
	return s
}

// GetPrivateKey get private key
func (k *KeystoreConfig) GetPrivateKey() string {
	return k.privKey
}

// GetProofSubmitters get proof submitters
func GetProofSubmitters() []*KeystoreConfig {
	return GetRouterServerConfig().ProofSubmitters
}

// LoadProofSubmitters load proof submitters
func (c *RouterServerConfig) LoadProofSubmitters() {
	if len(c.ProofSubmitters) == 0 {
		return
	}
	for _, s := range c.ProofSubmitters {
		if s.PrivateKeyFile != "" {
			content, err := tools.SafeReadFile(s.PrivateKeyFile)
			if err != nil {
				log.Fatal("load proof submitters failed", "err", err)
			}
			s.privKey = strings.TrimSpace(string(content))
		} else {
			key, err := tools.LoadKeyStore(s.KeystoreFile, s.PasswordFile)
			if err != nil {
				log.Fatal("load proof submitters failed", "err", err)
			}
			if common.HexToAddress(s.Address) != key.Address {
				log.Fatal("load proof submitters address mismatch", "have", s.Address, "want", key.Address.LowerHex())
			}
			s.privKey = hex.EncodeToString(crypto.FromECDSA(key.PrivateKey))
		}
	}
	log.Info("load proof submitters success", "count", len(c.ProofSubmitters), "submitters", c.ProofSubmitters.String())
}
