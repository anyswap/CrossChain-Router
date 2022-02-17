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

package system

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	bin "github.com/streamingfast/binary"
)

// programID constants
var (
	SystemProgramID      = types.MustPublicKeyFromBase58("11111111111111111111111111111111")
	SysvarRentProgramID  = types.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	SysvarC1ockProgramID = types.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111")
)

// typeID constants
const (
	CreateAccountTypeID          uint32 = iota
	AssignTypeID                        // 1
	TransferTypeID                      // 2
	CreateAccountWithSeedTypeID         // 3
	AdvanceNonceAccountTypeID           // 4
	WithdrawNonceAccountTypeID          // 5
	InitializeNonceAccountTypeID        // 6
	AuthorizeNonceAccountTypeID         // 7
	AllocateTypeID                      // 8
	AllocateWithSeedTypeID              // 9
	AssignWithSeedTypeID                // 10
)

func init() {
	types.RegisterInstructionDecoder(SystemProgramID, registryDecodeInstruction)
}

func registryDecodeInstruction(accounts []*types.AccountMeta, data []byte) (interface{}, error) {
	inst, err := DecodeInstruction(accounts, data)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

// DecodeInstruction decode instruction
func DecodeInstruction(accounts []*types.AccountMeta, data []byte) (*Instruction, error) {
	var inst Instruction
	if err := bin.NewDecoder(data).Decode(&inst); err != nil {
		return nil, fmt.Errorf("unable to decode instruction: %w", err)
	}

	if v, ok := inst.Impl.(types.AccountSettable); ok {
		err := v.SetAccounts(accounts)
		if err != nil {
			return nil, fmt.Errorf("unable to set accounts for instruction: %w", err)
		}
	}

	return &inst, nil
}

// NewCreateAccountInstruction new create account instruction
func NewCreateAccountInstruction(lamports uint64, space uint64, owner, from, to types.PublicKey) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: CreateAccountTypeID,
			Impl: &CreateAccount{
				Lamports: bin.Uint64(lamports),
				Space:    bin.Uint64(space),
				Owner:    owner,
				Accounts: &CreateAccountAccounts{
					From: &types.AccountMeta{PublicKey: from, IsSigner: true, IsWritable: true},
					New:  &types.AccountMeta{PublicKey: to, IsSigner: true, IsWritable: true},
				},
			},
		},
	}
}

// NewTransferSolanaInstruction new transfer solana instruction
func NewTransferSolanaInstruction(from, to types.PublicKey, lamports uint64) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: TransferTypeID,
			Impl: &Transfer{
				Lamports: bin.Uint64(lamports),
				Accounts: &TransferAccounts{
					From: &types.AccountMeta{PublicKey: from, IsSigner: true, IsWritable: true},
					To:   &types.AccountMeta{PublicKey: to, IsSigner: false, IsWritable: true},
				},
			},
		},
	}
}

// Instruction type
type Instruction struct {
	bin.BaseVariant
}

// Accounts get accounts
func (i *Instruction) Accounts() (out []*types.AccountMeta) {
	switch i.TypeID {
	case CreateAccountTypeID:
		accounts := i.Impl.(*CreateAccount).Accounts
		out = []*types.AccountMeta{accounts.From, accounts.New}
	case TransferTypeID:
		accounts := i.Impl.(*Transfer).Accounts
		out = []*types.AccountMeta{accounts.From, accounts.To}
	}
	return out
}

// ProgramID return program id
func (i *Instruction) ProgramID() types.PublicKey {
	return SystemProgramID
}

// Data return data
func (i *Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewEncoder(buf).Encode(i); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// InstructionImplDef default
var InstructionImplDef = bin.NewVariantDefinition(bin.Uint32TypeIDEncoding, []bin.VariantType{
	{Name: "create_account", Type: (*CreateAccount)(nil)},
	{Name: "assign", Type: (*Assign)(nil)},
	{Name: "transfer", Type: (*Transfer)(nil)},
})

// UnmarshalBinary unmarshal binary
func (i *Instruction) UnmarshalBinary(decoder *bin.Decoder) error {
	return i.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionImplDef)
}

// MarshalBinary marshal binary
func (i *Instruction) MarshalBinary(encoder *bin.Encoder) error {
	err := encoder.WriteUint32(i.TypeID, binary.LittleEndian)
	if err != nil {
		return fmt.Errorf("unable to write variant type: %w", err)
	}
	return encoder.Encode(i.Impl)
}

// CreateAccountAccounts type
type CreateAccountAccounts struct {
	From *types.AccountMeta `text:"linear,notype"`
	New  *types.AccountMeta `text:"linear,notype"`
}

// CreateAccount type
type CreateAccount struct {
	Lamports bin.Uint64
	Space    bin.Uint64
	Owner    types.PublicKey
	Accounts *CreateAccountAccounts `bin:"-"`
}

// SetAccounts set accounts
func (i *CreateAccount) SetAccounts(accounts []*types.AccountMeta) error {
	i.Accounts = &CreateAccountAccounts{
		From: accounts[0],
		New:  accounts[1],
	}
	return nil
}

// Assign type
type Assign struct {
	// Prefix with byte 0x01
	Owner types.PublicKey
}

// Transfer type
type Transfer struct {
	// Prefix with byte 0x02
	Lamports bin.Uint64
	Accounts *TransferAccounts `bin:"-"`
}

// TransferAccounts type
type TransferAccounts struct {
	From *types.AccountMeta `text:"linear,notype"`
	To   *types.AccountMeta `text:"linear,notype"`
}

// SetAccounts set accounts
func (i *Transfer) SetAccounts(accounts []*types.AccountMeta) error {
	i.Accounts = &TransferAccounts{
		From: accounts[0],
		To:   accounts[1],
	}
	return nil
}

// CreateAccountWithSeed type
type CreateAccountWithSeed struct {
	// Prefix with byte 0x03
	Base     types.PublicKey
	Seed     types.BorshString
	Lamports bin.Uint64
	Space    bin.Uint64
	Owner    types.PublicKey
}

// AdvanceNonceAccount advance nonce account
type AdvanceNonceAccount struct {
	// Prefix with 0x04
}

// WithdrawNonceAccount withdraw nonce account
type WithdrawNonceAccount struct {
	// Prefix with 0x05
	Lamports bin.Uint64
}

// InitializeNonceAccount initialize nonce account
type InitializeNonceAccount struct {
	// Prefix with 0x06
	AuthorizedAccount types.PublicKey
}

// AuthorizeNonceAccount authorize nonce account
type AuthorizeNonceAccount struct {
	// Prefix with 0x07
	AuthorizeAccount types.PublicKey
}

// Allocate type
type Allocate struct {
	// Prefix with 0x08
	Space bin.Uint64
}

// AllocateWithSeed type
type AllocateWithSeed struct {
	// Prefix with byte 0x09
	Base  types.PublicKey
	Seed  types.BorshString
	Space bin.Uint64
	Owner types.PublicKey
}

// AssignWithSeed type
type AssignWithSeed struct {
	// Prefix with byte 0x0a
	Base  types.PublicKey
	Seed  types.BorshString
	Owner types.PublicKey
}
