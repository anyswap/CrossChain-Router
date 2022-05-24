// Package server provides JSON/RESTful RPC service.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	rpcjson "github.com/gorilla/rpc/v2/json2"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/rpc/restapi"
	"github.com/anyswap/CrossChain-Router/v3/rpc/rpcapi"
)

// StartAPIServer start api server
func StartAPIServer() {
	router := mux.NewRouter()
	initRouterSwapRouter(router)

	apiServer := params.GetRouterConfig().Server.APIServer
	apiPort := apiServer.Port
	allowedOrigins := apiServer.AllowedOrigins
	maxRequestsLimit := apiServer.MaxRequestsLimit
	if maxRequestsLimit <= 0 {
		maxRequestsLimit = 10 // default value
	}

	corsOptions := []handlers.CORSOption{
		handlers.AllowedMethods([]string{"GET", "POST"}),
	}
	if len(allowedOrigins) != 0 {
		corsOptions = append(corsOptions,
			handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"}),
			handlers.AllowedOrigins(allowedOrigins),
		)
	}

	log.Info("JSON RPC service listen and serving", "port", apiPort, "allowedOrigins", allowedOrigins)
	lmt := tollbooth.NewLimiter(float64(maxRequestsLimit),
		&limiter.ExpirableOptions{
			DefaultExpirationTTL: 600 * time.Second,
		},
	)
	handler := tollbooth.LimitHandler(lmt, handlers.CORS(corsOptions...)(router))
	svr := http.Server{
		Addr:         fmt.Sprintf(":%v", apiPort),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second,
		Handler:      handler,
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) && utils.IsCleanuping() {
				return
			}
			log.Fatal("ListenAndServe error", "err", err)
		}
	}()

	utils.TopWaitGroup.Add(1)
	go utils.WaitAndCleanup(func() { doCleanup(&svr) })
}

// StartTestServer start api test server
func StartTestServer(apiPort int) {
	router := mux.NewRouter()
	router.HandleFunc("/swap/test/{txid}", restapi.TestRouterSwapHandler).Methods("GET", "POST")

	corsOptions := []handlers.CORSOption{
		handlers.AllowedMethods([]string{"GET", "POST"}),
	}
	handler := handlers.CORS(corsOptions...)(router)

	log.Info("JSON RPC test service listen and serving", "port", apiPort)
	svr := http.Server{
		Addr:         fmt.Sprintf(":%v", apiPort),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second,
		Handler:      handler,
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) && utils.IsCleanuping() {
				return
			}
			log.Fatal("ListenAndServe error", "err", err)
		}
	}()

	utils.TopWaitGroup.Add(1)
	go utils.WaitAndCleanup(func() { doCleanup(&svr) })
}

func doCleanup(svr *http.Server) {
	defer utils.TopWaitGroup.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := svr.Shutdown(ctx); err != nil {
		log.Error("Server Shutdown failed", "err", err)
	}
	log.Info("Close http server success")
}

func initRouterSwapRouter(r *mux.Router) {
	rpcserver := rpc.NewServer()
	rpcserver.RegisterCodec(rpcjson.NewCodec(), "application/json")
	err := rpcserver.RegisterService(new(rpcapi.RouterSwapAPI), "swap")
	if err != nil {
		log.Fatal("start rpc service failed", "err", err)
	}

	r.Handle("/rpc", rpcserver)

	r.HandleFunc("/versioninfo", restapi.VersionInfoHandler).Methods("GET")
	r.HandleFunc("/serverinfo", restapi.ServerInfoHandler).Methods("GET")
	r.HandleFunc("/oracleinfo", restapi.OracleInfoHandler).Methods("GET")
	r.HandleFunc("/statusinfo", restapi.StatusInfoHandler).Methods("GET")
	r.HandleFunc("/swap/register/{chainid}/{txid}", restapi.RegisterRouterSwapHandler).Methods("POST")
	r.HandleFunc("/swap/status/{chainid}/{txid}", restapi.GetRouterSwapHandler).Methods("GET")
	r.HandleFunc("/swap/history/{chainid}/{address}", restapi.GetRouterSwapHistoryHandler).Methods("GET")

	r.HandleFunc("/allchainids", restapi.GetAllChainIDsHandler).Methods("GET")
	r.HandleFunc("/alltokenids", restapi.GetAllTokenIDsHandler).Methods("GET")
	r.HandleFunc("/allmultichaintokens/{tokenid}", restapi.GetAllMultichainTokensHandler).Methods("GET")
	r.HandleFunc("/chainconfig/{chainid}", restapi.GetChainConfigHandler).Methods("GET")
	r.HandleFunc("/tokenconfig/{chainid}/{address:.*}", restapi.GetTokenConfigHandler).Methods("GET")
	r.HandleFunc("/swapconfig/{tokenid}/{fromchainid}/{tochainid}", restapi.GetSwapConfigHandler).Methods("GET")
	r.HandleFunc("/feeconfig/{tokenid}/{fromchainid}/{tochainid}", restapi.GetFeeConfigHandler).Methods("GET")
}
