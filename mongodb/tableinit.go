package mongodb

import (
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	tbRouterSwaps       string = "RouterSwaps"
	tbRouterSwapResults string = "RouterSwapResults"
	tbUsedRValues       string = "UsedRValues"
)

var (
	collRouterSwap       *mongo.Collection
	collRouterSwapResult *mongo.Collection
	collUsedRValue       *mongo.Collection
)

func initCollections() {
	database := client.Database(databaseName)

	collRouterSwap = database.Collection(tbRouterSwaps)
	collRouterSwapResult = database.Collection(tbRouterSwapResults)
	collUsedRValue = database.Collection(tbUsedRValues)
}
