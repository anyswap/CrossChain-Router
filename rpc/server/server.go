package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	lmt := tollbooth.NewLimiter(10, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	handler := tollbooth.LimitHandler(lmt, handlers.CORS(corsOptions...)(router))
	svr := http.Server{
		Addr:         fmt.Sprintf(":%v", apiPort),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second,
		Handler:      handler,
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
			log.Error("ListenAndServe error", "err", err)
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
	_ = rpcserver.RegisterService(new(rpcapi.RouterSwapAPI), "swap")

	r.Handle("/rpc", rpcserver)

	registerHandleFunc(r, "/versioninfo", restapi.VersionInfoHandler, "GET")
	registerHandleFunc(r, "/serverinfo", restapi.ServerInfoHandler, "GET")
	registerHandleFunc(r, "/swap/register/{chainid}/{txid}", restapi.RegisterRouterSwapHandler, "POST")
	registerHandleFunc(r, "/swap/status/{chainid}/{txid}", restapi.GetRouterSwapHandler, "GET")
	registerHandleFunc(r, "/swap/history/{chainid}/{address}", restapi.GetRouterSwapHistoryHandler, "GET")

	registerHandleFunc(r, "/allchainids", restapi.GetAllChainIDsHandler, "GET")
	registerHandleFunc(r, "/alltokenids", restapi.GetAllTokenIDsHandler, "GET")
	registerHandleFunc(r, "/allmultichaintokens/{tokenid}", restapi.GetAllMultichainTokensHandler, "GET")
	registerHandleFunc(r, "/chainconfig/{chainid}", restapi.GetChainConfigHandler, "GET")
	registerHandleFunc(r, "/tokenconfig/{chainid}/{address}", restapi.GetTokenConfigHandler, "GET")
	registerHandleFunc(r, "/swapconfig/{tokenid}/{chainid}", restapi.GetSwapConfigHandler, "GET")
}

type handleFuncType = func(w http.ResponseWriter, r *http.Request)

func registerHandleFunc(r *mux.Router, path string, handler handleFuncType, methods ...string) {
	for i := 0; i < len(methods); i++ {
		methods[i] = strings.ToUpper(methods[i])
	}
	isAcceptMethod := func(method string) bool {
		for _, acceptMethod := range methods {
			if method == acceptMethod {
				return true
			}
		}
		return false
	}
	allMethods := []string{"GET", "POST", "HEAD", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
	excludedMethods := make([]string, 0, len(allMethods))
	for _, method := range allMethods {
		if !isAcceptMethod(method) {
			excludedMethods = append(excludedMethods, method)
		}
	}
	if len(methods) > 0 {
		acceptMethods := strings.Join(methods, ",")
		r.HandleFunc(path, handler).Methods(acceptMethods)
	}
	if len(excludedMethods) > 0 {
		forbidMethods := strings.Join(excludedMethods, ",")
		r.HandleFunc(path, warnHandler).Methods(forbidMethods)
	}
}

func warnHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Forbid '%v' on '%v'\n", r.Method, r.RequestURI)
}
