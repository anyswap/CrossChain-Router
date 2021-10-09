// Package restapi provides RESTful RPC service.
package restapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/internal/swapapi"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/gorilla/mux"
)

func writeResponse(w http.ResponseWriter, resp interface{}, err error) {
	if err != nil {
		writeErrResponse(w, err)
		return
	}
	jsonData, err := json.Marshal(resp)
	if err != nil {
		writeErrResponse(w, err)
		return
	}
	writeJSONResponse(w, jsonData)
}

func writeJSONResponse(w http.ResponseWriter, jsonData []byte) {
	// Note: must set header before write header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(jsonData)
	if err != nil {
		log.Warn("write response error", "data", common.ToHex(jsonData), "err", err)
	}
}

func writeErrResponse(w http.ResponseWriter, err error) {
	// Note: must set header before write header
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, err.Error())
}

// VersionInfoHandler handler
func VersionInfoHandler(w http.ResponseWriter, r *http.Request) {
	version := params.VersionWithMeta
	writeResponse(w, version, nil)
}

// ServerInfoHandler handler
func ServerInfoHandler(w http.ResponseWriter, r *http.Request) {
	serverInfo := swapapi.GetServerInfo()
	writeResponse(w, serverInfo, nil)
}

// OracleInfoHandler handler
func OracleInfoHandler(w http.ResponseWriter, r *http.Request) {
	oracleInfo := swapapi.GetOracleInfo()
	writeResponse(w, oracleInfo, nil)
}

func getRouterSwapKeys(r *http.Request) (chainID, txid, logIndex string) {
	vars := mux.Vars(r)
	chainID = vars["chainid"]
	txid = vars["txid"]

	vals := r.URL.Query()
	logIndex = "0"
	logIndexVals, exist := vals["logindex"]
	if exist {
		logIndex = logIndexVals[0]
	}
	return chainID, txid, logIndex
}

// RegisterRouterSwapHandler handler
func RegisterRouterSwapHandler(w http.ResponseWriter, r *http.Request) {
	chainID, txid, logIndex := getRouterSwapKeys(r)
	res, err := swapapi.RegisterRouterSwap(chainID, txid, logIndex)
	writeResponse(w, res, err)
}

// GetRouterSwapHandler handler
func GetRouterSwapHandler(w http.ResponseWriter, r *http.Request) {
	chainID, txid, logIndex := getRouterSwapKeys(r)
	res, err := swapapi.GetRouterSwap(chainID, txid, logIndex)
	writeResponse(w, res, err)
}

func getHistoryRequestVaules(r *http.Request) (offset, limit int, status string, err error) {
	vals := r.URL.Query()

	offsetStr, exist := vals["offset"]
	if exist {
		offset, err = common.GetIntFromStr(offsetStr[0])
		if err != nil {
			return offset, limit, status, err
		}
	}

	limitStr, exist := vals["limit"]
	if exist {
		limit, err = common.GetIntFromStr(limitStr[0])
		if err != nil {
			return offset, limit, status, err
		}
	}

	statusStr, exist := vals["status"]
	if exist {
		status = statusStr[0]
	}

	return offset, limit, status, nil
}

// GetRouterSwapHistoryHandler handler
func GetRouterSwapHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["chainid"]
	address := vars["address"]
	offset, limit, status, err := getHistoryRequestVaules(r)
	if err != nil {
		writeResponse(w, nil, err)
	} else {
		res, err := swapapi.GetRouterSwapHistory(chainID, address, offset, limit, status)
		writeResponse(w, res, err)
	}
}

// GetAllChainIDsHandler handler
func GetAllChainIDsHandler(w http.ResponseWriter, r *http.Request) {
	allChainIDs := router.AllChainIDs
	writeResponse(w, allChainIDs, nil)
}

// GetAllTokenIDsHandler handler
func GetAllTokenIDsHandler(w http.ResponseWriter, r *http.Request) {
	allTokenIDs := router.AllTokenIDs
	writeResponse(w, allTokenIDs, nil)
}

// GetAllMultichainTokensHandler handler
func GetAllMultichainTokensHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tokenID := vars["tokenid"]
	allMultichainTokens := router.GetCachedMultichainTokens(tokenID)
	writeResponse(w, allMultichainTokens, nil)
}

// GetChainConfigHandler handler
func GetChainConfigHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["chainid"]
	bridge := router.GetBridgeByChainID(chainID)
	if bridge == nil {
		writeResponse(w, nil, fmt.Errorf("chainID %v not exist", chainID))
	} else {
		chainConfig := swapapi.ConvertChainConfig(bridge.GetChainConfig())
		writeResponse(w, chainConfig, nil)
	}
}

// GetTokenConfigHandler handler
func GetTokenConfigHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["chainid"]
	address := vars["address"]
	bridge := router.GetBridgeByChainID(chainID)
	if bridge == nil {
		writeResponse(w, nil, fmt.Errorf("chainID %v not exist", chainID))
	} else {
		tokenConfig := swapapi.ConvertTokenConfig(bridge.GetTokenConfig(address))
		writeResponse(w, tokenConfig, nil)
	}
}

// GetSwapConfigHandler handler
func GetSwapConfigHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tokenID := vars["tokenid"]
	chainID := vars["chainid"]
	swapConfig := swapapi.ConvertSwapConfig(tokens.GetSwapConfig(tokenID, chainID))
	if swapConfig == nil {
		writeResponse(w, nil, fmt.Errorf("swap config not found"))
	} else {
		writeResponse(w, swapConfig, nil)
	}
}
