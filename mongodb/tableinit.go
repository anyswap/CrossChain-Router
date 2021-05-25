package mongodb

import (
	"gopkg.in/mgo.v2"
)

var (
	collRouterSwap       *mgo.Collection
	collRouterSwapResult *mgo.Collection
)

// do this when reconnect to the database
func deinintCollections() {
	collRouterSwap = database.C(tbRouterSwaps)
	collRouterSwapResult = database.C(tbRouterSwapResults)
}

func initCollections() {
	initCollection(tbRouterSwaps, &collRouterSwap, "inittime", "status", "fromChainID")
	initCollection(tbRouterSwapResults, &collRouterSwapResult, "inittime", "status", "fromChainID")
	_ = collRouterSwap.EnsureIndexKey("txid")                      // speed find swap
	_ = collRouterSwapResult.EnsureIndexKey("txid")                // speed find swap result
	_ = collRouterSwapResult.EnsureIndexKey("from", "fromChainID") // speed find history
}

func initCollection(table string, collection **mgo.Collection, indexKey ...string) {
	*collection = database.C(table)
	if len(indexKey) != 0 && indexKey[0] != "" {
		_ = (*collection).EnsureIndexKey(indexKey...)
	}
}
