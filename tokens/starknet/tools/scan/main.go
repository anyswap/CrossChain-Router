package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet"
	"github.com/dontpanicdao/caigo/rpcv01"
	"golang.org/x/exp/slices"
)

type EventFilter struct {
	FromBlock         rpcv01.BlockID `json:"from_block"`
	ToBlock           rpcv01.BlockID `json:"to_block,omitempty"`
	Address           string         `json:"address,omitempty"`
	Keys              []string       `json:"keys,omitempty"`
	ChunkSize         int            `json:"chunk_size"`
	ContinuationToken string         `json:"continuation_token,omitempty"`
}

type EventsOutput struct {
	Events            []rpcv01.EmittedEvent `json:"events,omitempty"`
	ContinuationToken string                `json:"continuation_token,omitempty"`
}

var (
	paramURL  string
	paramKeys starknet.ArrayFlag
	paramFrom uint64
	paramTo   uint64
)

func main() {
	log.SetLogger(6, false, true)
	initFlags()

	if paramURL == "" {
		log.Fatalf("RPC endpoint is required")
	}

	output, err := getEvents(paramFrom, paramTo, paramURL)
	if err != nil {
		log.Fatalf("filter failed: ", err)
	}
	for _, event := range output.Events {
		for _, k := range paramKeys {
			if slices.Contains(event.Keys, k) {
				fmt.Printf("Block Number: %d\nData: %s\n", event.BlockNumber, event.Data)
			}
		}
	}
}

func initFlags() {
	flag.StringVar(&paramURL, "network", "", "network, eg. https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f")
	flag.Var(&paramKeys, "keys", "event topics (key)")
	flag.Uint64Var(&paramFrom, "from", 0, "from block number")
	flag.Uint64Var(&paramTo, "to", 0, "to block number")
	flag.Parse()
}

func getEvents(from, to uint64, url string) (*EventsOutput, error) {
	if from > to {
		log.Fatalf("from block number should be smaller than to block number")
	}
	var result EventsOutput
	f := EventFilter{
		/*
			CC: Pathfinder's starknet_getEvents supports filtering by keys, but it's super slow.
			Avg. response time is longer than 60s, sometimes returning with empty results though they are actually in block.
			So it's suggested to get all events, which takes much shorter, then filter them locally.
		*/
		FromBlock: rpcv01.BlockID{Number: &from},
		ToBlock:   rpcv01.BlockID{Number: &to},
		ChunkSize: 1000,
	}
	t := time.Now()
	if err := client.RPCPost(&result, url, "starknet_getEvents", f); err != nil {
		return nil, err
	}
	fmt.Printf("time spent: %fs\n", time.Since(t).Seconds())
	return &result, nil
}
