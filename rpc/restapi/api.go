package restapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/internal/swapapi"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/gorilla/mux"
)

func writeResponse(w http.ResponseWriter, resp interface{}, err error) {
	// Note: must set header before write header
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusOK)
	if err == nil {
		jsonData, _ := json.Marshal(resp)
		_, _ = w.Write(jsonData)
	} else {
		fmt.Fprintln(w, err.Error())
	}
}

// VersionInfoHandler handler
func VersionInfoHandler(w http.ResponseWriter, r *http.Request) {
	version := params.VersionWithMeta
	writeResponse(w, version, nil)
}

// IdentifierHandler handler
func IdentifierHandler(w http.ResponseWriter, r *http.Request) {
	identifier := params.GetIdentifier()
	writeResponse(w, identifier, nil)
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

func getHistoryRequestVaules(r *http.Request) (offset, limit int, err error) {
	vals := r.URL.Query()

	offsetStr, exist := vals["offset"]
	if exist {
		offset, err = common.GetIntFromStr(offsetStr[0])
		if err != nil {
			return offset, limit, err
		}
	}

	limitStr, exist := vals["limit"]
	if exist {
		limit, err = common.GetIntFromStr(limitStr[0])
		if err != nil {
			return offset, limit, err
		}
	}

	return offset, limit, nil
}

// GetRouterSwapHistoryHandler handler
func GetRouterSwapHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["chainid"]
	address := vars["address"]
	offset, limit, err := getHistoryRequestVaules(r)
	if err != nil {
		writeResponse(w, nil, err)
	} else {
		res, err := swapapi.GetRouterSwapHistory(chainID, address, offset, limit)
		writeResponse(w, res, err)
	}
}
