package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	url   = "https://api.lb-0.h.chrysalis-devnet.iota.cafe"
	ctx   = context.Background()
	Index = "swapOut"
	mpc   = "9fb648524b9747608791dbd76bacbebc2f7ac0e3ace10e896739a0a44190102f"
)

func main() {
	log.SetLogger(6, false, true)

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)

	if messages, err := nodeHTTPAPIClient.MessageIDsByIndex(ctx, []byte(Index)); err != nil {
		log.Fatal("MessageIDsByIndex error", "Index", Index, "err", err)
	} else {
		for _, messageId := range messages.MessageIDs {
			if msgID, err := iota.ConvertMessageID(messageId); err != nil {
				log.Error("ConvertMessageID error", "messageId", messageId, "err", err)
			} else {
				if _, err := iota.GetTransactionMetadata(url, msgID); err != nil {
					log.Error("GetTransactionMetadata error", "messageId", messageId, "err", err)
				} else {
					if txRes, err := iota.GetTransactionByHash(url, msgID); err != nil {
						log.Error("GetTransactionByHash error", "messageId", messageId, "err", err)
					} else {
						if payloadRaw, err := txRes.Payload.MarshalJSON(); err != nil {
							log.Error("MarshalJSON error", "messageId", messageId, "err", err)
						} else {
							if err := ParseMessagePayload(payloadRaw); err != nil {
								log.Error("ParseMessagePayload error", "messageId", messageId, "err", err)
							} else {
								log.Info("verify success", "messageId", messageId)
							}
						}
					}
				}
			}

		}
	}
}

func ParseMessagePayload(payload []byte) error {
	var messagePayload iota.MessagePayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		return err
	} else {
		var amount uint64
		if messagePayload.Type != 0 {
			return tokens.ErrPayloadType
		}
		for _, output := range messagePayload.Essence.Outputs {
			if output.Address.Address == mpc {
				amount += output.Amount
			}
		}
		if amount == 0 {
			return tokens.ErrTxWithWrongValue
		} else {
			if err := ParseIndexPayload(messagePayload.Essence.Payload); err != nil {
				return err
			} else {
				return nil
			}
		}
	}
}

func ParseIndexPayload(payload iota.Payload) error {
	if payload.Type != 2 {
		return tokens.ErrPayloadType
	}
	if index, err := hex.DecodeString(payload.Index); err != nil || string(index) != Index {
		return err
	}
	if data, err := hex.DecodeString(payload.Data); err != nil {
		return tokens.ErrPayloadType
	} else {
		if fields := strings.Split(string(data), ":"); len(fields) != 2 {
			return tokens.ErrPayloadType
		} else {
			return nil
		}
	}
}
