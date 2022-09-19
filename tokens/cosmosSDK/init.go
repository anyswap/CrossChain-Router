package cosmosSDK

func NewCosmosRestClient(urls []string) *CosmosRestClient {
	return &CosmosRestClient{
		BaseUrls: urls,
	}
}

func (c *CosmosRestClient) SetBaseUrls(urls []string) {
	c.BaseUrls = urls
}
