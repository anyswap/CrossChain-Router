package cosmosRouter

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return b.CosmosRestClient.SendTransaction(signedTx)
}
