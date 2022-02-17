package types

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sort"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/mr-tron/base58"
	bin "github.com/streamingfast/binary"
)

// TransactionInstruction interface
type TransactionInstruction interface {
	Accounts() []*AccountMeta // returns the list of accounts the instructions requires
	ProgramID() PublicKey     // the programID the instruction acts on
	Data() ([]byte, error)    // the binary encoded instructions
}

// TransactionOption interface
type TransactionOption interface {
	apply(opts *transactionOptions)
}

type transactionOptions struct {
	payer PublicKey
}

type transactionOptionFunc func(opts *transactionOptions)

func (f transactionOptionFunc) apply(opts *transactionOptions) {
	f(opts)
}

// TransactionPayer tx payer
func TransactionPayer(payer PublicKey) TransactionOption {
	return transactionOptionFunc(func(opts *transactionOptions) { opts.payer = payer })
}

// NewTransaction new tx
func NewTransaction(instructions []TransactionInstruction, blockHash Hash, opts ...TransactionOption) (*Transaction, error) {
	if len(instructions) == 0 {
		return nil, fmt.Errorf("requires at-least one instruction to create a transaction")
	}

	options := transactionOptions{}
	for _, opt := range opts {
		opt.apply(&options)
	}

	feePayer := options.payer
	if feePayer.IsZero() {
		found := false
		for _, act := range instructions[0].Accounts() {
			if act.IsSigner {
				feePayer = act.PublicKey
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("cannot determine fee payer. You can either pass the fee payer vai the 'TransactionWithInstructions' option parameter or it fallback to the first instruction's first signer")
		}
	}

	programIDs := []PublicKey{}
	accounts := []*AccountMeta{}
	for _, instruction := range instructions {
		accounts = append(accounts, instruction.Accounts()...)
		programIDs = append(programIDs, instruction.ProgramID())
	}

	// Add programID to the account list
	for _, programID := range programIDs {
		accounts = append(accounts, &AccountMeta{
			PublicKey:  programID,
			IsSigner:   false,
			IsWritable: false,
		})
	}

	// Sort. Prioritizing first by signer, then by writable
	sort.SliceStable(accounts, func(i, j int) bool {
		return accounts[i].less(accounts[j])
	})

	uniqAccountsMap := make(map[PublicKey]int, 5)
	uniqAccounts := make([]*AccountMeta, 0, 5)
	for _, acc := range accounts {
		if index, found := uniqAccountsMap[acc.PublicKey]; found {
			uniqAccounts[index].IsWritable = uniqAccounts[index].IsWritable || acc.IsWritable
			continue
		}
		uniqAccounts = append(uniqAccounts, acc)
		uniqAccountsMap[acc.PublicKey] = len(uniqAccounts) - 1
	}

	log.Debug("unique account sorted", "account_count", len(uniqAccounts))
	feePayerIndex := -1
	if idx, exist := uniqAccountsMap[feePayer]; exist {
		feePayerIndex = idx
	}
	log.Debug("current fee payer index", "fee_payer_index", feePayerIndex)

	finalAccounts := uniqAccounts
	// Move fee payer to the front
	if feePayerIndex != 0 {
		if feePayerIndex < 0 {
			// fee payer is not part of accounts we want to add it
			finalAccounts = make([]*AccountMeta, len(uniqAccounts)+1)
			copy(finalAccounts[1:], uniqAccounts)
		} else {
			copy(finalAccounts[1:], uniqAccounts[0:feePayerIndex])
		}
	}
	finalAccounts[0] = &AccountMeta{
		PublicKey:  feePayer,
		IsSigner:   true,
		IsWritable: true,
	}

	message := Message{
		RecentBlockhash: blockHash,
	}
	accountKeyIndex := make(map[PublicKey]uint8, len(finalAccounts))
	for idx, acc := range finalAccounts {
		message.AccountKeys = append(message.AccountKeys, acc.PublicKey)
		accountKeyIndex[acc.PublicKey] = uint8(idx)
		if acc.IsSigner {
			message.Header.NumRequiredSignatures++
			if !acc.IsWritable {
				message.Header.NumReadonlySignedAccounts++
			}
			continue
		}

		if !acc.IsWritable {
			message.Header.NumReadonlyUnsignedAccounts++
		}
	}
	log.Debug("message header compiled",
		"num_required_signatures", message.Header.NumRequiredSignatures,
		"num_readonly_signed_accounts", message.Header.NumReadonlySignedAccounts,
		"num_readonly_unsigned_accounts", message.Header.NumReadonlyUnsignedAccounts,
	)

	for trxIdx, instruction := range instructions {
		accounts = instruction.Accounts()
		accountIndex := make(Uint8Arr, len(accounts))
		for idx, acc := range accounts {
			accountIndex[idx] = accountKeyIndex[acc.PublicKey]
		}
		data, err := instruction.Data()
		if err != nil {
			return nil, fmt.Errorf("unable to encode instructions [%d]: %w", trxIdx, err)
		}
		message.Instructions = append(message.Instructions, CompiledInstruction{
			ProgramIDIndex: accountKeyIndex[instruction.ProgramID()],
			AccountCount:   bin.Varuint16(uint16(len(accountIndex))),
			Accounts:       accountIndex,
			DataLength:     bin.Varuint16(uint16(len(data))),
			Data:           data,
		})
	}

	return &Transaction{
		Message: message,
	}, nil
}

type privateKeyGetter func(key PublicKey) *PrivateKey

// Sign sign with private key
func (t *Transaction) Sign(getter privateKeyGetter) (out []Signature, err error) {
	buf := new(bytes.Buffer)
	if err = bin.NewEncoder(buf).Encode(t.Message); err != nil {
		return nil, fmt.Errorf("unable to encode message for signing: %w", err)
	}
	messageCnt := buf.Bytes()

	signerKeys := t.Message.SignerKeys()

	for _, key := range signerKeys {
		privateKey := getter(key)
		if privateKey == nil {
			return nil, fmt.Errorf("signer key %q not found. Ensure all the signer keys are in the vault", key.String())
		}

		s, err := privateKey.Sign(messageCnt)
		if err != nil {
			return nil, fmt.Errorf("failed to signed with key %q: %w", key.String(), err)
		}

		t.Signatures = append(t.Signatures, s)
	}
	return t.Signatures, nil
}

// DecodeTransaction decode transaction
func DecodeTransaction(encodedStr string, encoding string) (*Transaction, error) {
	var data []byte
	var err error
	switch encoding {
	case "base58":
		data, err = base58.Decode(encodedStr)
	case "base64":
		data, err = base64.StdEncoding.DecodeString(encodedStr)
	default:
		data = []byte(encodedStr)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to decode string: %w", err)
	}
	var tx Transaction
	err = bin.NewDecoder(data).Decode(&tx)
	if err != nil {
		return nil, fmt.Errorf("unable to decode transaction: %w", err)
	}
	return &tx, nil
}
