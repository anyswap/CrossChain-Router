package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	paramNetwork string
	paramIndex   string
	paramData    string
)

func initFlags() {
	flag.StringVar(&paramNetwork, "p", "", "network url")
	flag.StringVar(&paramIndex, "index", "", "payload index")
	flag.StringVar(&paramData, "data", "", "payload data")
	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(paramNetwork)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()

	// fetch the node's info to know the min. required PoW score
	if info, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		log.Fatal("Info", "paramNetwork", paramNetwork, "err", err)
	} else {
		fmt.Printf("info: %+v\n", info)

		// craft an indexation payload
		indexationPayload := &iotago.Indexation{
			Index: []byte(paramIndex),
			Data:  []byte(paramData),
		}

		if message, err := iotago.NewMessageBuilder().
			Payload(indexationPayload).
			Tips(ctx, nodeHTTPAPIClient).
			NetworkID(iotago.NetworkIDFromString(info.NetworkID)).
			ProofOfWork(ctx, info.MinPowScore).
			Build(); err != nil {
			log.Fatal("NewMessageBuilder", "err", err)
		} else {
			res, _ := nodeHTTPAPIClient.SubmitMessage(ctx, message)
			fmt.Printf("res: %+v\n", res)
		}
	}
}
