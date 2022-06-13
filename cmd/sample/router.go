package main

import(
    "fmt"
    "encoding/json"
    "net/http"
    
	"context"
	"crypto/ecdsa"
	"math/big"
    "math"
    "strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Token struct {
	Address  string `json:"address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type UnderlyingToken struct {
	Token
	HasUnderlying bool
}

type Chain struct {
	Address    string               `json:"address"`
	AnyToken   Token                `json:"anyToken"` 
    Underlying UnderlyingToken      `json:"underlying"`
	DestChains map[string]DestChain `json:"destChains"`
	Price      float64              `json:"price"`
	LogoURL    string               `json:"logoUrl"`
	ChainID    string               `json:"chainId"`
	Tokenid    string               `json:"tokenid"`
	Version    string               `json:"version"`
	Router     string               `json:"router"`
	RouterABI  string               `json:"routerABI"`
}

type DestChain struct {
	Address               string          `json:"address"`
	Underlying            UnderlyingToken `json:"underlying"`
	Swapfeeon             int             `json:"swapfeeon"`
	MaximumSwap           string          `json:"MaximumSwap"`
	MinimumSwap           string          `json:"MinimumSwap"`
	BigValueThreshold     string          `json:"BigValueThreshold"`
	SwapFeeRatePerMillion float64         `json:"SwapFeeRatePerMillion"`
	MaximumSwapFee        string          `json:"MaximumSwapFee"`
	MinimumSwapFee        string          `json:"MinimumSwapFee"`
	AnyToken              Token           `json:"anyToken"`
}

func (ut *UnderlyingToken) UnmarshalJSON(data []byte) error {
	var t Token
	if err := json.Unmarshal(data, &t); err == nil {
		ut.Address = t.Address
		ut.Decimals = t.Decimals
		ut.Name = t.Name
		ut.Symbol = t.Symbol
		ut.HasUnderlying = true
		return nil
	}
	return json.Unmarshal(data, &ut.HasUnderlying)
}

func floatStringToBigInt(amount string, decimals int)*big.Int{
	fAmount, _ := new(big.Float).SetString(amount)
	fi, _ := new(big.Float).Mul(fAmount, big.NewFloat(math.Pow10(decimals))).Int(nil)
	return fi
}

func getOpts(client *ethclient.Client, nonce *big.Int, privateKey *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		panic(err.Error())
	}

	opts, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		panic(err.Error())
	}
	if nonce != nil {
		opts.Nonce = nonce
	} else {
		var uint64Nonce uint64
		uint64Nonce, err = client.NonceAt(context.Background(), opts.From, nil)
		if err != nil {
			return nil, err
		}
		opts.Nonce = new(big.Int)
		opts.Nonce.SetUint64(uint64Nonce)
	}
	opts.Context = context.TODO()
	return opts, nil
}

const (
	bscUSDT   = "0x55d398326f99059ff775485246999027b3197955"
    bscChainIDStr = "56"
    oecChainID = 66
    oecChainIDStr = "66" 
    routerABI = `[{ "inputs": [ { "internalType": "address", "name": "token", "type": "address" }, { "internalType": "address", "name": "to", "type": "address" }, { "internalType": "uint256", "name": "amount", "type": "uint256" }, { "internalType": "uint256", "name": "toChainID", "type": "uint256" } ], "name": "anySwapOutUnderlying", "outputs": [ ], "stateMutability": "nonpayable", "type": "function" }]`
)

func main(){
	response, err := http.Get("https://bridgeapi.anyswap.exchange/v3/serverinfoV3?chainId=all&version=all")
	if err != nil {
        panic(err.Error())
	}
	if response.StatusCode != http.StatusOK {
        fmt.Printf("anyswap response %d\n", response.StatusCode)
	}

    var tl map[string]map[string]map[string]Chain
    err = json.NewDecoder(response.Body).Decode(&tl)
    if err != nil{
        panic(err)
    }

    privateKey, err := crypto.GenerateKey() //replace your private key here
	if err != nil {
		panic(err.Error())
	}
    token := common.HexToAddress(tl["STABLEV3"][bscChainIDStr][bscUSDT].AnyToken.Address)
    node := "https://bsc-dataseed.binance.org/"
    routerContract := common.HexToAddress(tl["STABLEV3"][bscChainIDStr][bscUSDT].Router)
    minimalswap := tl["STABLEV3"][bscChainIDStr][bscUSDT].DestChains[oecChainIDStr].MinimumSwap
    decimal := tl["STABLEV3"][bscChainIDStr][bscUSDT].AnyToken.Decimals
    amount := floatStringToBigInt(minimalswap, decimal)
    abiIns, _ := abi.JSON(strings.NewReader(routerABI)) 

    fmt.Println("Dial blockchain network:", node)
	rpcClient, err := ethclient.Dial(node)
	if err != nil {
		panic(err.Error())
	}
	opts, err:=getOpts(rpcClient, nil, privateKey)
	if err!= nil{
		panic(err.Error())
	}
    
    fmt.Println("Sending transction....")
    transact := bind.NewBoundContract(routerContract, abiIns, rpcClient, rpcClient, rpcClient)
    tx, err := transact.Transact(opts, "anySwapOutUnderlying",
        token,
        crypto.PubkeyToAddress(privateKey.PublicKey),
        amount,
        big.NewInt(int64(oecChainID)),
    )
    if err != nil{
        panic(err.Error())
    }
    fmt.Println(tx.Hash())
}
