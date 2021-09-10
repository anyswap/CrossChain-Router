package mongodb

import (
	"github.com/anyswap/CrossChain-Router/v3/log"

	"go.mongodb.org/mongo-driver/bson"
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

	createOneIndex(collRouterSwap, "inittime", "status", "fromChainID")
	createOneIndex(collRouterSwap, "txid")

	createOneIndex(collRouterSwapResult, "inittime", "status", "fromChainID")
	createOneIndex(collRouterSwapResult, "txid")
	createOneIndex(collRouterSwapResult, "from", "fromChainID")

	log.Info("[mongodb] create indexes finished")
}

func createOneIndex(coll *mongo.Collection, indexes ...string) {
	keys := make([]bson.E, len(indexes))
	for i, index := range indexes {
		keys[i] = bson.E{Key: index, Value: 1}
	}
	model := mongo.IndexModel{Keys: keys}
	_, err := coll.Indexes().CreateOne(clientCtx, model)
	if err != nil {
		log.Error("[mongodb] create indexes failed", "collection", coll.Name(), "indexes", indexes, "err", err)
	}
}
