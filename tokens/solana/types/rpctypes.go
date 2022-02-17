// Copyright 2020 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	bin "github.com/streamingfast/binary"
)

// Context type
type Context struct {
	Slot bin.Uint64
}

// RPCContext rpc context
type RPCContext struct {
	Context Context `json:"context,omitempty"`
}

// GetBalanceResult get balance result
type GetBalanceResult struct {
	RPCContext
	Value bin.Uint64 `json:"value"`
}

// GetTokenAmountResult get token amount result
// including get token account balance and get token supply
type GetTokenAmountResult struct {
	RPCContext
	Value TokenAmountResult `json:"value"`
}

// TokenAmountResult token amount result
type TokenAmountResult struct {
	Amount         bin.Uint64 `json:"amount"`
	Decimals       uint8      `json:"decimals"`
	UIAmountString string     `json:"uiAmountString"`
}

// GetSlotResult get slot result
type GetSlotResult bin.Uint64

// GetRecentBlockhashResult get recent block hash result
// DEPRECATED: Please use getFeeForMessage instead
// This method is expected to be removed in solana-core v2.0
type GetRecentBlockhashResult struct {
	RPCContext
	Value RecentBlockhashResult `json:"value"`
}

// RecentBlockhashResult block hash result
type RecentBlockhashResult struct {
	Blockhash     Hash          `json:"blockhash"`
	FeeCalculator FeeCalculator `json:"feeCalculator"`
}

// FeeCalculator fee calculator
type FeeCalculator struct {
	LamportsPerSignature bin.Uint64 `json:"lamportsPerSignature"`
}

// GetLatestBlockhashResult get latest block hash result
type GetLatestBlockhashResult struct {
	RPCContext
	Value LatestBlockhashResult `json:"value"`
}

// LatestBlockhashResult latest block hash result
type LatestBlockhashResult struct {
	Blockhash            Hash       `json:"blockhash"`
	LastValidBlockHeight bin.Uint64 `json:"lastValidBlockHeight"`
}

// GetBlockResult get block result
type GetBlockResult struct {
	Blockhash         Hash                  `json:"blockhash"`
	PreviousBlockhash Hash                  `json:"previousBlockhash"`
	ParentSlot        bin.Uint64            `json:"parentSlot"`
	Transactions      []TransactionWithMeta `json:"transactions,omitempty"`
	Signatures        []Signature           `json:"signatures,omitempty"`
	Rewards           []BlockReward         `json:"rewards"`
	BlockTime         bin.Uint64            `json:"blockTime,omitempty"`
}

// BlockReward block reward
type BlockReward struct {
	Pubkey   PublicKey  `json:"pubkey"`
	Lamports bin.Uint64 `json:"lamports"`
}

// TransactionWithMeta tx with meta
type TransactionWithMeta struct {
	Slot        bin.Uint64       `json:"slot,omitempty"`
	Transaction *Transaction     `json:"transaction"`
	BlockTime   bin.Uint64       `json:"blockTime,omitempty"`
	Meta        *TransactionMeta `json:"meta,omitempty"`
}

// TransactionParsed tx parsed
type TransactionParsed struct {
	Transaction *ParsedTransaction `json:"transaction"`
	Meta        *TransactionMeta   `json:"meta,omitempty"`
}

// TransactionMeta tx meta
type TransactionMeta struct {
	Err               interface{}     `json:"err"`
	Fee               bin.Uint64      `json:"fee"`
	PreBalances       []bin.Uint64    `json:"preBalances,omitempty"`
	PostBalances      []bin.Uint64    `json:"postBalances,omitempty"`
	PreTokenBalances  []TokenBalances `json:"preTokenBalances,omitempty"`
	PostTokenBalances []TokenBalances `json:"postTokenBalances,omitempty"`
	LogMessages       []string        `json:"logMessages"`
}

// TokenBalances token balances
type TokenBalances struct {
	AccountIndex  uint8             `json:"accountIndex"`
	Mint          PublicKey         `json:"mint"`
	Owner         PublicKey         `json:"owner"`
	UITokenAmount TokenAmountResult `json:"uiTokenAmount"`
}

// IsStatusOk in status ok
func (m *TransactionMeta) IsStatusOk() bool {
	return m != nil && m.Err == nil && len(m.LogMessages) > 0
}

// TransactionSignature tx signature
type TransactionSignature struct {
	Err       interface{} `json:"err,omitempty"`
	Memo      string      `json:"memo,omitempty"`
	Signature string      `json:"signature,omitempty"`
	Slot      bin.Uint64  `json:"slot,omitempty"`
}

// GetAccountInfoResult get account info result
type GetAccountInfoResult struct {
	RPCContext
	Value *AccountInfo `json:"value"`
}

// AccountInfo account info
type AccountInfo struct {
	Lamports   bin.Uint64  `json:"lamports"`
	Data       interface{} `json:"data"` // <[string, encoding]|object>
	Owner      PublicKey   `json:"owner"`
	Executable bool        `json:"executable"`
	RentEpoch  bin.Uint64  `json:"rentEpoch"`
}

// GetProgramAccountsOpts get program account options
type GetProgramAccountsOpts struct {
	Commitment CommitmentType `json:"commitment,omitempty"`

	// Filter on accounts, implicit AND between filters
	Filters []RPCFilter `json:"filters,omitempty"`
}

// GetProgramAccountsResult get program accounts result
type GetProgramAccountsResult []*KeyedAccount

// KeyedAccount keyed account
type KeyedAccount struct {
	Pubkey  PublicKey    `json:"pubkey"`
	Account *AccountInfo `json:"account"`
}

// GetSignaturesForAddressOpts get signatures for address opts
type GetSignaturesForAddressOpts struct {
	Limit  uint64 `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
	Until  string `json:"until,omitempty"`
}

// GetSignaturesForAddressResult get signatures for address result
type GetSignaturesForAddressResult []*TransactionSignature

// RPCFilter type
type RPCFilter struct {
	Memcmp   *RPCFilterMemcmp `json:"memcmp,omitempty"`
	DataSize bin.Uint64       `json:"dataSize,omitempty"`
}

// RPCFilterMemcmp type
type RPCFilterMemcmp struct {
	Offset int    `json:"offset"`
	Bytes  Base58 `json:"bytes"`
}

// CommitmentType is the level of commitment desired when querying state.
// https://docs.com/developing/clients/jsonrpc-api#configuring-state-commitment
type CommitmentType string

// commitment contants
const (
	// CommitmentProcessed queries the most recent block which has reached 1 confirmation by the connected node
	CommitmentProcessed = CommitmentType("processed")
	// CommitmentConfirmed queries the most recent block which has reached 1 confirmation by the cluster
	CommitmentConfirmed = CommitmentType("confirmed")
	// CommitmentFinalized queries the most recent block which has been finalized by the cluster
	CommitmentFinalized = CommitmentType("finalized")
)

// ParsedTransaction parsed tx
type ParsedTransaction struct {
	Signatures []Signature `json:"signatures"`
	Message    RPCMessage  `json:"message"`
}

// RPCMessage type
type RPCMessage struct {
	AccountKeys     []*AccountKey       `json:"accountKeys"`
	RecentBlockhash Hash                `json:"recentBlockhash"`
	Instructions    []ParsedInstruction `json:"instructions"`
}

// AccountKey account key
type AccountKey struct {
	PublicKey PublicKey `json:"pubkey"`
	Signer    bool      `json:"signer"`
	Writable  bool      `json:"writable"`
}

// ParsedInstruction parsed instruction
type ParsedInstruction struct {
	Accounts  []PublicKey      `json:"accounts,omitempty"`
	Data      Base58           `json:"data,omitempty"`
	Parsed    *InstructionInfo `json:"parsed,omitempty"`
	Program   string           `json:"program,omitempty"`
	ProgramID PublicKey        `json:"programId"`
}

// InstructionInfo instruction info
type InstructionInfo struct {
	Info            map[string]interface{} `json:"info"`
	InstructionType string                 `json:"type"`
}

// IsParsed is parsed
func (p *ParsedInstruction) IsParsed() bool {
	return p.Parsed != nil
}

// SendTransactionOptions send tx options
type SendTransactionOptions struct {
	SkipPreflight       bool           // disable transaction verification step
	PreflightCommitment CommitmentType // preflight commitment level; default: "finalized"
}

// SimulateTransactionResponse simulate tx responce
type SimulateTransactionResponse struct {
	Err  interface{} `json:"err"`
	Logs []string    `json:"logs"`
}

// GetFeesResult get fees result
type GetFeesResult struct {
	RPCContext
	Value FeeResult `json:"value"`
}

// FeeResult fee result
type FeeResult struct {
	Blockhash            Hash          `json:"blockhash"`
	FeeCalculator        FeeCalculator `json:"feeCalculator"`
	LastValidBlockHeight bin.Uint64    `json:"lastValidBlockHeight"`
	LastValidSlot        bin.Uint64    `json:"lastValidSlot"`
}

// GetSignatureStatusesResult result
type GetSignatureStatusesResult struct {
	RPCContext
	Value []SignatureStatus `json:"value"`
}

// SignatureStatus signature status
type SignatureStatus struct {
	Slot               bin.Uint64  `json:"slot"`
	Confirmations      *bin.Uint64 `json:"confirmations"`
	Err                interface{} `json:"err"`
	ConfirmationStatus *string     `json:"confirmationStatus"`
}
