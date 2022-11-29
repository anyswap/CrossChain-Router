package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/client"
	"github.com/centrifuge/go-substrate-rpc-client/v4/config"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/gorilla/websocket"
	"github.com/mr-tron/base58"
	"github.com/vedhavyas/go-subkey"
)

var socketUrl = "wss://testnet.reefscan.com/graphql"
var httpUrl = "https://rpc-testnet.reefscan.com/"

func main() {

	api, err := NewReefAPI(config.Default().RPCURL)
	if err != nil {
		panic(err)
	}

	// meta, err := api.SubAPI.RPC.State.GetMetadataLatest()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf(meta.AsMetadataV13.Extrinsic.SignedExtensions[0])

	Example_simpleConnect(api.SubAPI)

	// Example_makeASimpleTransfer(api)

	// Example_swapoutTransfer(api,meta)

	// Example_query_tx()

	// Example_websocket()
}

func NewReefAPI(url string) (*ReefSubstrateAPI, error) {
	api, err := gsrpc.NewSubstrateAPI(config.Default().RPCURL)
	if err != nil {
		panic(err)
	}
	return &ReefSubstrateAPI{
		SubAPI: api,
		ReefAPI: &ReefAPI{
			client: api.Client,
		},
	}, nil
}

type ReefSubstrateAPI struct {
	SubAPI  *gsrpc.SubstrateAPI
	ReefAPI *ReefAPI
}

type ReefAPI struct {
	client client.Client
}

func (c *ReefAPI) EstimateGas(args ...interface{}) (types.Text, error) {
	var t types.Text
	err := c.client.Call(&t, "evm_estimateGas", args)
	return t, err
}

func Example_simpleConnect(api *gsrpc.SubstrateAPI) {
	// The following example shows how to instantiate a Substrate API and use it to connect to a node

	chain, err := api.RPC.System.Chain()
	if err != nil {
		panic(err)
	}
	nodeName, err := api.RPC.System.Name()
	if err != nil {
		panic(err)
	}
	nodeVersion, err := api.RPC.System.Version()
	if err != nil {
		panic(err)
	}
	fmt.Printf("You are connected to chain %v using %v v%v\n", chain, nodeName, nodeVersion)

	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		panic(err)
	}

	// jb, err := json.Marshal(meta)
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Printf("meta %s \n", string(jb))

	alice, err := signature.KeyringPairFromSecret("gentle spawn alien spider laptop output law curtain right ball someone churn", 42)
	if err != nil {
		panic(err)
	}

	fmt.Println(alice.PublicKey)
	addr, _ := subkey.SS58Address(alice.PublicKey, 42)
	fmt.Println(addr)
	fmt.Println(alice.Address)
	fmt.Println(common.ToHex(alice.PublicKey))
	fmt.Println(AddressToPubkey("5EJCwBLtHxNHJgyJjwgagtxP4x39CEMCPRoA48ZdajsR2DnR"))

	key, err := types.CreateStorageKey(meta, "System", "Account", alice.PublicKey)
	if err != nil {
		panic(err)
	}

	QueryAccount(api, key)
}

func QueryAccount(api *gsrpc.SubstrateAPI, key types.StorageKey) *types.AccountInfo {
	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		panic(err)
	}
	fmt.Printf("balance: %v\n", accountInfo.Data.Free.Uint64())
	return &accountInfo
}

func QueryContract(api *gsrpc.SubstrateAPI, key types.StorageKey) {
	var accountInfo map[string]interface{}
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		panic(err)
	}
	jb, err := json.Marshal(accountInfo)
	if err != nil {
		panic(err)
	}

	fmt.Printf("EvmAccountInfo  %s \n", string(jb))
}

func AddressToPubkey(base58Address string) []byte {
	addrBytes, _ := base58.Decode(base58Address)
	return addrBytes[1 : len(addrBytes)-2]
}

func Example_listenToNewBlocks(api *gsrpc.SubstrateAPI) {

	sub, err := api.RPC.Chain.SubscribeNewHeads()
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()

	count := 0

	for {
		head := <-sub.Chan()
		fmt.Printf("Chain is at block: #%v\n", head.Number)
		count++

		if count == 10 {
			sub.Unsubscribe()
			break
		}
	}
}

func Example_makeASimpleTransfer(reef *ReefSubstrateAPI, meta *types.Metadata) {
	// This sample shows how to create a transaction to make a transfer from one an account to another.

	api := reef.SubAPI
	// Create a call, transferring 12345 units to Bob
	bob := types.NewMultiAddressFromAccountID(AddressToPubkey("5FWXWnrt5uNSNuCWeuiDppuEvGT5CEPUHvzJSWDdzjJXLnbJ"))

	bobkey, _ := types.CreateStorageKey(meta, "System", "Account", bob.AsID[:])
	QueryAccount(api, bobkey)

	// 1 unit of transfer
	bal, ok := new(big.Int).SetString("1000000000000000000", 10)
	if !ok {
		panic(fmt.Errorf("failed to convert balance"))
	}

	c, err := types.NewCall(meta, "Balances.transfer", bob, types.NewUCompact(bal))
	if err != nil {
		panic(err)
	}

	// Create the extrinsic
	ext := types.NewExtrinsic(c)

	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		panic(err)
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		panic(err)
	}

	alice, err := signature.KeyringPairFromSecret("gentle spawn alien spider laptop output law curtain right ball someone churn", 42)
	if err != nil {
		panic(err)
	}

	fmt.Printf("alice addr %s /n", alice.Address)

	key, err := types.CreateStorageKey(meta, "System", "Account", alice.PublicKey)
	if err != nil {
		panic(err)
	}

	accountInfo := QueryAccount(api, key)

	nonce := uint32(accountInfo.Nonce)
	o := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(100),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the transaction using Alice's default account
	err = ext.Sign(alice, o)
	if err != nil {
		panic(err)
	}

	// Send the extrinsic
	hash, err := api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		panic(err)
	}
	fmt.Println(hash.Hex())

	// fmt.Printf("Balance transferred from Alice to Bob: %s %v\n", hash.Hex(), bal.String())
	// Output: Balance transferred from Alice to Bob: 100000000000000

	// Do the transfer and track the actual status
	// sub, err := api.RPC.Author.SubmitAndWatchExtrinsic(ext)
	// if err != nil {
	// 	panic(err)
	// }
	// defer sub.Unsubscribe()

	// for {
	// 	status := <-sub.Chan()
	// 	fmt.Printf("Transaction status: %#v\n", status)

	// 	//&& status.IsFinalized
	// 	if status.IsInBlock {
	// 		fmt.Printf("Completed at block hash: %#x\n, AsRetracted: %#x\n, AsFinalized: %#x\n, AsUsurped: %#x\n", status.AsInBlock, status.AsRetracted, status.AsFinalized, status.AsUsurped)
	// 		QueryAccount(api, bobkey)
	// 		return
	// 	}
	// }

}

func Example_swapoutTransfer(api *ReefSubstrateAPI, meta *types.Metadata) {

	amount, _ := common.GetBigIntFromStr("10000000000000000000000000")
	toChainID, _ := common.GetUint64FromStr("5777")

	input := abicoder.PackDataWithFuncHash(common.FromHex("0x825bb13c"),
		common.HexToHash("0x5f31dac7618ccf2df75e0f5c458603d7a3ee2acb48d977ee41da3e562d7a90f6"),
		common.HexToAddress("0x3A641961CEfA97052eC7f283C408CAb9682f540A"),
		common.HexToAddress("0x64e55A52425993D2b059CB398ec860c0339bCD01"),
		amount,
		new(big.Int).SetUint64(toChainID),
	)
	fmt.Println(common.ToHex(input) == "0x825bb13c5f31dac7618ccf2df75e0f5c458603d7a3ee2acb48d977ee41da3e562d7a90f60000000000000000000000003a641961cefa97052ec7f283c408cab9682f540a00000000000000000000000064e55a52425993d2b059cb398ec860c0339bcd01000000000000000000000000000000000000000000084595161401484a0000000000000000000000000000000000000000000000000000000000000000001691")
	fmt.Println(common.ToHex(input))

	router := types.NewAddressFromAccountID(common.FromHex("0x6E0aa801AA5B971ECEB1daD8D7CB9237a18617FD"))

	// routerKey, _ := types.CreateStorageKey(meta, "EVM", "Accounts", router.AsAccountID[:])
	// QueryContract(api, routerKey)
	value, _ := new(big.Int).SetString("0", 10)

	// gas, err := api.ReefAPI.EstimateGas(common.FromHex("0x6E0aa801AA5B971ECEB1daD8D7CB9237a18617FD"), input, types.NewUCompact(value), types.NewU64(100000000), types.NewU32(100000000))
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("gas %s /n", gas)

	c, err := types.NewCall(meta, "EVM.call", router.AsAccountID[:], input, types.NewUCompact(value), types.NewU64(100000000), types.NewU32(100000000))
	if err != nil {
		panic(err)
	}

	// Create the extrinsic
	ext := types.NewExtrinsic(c)

	genesisHash, err := api.SubAPI.RPC.Chain.GetBlockHash(0)
	if err != nil {
		panic(err)
	}

	rv, err := api.SubAPI.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		panic(err)
	}

	alice, err := signature.KeyringPairFromSecret("gentle spawn alien spider laptop output law curtain right ball someone churn", 42)
	if err != nil {
		panic(err)
	}

	fmt.Printf("alice addr %s /n", alice.Address)

	key, err := types.CreateStorageKey(meta, "System", "Account", alice.PublicKey)
	if err != nil {
		panic(err)
	}

	accountInfo := QueryAccount(api.SubAPI, key)

	nonce := uint32(accountInfo.Nonce)
	o := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(100),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the transaction using Alice's default account
	err = ext.Sign(alice, o)
	if err != nil {
		panic(err)
	}

	// Send the extrinsic
	hash, err := api.SubAPI.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		panic(err)
	}
	fmt.Println(hash.Hex())
}

var done chan interface{}
var interrupt chan os.Signal

func receiveHandler(connection *websocket.Conn) {
	defer close(done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			log.Println("Error in receive:", err)
			return
		}
		log.Printf("Received: %s\n", msg)
	}
}

type ReefGraphQLRequest struct {
	ID      string             `json:"id,omitempty"`
	Type    string             `json:"type"`
	Payload ReefGraphQLPayLoad `json:"payload"`
}

type ReefGraphQLPayLoad struct {
	Extensions    interface{}            `json:"extensions,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
	Query         string                 `json:"query,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

func Example_query_tx() {
	done = make(chan interface{})    // Channel to indicate that the receiverHandler is done
	interrupt = make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully

	signal.Notify(interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT

	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		log.Fatal("Error connecting to Websocket Server:", err)
	}
	defer conn.Close()
	go receiveHandler(conn)

	i := 0
	// Our main loop for the client
	// We send our relevant packets here
	for {
		select {
		case <-time.After(time.Duration(1) * time.Millisecond * 2000):
			var msg []byte
			switch i {
			case 0:
				msg, _ = json.Marshal(ReefGraphQLRequest{Type: "connection_init"})
			case 1:
				msg, _ = json.Marshal(ReefGraphQLRequest{
					ID:   strconv.Itoa(i),
					Type: "start",
					Payload: ReefGraphQLPayLoad{
						OperationName: "chain_info",
						Query:         "subscription chain_info {\n  chain_info {\n    name\n    count\n    __typename\n  }\n}",
					}})
			case 2:
				msg, _ = json.Marshal(ReefGraphQLRequest{
					ID:   strconv.Itoa(i),
					Type: "start",
					Payload: ReefGraphQLPayLoad{
						OperationName: "lastest_blocks",
						Query:         "subscription lastest_blocks {\n  block(limit: 1, order_by: {id: desc}, where: {finalized: {_eq: true}}) {\n    id\n    __typename\n  }\n}\n",
					}})
			case 3:
				msg, _ = json.Marshal(ReefGraphQLRequest{
					ID:   strconv.Itoa(i),
					Type: "start",
					Payload: ReefGraphQLPayLoad{
						OperationName: "query_tx_by_hash",
						Query:         "query query_tx_by_hash($hash: String!) {\n  extrinsic(where: {hash: {_eq: $hash}}) {\n    id\n    block_id\n    index\n    type\n    signer\n    section\n    method\n    args\n    hash\n    status\n    timestamp\n    error_message\n inherent_data\n signed_data\n   __typename\n  }\n}\n",
						Variables: map[string]interface{}{
							"hash": "0xfaba926e79f92c5dd62c40ae808342cb7beb8dd9e8ba15f848fdabbf0cd5af55",
						},
					}})
			case 4:
				msg, _ = json.Marshal(ReefGraphQLRequest{
					ID:   strconv.Itoa(i),
					Type: "start",
					Payload: ReefGraphQLPayLoad{
						OperationName: "query_eventlogs_by_extrinsic_id",
						Query:         "subscription query_eventlogs_by_extrinsic_id($extrinsic_id: bigint!) {\n  event(order_by: {index: asc}, where: {extrinsic_id: {_eq: $extrinsic_id}}) {\n    extrinsic {\n      id\n      block_id\n      index\n      __typename\n    }\n    index\n    data\n    method\n    section\n    __typename\n  }\n}\n",
						Variables: map[string]interface{}{
							"extrinsic_id": 5057193,
						},
					}})

			default:
				// msg, _ = json.Marshal(ReefGraphQLRequest{Type: "ka"})
			}
			if len(msg) > 0 {

				log.Println("send msg....", string(msg))
				// Send an echo packet every second
				err := conn.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					log.Println("Error during writing to websocket:", err)
					return
				}
			}
			i++
		case <-interrupt:
			// We received a SIGINT (Ctrl + C). Terminate gracefully...
			log.Println("Received SIGINT interrupt signal. Closing all pending connections")

			// Close our websocket connection
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Error during closing websocket:", err)
				return
			}

			select {
			case <-done:
				log.Println("Receiver Channel Closed! Exiting....")
			case <-time.After(time.Duration(1) * time.Second):
				log.Println("Timeout in closing receiving channel. Exiting....")
			}
			return
		}
	}
}

func Example_websocket() {

	ws, err := reef.NewWebSocket(socketUrl)
	if err != nil {
		log.Fatal("Error connecting to Websocket Server:", err)
	}

	go func() {
		for {
			select {
			case <-time.After(time.Duration(1) * time.Millisecond * 2000):
				go func() {
					log.Println("QueryTx start...")
					tx, err := ws.QueryTx("0xfaba926e79f92c5dd62c40ae808342cb7beb8dd9e8ba15f848fdabbf0cd5af55")
					if err != nil {
						log.Printf(err.Error())
					} else {
						log.Printf("QueryTx result %s %s", tx.Hash, tx.Status)
					}
				}()
			}
		}
	}()

	ws.Run()

	time.Sleep(30000 * time.Second)
}
