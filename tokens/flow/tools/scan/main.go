package main

import (
	"context"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/grpc"
)

var (
	ctx       = context.Background()
	url       = "access.devnet.nodes.onflow.org:9000"
	eventType = "A.cb2d04fc89307107.JoyrideMultiToken.JoyrideMultiTokenInfoEvent"
)

func main() {

	flowClient, err := grpc.NewClient(url)
	if err != nil {
		log.Fatal("connect failed", "url", url, "err", err)
	}

	result, err := flowClient.GetEventsForHeightRange(ctx, eventType, 73857392, 73857394)
	if err != nil {
		fmt.Printf("\n\nerr: %s", err)
	}
	allTransaction := printEvents(result)
	fmt.Printf("\n\nallTransaction: %s", allTransaction)
}

func printEvents(result []sdk.BlockEvents) []string {
	var allRes []string
	for _, block := range result {
		allRes = append(allRes, printEvent(block.Events)...)
	}
	return allRes
}

func printEvent(events []sdk.Event) []string {
	var res []string
	for _, event := range events {
		// fmt.Printf("\n\nType: %s", event.Type)
		// fmt.Printf("\nValues: %v", event.Value)
		// fmt.Printf("\nTransaction ID: %s", event.TransactionID)
		res = append(res, event.TransactionID.String())
	}
	return res
}
