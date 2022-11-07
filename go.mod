module github.com/anyswap/CrossChain-Router/v3

go 1.15

require (
	github.com/BurntSushi/toml v1.2.0
	github.com/btcsuite/btcd v0.22.1
	github.com/cosmos/cosmos-sdk v0.46.4
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
	github.com/cosmos/ibc-go/v6 v6.0.0-alpha1
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/lestrrat-go/strftime v1.0.6 // indirect
	github.com/tendermint/tendermint v0.34.22 // indirect
)

replace google.golang.org/grpc => google.golang.org/grpc v1.49.0

// replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.45.9

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
