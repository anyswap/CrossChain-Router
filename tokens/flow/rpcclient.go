package flow

var (
	rpcTimeout = 60
)

// SetRPCTimeout set rpc timeout
func SetRPCTimeout(timeout int) {
	rpcTimeout = timeout
}

func GetBlockNumberByHash(url, hash string) (uint64, error) {
	return 0, nil
}

func GetLatestBlockHash(url string) (string, error) {
	return "", nil
}

// GetLatestBlockNumber get latest block height
func GetLatestBlockNumber(url string) (uint64, error) {
	return 0, nil
}

// GetLatestBlockNumber get latest block height
func GetAccountNonce(url, account, publicKey string) (uint64, error) {
	return 0, nil
}

func BroadcastTxCommit(url string, signedTx []byte) (string, error) {
	return "", nil
}

func functionCall(url, accountID, methodName, args string) ([]byte, error) {
	return nil, nil
}
