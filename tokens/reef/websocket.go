package reef

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	// writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time allowed to connect to server.
	dialTimeout = 5 * time.Second
)

var counter uint64

type WebSocket struct {
	// Incoming chan interface{} // read chan
	outgoing  chan Syncer // write chan
	interrupt chan interface{}
	ws        *websocket.Conn
	endpoint  string
	IsClose   bool
}

type Syncer interface {
	CommandName() string
	Done()
	Fail(message string)
	SetResult(data interface{})
}

func NewWebSocket(_endpoint string) (*WebSocket, error) {
	log.Info("new remote session", "remote", _endpoint)
	conn, _, err := websocket.DefaultDialer.Dial(_endpoint, nil)
	if err != nil {
		log.Fatal("Error connecting to Websocket Server", "err", err)
		return nil, err
	}
	r := &WebSocket{
		// Incoming: make(chan interface{}, 1000),
		outgoing:  make(chan Syncer, 1000),
		interrupt: make(chan interface{}),
		ws:        conn,
		endpoint:  _endpoint,
		IsClose:   true,
	}
	log.Info("new remote session", "remote", _endpoint, "status", "success")
	return r, nil
}

// Close shuts down the Remote session and blocks until all internal
// goroutines have been cleaned up.
// Any commands that are pending a response will return with an error.
func (r *WebSocket) Close() {
	r.IsClose = true
	close(r.outgoing)
	go r.ReConn()
}

func (r *WebSocket) ReConn() {
	for {
		conn, _, err := websocket.DefaultDialer.Dial(r.endpoint, nil)
		if err != nil {
			log.Warn("Reef reconnecting to Websocket Server", "err", err)
			time.Sleep(dialTimeout)
			continue
		}
		log.Info("Reef ws reconnect success", "url", r.endpoint)
		r.outgoing = make(chan Syncer, 100)
		r.ws = conn
		go r.Run()
		break
	}
}

// run spawns the read/write pumps and then runs until Close() is called.
func (r *WebSocket) Run() {
	outbound := make(chan interface{})
	inbound := make(chan []byte)
	pending := make(map[string]Syncer)

	defer func() {
		close(outbound) // Shuts down the write
		// close(r.Incoming)

		// Cancel all pending commands with an error
		for _, c := range pending {
			c.Fail("Connection Closed")
		}

		// Drain the inbound channel and block until it is closed,
		// indicating that the read has returned.
		for range inbound {
		}

		r.Close()
	}()

	// Spawn read/write goroutines
	go func() {
		defer func() {
			log.Info("Reef WS Write close", "url", r.ws.RemoteAddr().String())
			r.ws.Close()
		}()
		r.Write(outbound)
	}()
	go func() {
		defer func() {
			log.Info("Reef WS Read close", "url", r.ws.RemoteAddr().String())
			close(inbound)
		}()
		r.Read(inbound)
	}()
	go func() {
		time.Sleep(2 * time.Second)
		r.InitConn()
	}()
	// Main run loop
ForEnd:
	for {
		select {
		case command, ok := <-r.outgoing:
			if !ok {
				return
			}
			outbound <- command
			id := reflect.ValueOf(command).Elem().FieldByName("ID").String()
			pending[id] = command

		case in, ok := <-inbound:
			if !ok {
				log.Error("Connection closed by server", "remote", r.ws.RemoteAddr())
				return
			}
			var response ReefGraphQLBaseResponse

			if err := json.Unmarshal(in, &response); err != nil {
				log.Error("json unmarshal response error", "err", err)
				continue
			}

			log.Debugf("%s", string(in))
			// pong message
			if response.Type == "ka" {
				continue
			}
			// Stream message
			cmd, ok := pending[response.ID]
			if !ok {
				log.Warnf("Unexpected message: %+v", string(in))
				continue
			}

			switch response.Type {
			case "error":
				cmd.Fail(response.Message)
			case "complete":
				// ignore
				cmd.Done()
			default:
				factory, ok := streamMessageFactory[cmd.CommandName()]
				if ok {
					data := &ReefGraphQLResponse{
						Payload: ReefGraphQLResponsePayload{
							Data: factory(),
						},
					}
					if err := json.Unmarshal(in, data); err != nil {
						log := fmt.Sprintln("msg", string(in), "err", err)
						cmd.Fail(log)
					} else {
						cmd.SetResult(data)
					}
				} else {
					cmd.Fail("not support command: " + cmd.CommandName())
				}
			}
			delete(pending, response.ID)
		case <-r.interrupt:
			log.Warn("Received SIGINT interrupt signal. Closing all pending connections", "url", r.ws.RemoteAddr().String())
			break ForEnd
		}
	}
}

// Consumes from the outbound channel and sends them over the websocket.
// Also sends PING messages at the specified interval.
// Returns when outbound channel is closed, or an error is encountered.
func (r *WebSocket) Write(outbound <-chan interface{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {

		// An outbound message is available to send
		case message, ok := <-outbound:
			if !ok {
				r.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			b, err := json.Marshal(message)
			if err != nil {
				// Outbound message cannot be JSON serialized (log it and continue)
				log.Error("json marshal error", "err", err)
				continue
			}
			// if params.IsDebugMode() {
			log.Info("ws write message", "message", string(b))
			// }
			if err := r.ws.WriteMessage(websocket.TextMessage, b); err != nil {
				log.Error("ws write message error", "remote", r.ws.RemoteAddr(), "err", err)
				return
			}

		// Time to send a ping
		case <-ticker.C:
			if err := r.ws.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				log.Error("ws write ping message error", "remote", r.ws.RemoteAddr(), "err", err)
			}
		}
	}
}

// readPump reads from the websocket and sends to inbound channel.
// Expects to receive PONGs at specified interval, or logs an error and returns.
func (r *WebSocket) Read(inbound chan<- []byte) {
	r.ws.SetReadDeadline(time.Now().Add(pongWait))
	r.ws.SetPongHandler(func(message string) error {
		log.Info("ws pong handler", "message", message)
		r.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := r.ws.ReadMessage()
		if err != nil {
			log.Error("ws read message error", "remote", r.ws.RemoteAddr(), "err", err)
			// 异常断线重连
			r.interrupt <- struct{}{}
			break
		}
		if params.IsDebugMode() {
			log.Info("ws read message", "message", string(message))
		}
		r.ws.SetReadDeadline(time.Now().Add(pongWait))
		inbound <- message
	}
}

type Command struct {
	ID     string        `json:"id,omitempty"`
	Name   string        `json:"-"`
	Error  *string       `json:"-"`
	Ready  chan struct{} `json:"-"`
	Result interface{}   `json:"-"`
}

func (c *Command) Done() {
	c.Ready <- struct{}{}
}

func (c *Command) Fail(message string) {
	c.Error = &message
	c.Ready <- struct{}{}
}

func (c *Command) CommandName() string {
	return c.Name
}

func (c *Command) SetResult(data interface{}) {
	c.Result = data
	c.Ready <- struct{}{}
}

func newCommand(command string) *Command {
	return &Command{
		ID:    strconv.FormatUint(atomic.AddUint64(&counter, 1), 10),
		Name:  command,
		Ready: make(chan struct{}),
	}
}

func (r *WebSocket) InitConn() error {
	cmd := &ReefGraphQLRequest{
		Command: &Command{
			Name:  "init",
			Ready: make(chan struct{}),
		},
		Type: "connection_init",
	}
	r.outgoing <- cmd
	<-cmd.Ready
	if cmd.Error != nil {
		return fmt.Errorf("reef InitConn err: %s", *cmd.Error)
	}
	r.IsClose = false
	return nil
}

func (r *WebSocket) SendCommond(cmd *ReefGraphQLRequest) error {
	if r.IsClose {
		return fmt.Errorf("reef ws: %s closed", r.endpoint)
	}
	r.outgoing <- cmd
	return nil
}

// Synchronously get a single transaction
func (r *WebSocket) QueryTx(hash string) (*Extrinsic, error) {
	cmd := &ReefGraphQLRequest{
		Command: newCommand("tx"),
		Type:    "start",
		Payload: ReefGraphQLPayLoad{
			OperationName: "query_tx_by_hash",
			Query:         TxHash_GQL,
			Variables: map[string]interface{}{
				"hash": hash,
			},
		},
	}
	start := time.Now()
	err := r.SendCommond(cmd)
	if err != nil {
		return nil, err
	}
	<-cmd.Ready
	if cmd.Error != nil {
		return nil, fmt.Errorf("reef query tx err: %s", *cmd.Error)
	}
	log.Infof("reef QueryTx cost:%d", time.Since(start).String())
	if f, ok := cmd.Result.(*ReefGraphQLResponse).Payload.Data.(*ReefGraphQLTxData); ok {
		if len(f.Extrinsic) > 0 {
			return &f.Extrinsic[0], nil
		}
	}
	return nil, nil
}

func (r *WebSocket) QueryEventLogs(extrinsicId uint64) (*[]EventLog, error) {
	cmd := &ReefGraphQLRequest{
		Command: newCommand("eventlog"),
		Type:    "start",
		Payload: ReefGraphQLPayLoad{
			OperationName: "query_eventlogs_by_extrinsic_id",
			Query:         EventLog_GQL,
			Variables: map[string]interface{}{
				"extrinsic_id": extrinsicId,
			},
		}}
	start := time.Now()
	err := r.SendCommond(cmd)
	if err != nil {
		return nil, err
	}
	<-cmd.Ready
	if cmd.Error != nil {
		return nil, fmt.Errorf("reef query tx err: %s", *cmd.Error)
	}
	log.Infof("reef QueryEventLogs cost:%d", time.Since(start).String())
	if f, ok := cmd.Result.(*ReefGraphQLResponse).Payload.Data.(*ReefGraphQLEventLogsData); ok {
		if len(f.Events) > 0 {
			return &f.Events, nil
		}
	}
	return nil, nil
}

func (r *WebSocket) QueryEvmAddress(ss58address string) (*common.Address, error) {
	cmd := &ReefGraphQLRequest{
		Command: newCommand("address"),
		Type:    "start",
		Payload: ReefGraphQLPayLoad{
			OperationName: "query_evm_addr",
			Query:         EvmAddress_GQL,
			Variables: map[string]interface{}{
				"address": ss58address,
			},
		}}
	start := time.Now()
	err := r.SendCommond(cmd)
	if err != nil {
		return nil, err
	}
	<-cmd.Ready
	if cmd.Error != nil {
		return nil, fmt.Errorf("reef query tx err: %s", *cmd.Error)
	}
	log.Infof("reef QueryEventLogs cost:%d", time.Since(start).String())
	if f, ok := cmd.Result.(*ReefGraphQLResponse).Payload.Data.(*ReefGraphQLAccountData); ok {
		if len(f.Accounts) > 0 {
			evmAddr := common.HexToAddress(f.Accounts[0].EvmAddress)
			return &evmAddr, nil
		}
	}
	return nil, nil
}
