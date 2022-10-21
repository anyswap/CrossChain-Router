//nolint:deadcode,unused
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

func main() {
	client.InitHTTPClient()
	restClient := aptos.RestClient{
		Url:     "https://testnet.aptoslabs.com",
		Timeout: 10000,
	}

	mpc := "0x06da2b6027d581ded49b2314fa43016079e0277a17060437236f8009550961d6"

	// getCoinEventLog(&restClient, mpc, "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>", "deposit_events", 0, 10)

	// getSwapinEventLog(&restClient, mpc, mpc+"::Router::SwapInEventHolder", "events", 0, 10)

	getSwapoutEventLog(&restClient, mpc, mpc+"::Router::SwapOutEventHolder", "events", 0, 10)

}

func getCoinEventLog(restClient *aptos.RestClient, target, struct_resource, field_name string, start, limit int) {
	resp := &[]aptos.CoinEvent{}
	err := restClient.GetEventsByEventHandle(resp, target, struct_resource, field_name, start, limit)
	if err != nil {
		log.Fatal("GetEventsByEventHandle", "err", err)
	}
	json, err := json.Marshal(resp)
	if err != nil {
		log.Fatal("Marshal", "err", err)
	}
	println(string(json))
}

func getSwapinEventLog(restClient *aptos.RestClient, target, struct_resource, field_name string, start, limit int) {
	resp := &[]aptos.SwapinEvent{}
	err := restClient.GetEventsByEventHandle(resp, target, struct_resource, field_name, start, limit)
	if err != nil {
		log.Fatal("GetEventsByEventHandle", "err", err)
	}
	json, err := json.Marshal(resp)
	if err != nil {
		log.Fatal("Marshal", "err", err)
	}
	println(string(json))
}

func getSwapoutEventLog(restClient *aptos.RestClient, target, struct_resource, field_name string, start, limit int) {
	resp := &[]aptos.SwapoutEvent{}
	err := restClient.GetEventsByEventHandle(resp, target, struct_resource, field_name, start, limit)
	if err != nil {
		log.Fatal("GetEventsByEventHandle", "err", err)
	}
	for _, event := range *resp {
		tx, err := restClient.GetTransactionByVersion(event.Version)
		if err != nil {
			log.Fatal("GetEventsByEventHandle", "err", err)
		}
		fmt.Printf("version:%s, txhash:%s,status:%v type:%s", event.Version, tx.Hash, tx.Success, event.Type)
	}
}
