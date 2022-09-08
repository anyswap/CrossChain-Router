package cosmos

func NewCosmosRestClient(urls []string) *CosmosRestClient {
	return &CosmosRestClient{
		BaseUrls: urls,
	}
}
