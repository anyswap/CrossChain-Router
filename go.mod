module github.com/anyswap/CrossChain-Router/v3

go 1.15

require (
	github.com/BurntSushi/toml v1.2.0
	github.com/btcsuite/btcd v0.22.1
	github.com/cosmos/cosmos-sdk v0.46.1
	github.com/deckarep/golang-set v1.8.0
	github.com/didip/tollbooth/v6 v6.1.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/rpc v1.2.0
	github.com/gorilla/websocket v1.5.0
	github.com/jowenshaw/gethclient v0.3.1
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/mr-tron/base58 v1.2.0
	github.com/pborman/uuid v1.2.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.0
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	github.com/urfave/cli/v2 v2.10.2
	go.mongodb.org/mongo-driver v1.9.1
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
)

require (
	github.com/99designs/keyring v1.2.1 // indirect
	github.com/armon/go-metrics v0.4.0 // indirect
	github.com/coinbase/rosetta-sdk-go v0.7.9 // indirect
	github.com/fbsobreira/gotron-sdk v0.0.0-20221101181131-c4daceb828f0
	github.com/fxamacker/cbor/v2 v2.4.0
	github.com/golang/protobuf v1.5.2
	github.com/hdevalence/ed25519consensus v0.0.0-20220222234857-c00d1f31bab3 // indirect
	github.com/improbable-eng/grpc-web v0.15.0 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/lestrrat-go/strftime v1.0.6 // indirect
	github.com/near/borsh-go v0.3.1
	github.com/sei-protocol/sei-chain v0.0.0-20221020231357-7705edb69a11
	github.com/tendermint/tendermint v0.34.22 // indirect
	github.com/zondax/hid v0.9.1-0.20220302062450-5552068d2266 // indirect
	google.golang.org/protobuf v1.28.1
)

replace google.golang.org/grpc => google.golang.org/grpc v1.49.0

replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.45.9

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
