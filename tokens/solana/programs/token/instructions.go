package token

import (
	"bytes"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	bin "github.com/streamingfast/binary"
)

// programID contants
var (
	TokenProgramID = types.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
)

// typeID constants
const (
	InitializeMintTypeID uint32 = iota
	TransferTypeID              = 3 // 3
	SetAuthorityTypeID          = 6 // 6
	MintToTypeID                = 7 // 7
	BurnTypeID                  = 8 // 8
	CloseAccountTypeID          = 9 // 9
)

func init() {
	types.RegisterInstructionDecoder(TokenProgramID, registryDecodeInstruction)
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

// InstructionDefVariant default variant
var InstructionDefVariant = bin.NewVariantDefinition(bin.Uint8TypeIDEncoding, []bin.VariantType{
	{Name: "initialize_mint", Type: (*InitializeMint)(nil)},
	{Name: "initialize_account", Type: (*InitializeAccount)(nil)},
	{Name: "InitializeMultisig", Type: (*InitializeMultisig)(nil)},
	{Name: "Transfer", Type: (*Transfer)(nil)},
	{Name: "Approve", Type: (*Approve)(nil)},
	{Name: "Revoke", Type: (*Revoke)(nil)},
	{Name: "SetAuthority", Type: (*SetAuthority)(nil)},
	{Name: "MintTo", Type: (*MintTo)(nil)},
	{Name: "Burn", Type: (*Burn)(nil)},
	{Name: "CloseAccount", Type: (*CloseAccount)(nil)},
	{Name: "FreezeAccount", Type: (*FreezeAccount)(nil)},
	{Name: "ThawAccount", Type: (*ThawAccount)(nil)},
	{Name: "TransferChecked", Type: (*TransferChecked)(nil)},
	{Name: "ApproveChecked", Type: (*ApproveChecked)(nil)},
	{Name: "MintToChecked", Type: (*MintToChecked)(nil)},
	{Name: "BurnChecked", Type: (*BurnChecked)(nil)},
})

// Instruction type
type Instruction struct {
	bin.BaseVariant
}

// Accounts get accounts
func (i *Instruction) Accounts() (out []*types.AccountMeta) {
	switch i.TypeID {
	case InitializeMintTypeID:
		accounts := i.Impl.(*InitializeMint).Accounts
		out = []*types.AccountMeta{
			accounts.Mint,
			accounts.RentProgram,
		}
	case TransferTypeID:
		accounts := i.Impl.(*Transfer).Accounts
		out = []*types.AccountMeta{
			accounts.Source,
			accounts.Destination,
			accounts.From,
		}
	case MintToTypeID:
		accounts := i.Impl.(*MintTo).Accounts
		out = []*types.AccountMeta{
			accounts.Mint,
			accounts.Account,
			accounts.Minter,
		}
	case BurnTypeID:
		accounts := i.Impl.(*Burn).Accounts
		out = []*types.AccountMeta{
			accounts.Account,
			accounts.Mint,
			accounts.Owner,
		}
	case SetAuthorityTypeID:
		accounts := i.Impl.(*SetAuthority).Accounts
		out = []*types.AccountMeta{
			accounts.Account,
			accounts.CurrentAuthority,
		}
	case CloseAccountTypeID:
		accounts := i.Impl.(*CloseAccount).Accounts
		out = []*types.AccountMeta{
			accounts.Account,
			accounts.Desination,
			accounts.Owner,
		}
	}

	return
}

// ProgramID get proram ID
func (i *Instruction) ProgramID() types.PublicKey {
	return TokenProgramID
}

// Data get data
func (i *Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewEncoder(buf).Encode(i); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary unmarshal binary
func (i *Instruction) UnmarshalBinary(decoder *bin.Decoder) (err error) {
	return i.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionDefVariant)
}

// MarshalBinary marshal binary
func (i *Instruction) MarshalBinary(encoder *bin.Encoder) error {
	err := encoder.WriteUint8(uint8(i.TypeID))
	if err != nil {
		return fmt.Errorf("unable to write variant type: %w", err)
	}
	return encoder.Encode(i.Impl)
}

// InitializeMultisigAccounts type
type InitializeMultisigAccounts struct {
}

// InitializeMultisig type
type InitializeMultisig struct {
	Accounts *InitializeMultisigAccounts
}

// InitializeMintAccounts type
type InitializeMintAccounts struct {
	Mint        *types.AccountMeta
	RentProgram *types.AccountMeta
	///   0. `[writable]` The mint to initialize.
	///   1. `[]` Rent sysvar
}

// InitializeMint type
type InitializeMint struct {
	/// Number of base 10 digits to the right of the decimal place.
	Decimals uint8
	/// The authority/multisignature to mint tokens.
	MintAuthority types.PublicKey
	/// The freeze authority/multisignature of the mint.
	FreezeAuthority *types.PublicKey        `bin:"optional"`
	Accounts        *InitializeMintAccounts `bin:"-"`
}

// NewInitializeMintInstruction new InitializeMint instruction
func NewInitializeMintInstruction(
	decimals uint8,
	mint types.PublicKey,
	mintAuthority types.PublicKey,
	freezeAuthority *types.PublicKey,
	rentProgram types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: InitializeMintTypeID,
			Impl: &InitializeMint{
				Decimals:        decimals,
				MintAuthority:   mintAuthority,
				FreezeAuthority: freezeAuthority,
				Accounts: &InitializeMintAccounts{
					Mint:        &types.AccountMeta{PublicKey: mint, IsWritable: true},
					RentProgram: &types.AccountMeta{PublicKey: rentProgram},
				},
			},
		},
	}
}

// SetAccounts set accounts
func (i *InitializeMint) SetAccounts(accounts []*types.AccountMeta) error {
	i.Accounts = &InitializeMintAccounts{
		Mint:        accounts[0],
		RentProgram: accounts[1],
	}
	return nil
}

// TransferAccounts type
type TransferAccounts struct {
	Source      *types.AccountMeta
	Destination *types.AccountMeta
	From        *types.AccountMeta
}

// Transfer transfer type
type Transfer struct {
	Amount   uint64
	Accounts *TransferAccounts
}

// NewTransferInstruction new Transfer instruction
func NewTransferInstruction(
	amount uint64,
	source types.PublicKey,
	destination types.PublicKey,
	from types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: TransferTypeID,
			Impl: &Transfer{
				Amount: amount,
				Accounts: &TransferAccounts{
					Source:      &types.AccountMeta{PublicKey: source, IsWritable: true},
					Destination: &types.AccountMeta{PublicKey: destination, IsWritable: true},
					From:        &types.AccountMeta{PublicKey: from, IsSigner: true},
				},
			},
		},
	}
}

// ApproveAccounts type
type ApproveAccounts struct {
}

// Approve type
type Approve struct {
	Accounts *ApproveAccounts
}

// RevokeAccounts type
type RevokeAccounts struct {
}

// Revoke type
type Revoke struct {
	Accounts *RevokeAccounts
}

// SetAuthorityAccounts type
type SetAuthorityAccounts struct {
	Account          *types.AccountMeta
	CurrentAuthority *types.AccountMeta
}

// AuthorityType type
type AuthorityType byte

// AuthorityType constants
const (
	MintTokensAuthorityType AuthorityType = iota
	FreezeAccountAuthorityType
	AccountOwnerAuthorityType
	CloseAccountAuthorityType
)

// SetAuthority type
type SetAuthority struct {
	AuthorityType   AuthorityType
	NewAuthorityKey types.PublicKey       `bin:"optional"`
	Accounts        *SetAuthorityAccounts `bin:"-"`
}

// NewSetAuthorityInstruction new SetAuthority instruction
func NewSetAuthorityInstruction(
	account types.PublicKey, // either a spl_token::mint or spl_token::account
	newAuthority types.PublicKey,
	authorityType AuthorityType,
	currentAuthority types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: SetAuthorityTypeID,
			Impl: &SetAuthority{
				AuthorityType:   authorityType,
				NewAuthorityKey: newAuthority,
				Accounts: &SetAuthorityAccounts{
					Account:          &types.AccountMeta{PublicKey: account, IsWritable: true},
					CurrentAuthority: &types.AccountMeta{PublicKey: currentAuthority, IsSigner: true},
				},
			},
		},
	}
}

// MintToAccounts type
type MintToAccounts struct {
	Mint    *types.AccountMeta
	Account *types.AccountMeta
	Minter  *types.AccountMeta
}

// MintTo type
type MintTo struct {
	Amount   uint64
	Accounts *MintToAccounts
}

// NewMintToInstruction new MintTo instruction
func NewMintToInstruction(
	amount uint64,
	mint types.PublicKey,
	account types.PublicKey,
	minter types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: MintToTypeID,
			Impl: &MintTo{
				Amount: amount,
				Accounts: &MintToAccounts{
					Mint:    &types.AccountMeta{PublicKey: mint, IsWritable: true},
					Account: &types.AccountMeta{PublicKey: account, IsWritable: true},
					Minter:  &types.AccountMeta{PublicKey: minter, IsSigner: true},
				},
			},
		},
	}
}

// BurnAccounts type
type BurnAccounts struct {
	Account *types.AccountMeta
	Mint    *types.AccountMeta
	Owner   *types.AccountMeta
}

// Burn type
type Burn struct {
	Amount   uint64
	Accounts *BurnAccounts
}

// NewBurnToInstruction new MintTo instruction
func NewBurnToInstruction(
	amount uint64,
	account types.PublicKey,
	mint types.PublicKey,
	owner types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: BurnTypeID,
			Impl: &Burn{
				Amount: amount,
				Accounts: &BurnAccounts{
					Account: &types.AccountMeta{PublicKey: account, IsWritable: true},
					Mint:    &types.AccountMeta{PublicKey: mint, IsWritable: true},
					Owner:   &types.AccountMeta{PublicKey: owner, IsSigner: true},
				},
			},
		},
	}
}

// CloseAccountAccounts type
type CloseAccountAccounts struct {
	Account    *types.AccountMeta
	Desination *types.AccountMeta
	Owner      *types.AccountMeta
}

// CloseAccount type
type CloseAccount struct {
	Accounts *CloseAccountAccounts
}

// NewCloseAccountInstruction new CloseAccount instruction
func NewCloseAccountInstruction(
	account types.PublicKey,
	destination types.PublicKey,
	owner types.PublicKey,
) *Instruction {
	return &Instruction{
		BaseVariant: bin.BaseVariant{
			TypeID: CloseAccountTypeID,
			Impl: &CloseAccount{
				Accounts: &CloseAccountAccounts{
					Account:    &types.AccountMeta{PublicKey: account, IsWritable: true},
					Desination: &types.AccountMeta{PublicKey: destination, IsWritable: true},
					Owner:      &types.AccountMeta{PublicKey: owner, IsSigner: true},
				},
			},
		},
	}
}

// FreezeAccountAccounts type
type FreezeAccountAccounts struct {
}

// FreezeAccount type
type FreezeAccount struct {
	Accounts *FreezeAccountAccounts
}

// ThawAccountAccounts type
type ThawAccountAccounts struct {
}

// ThawAccount type
type ThawAccount struct {
	Accounts *ThawAccountAccounts
}

// TransferCheckedAccounts type
type TransferCheckedAccounts struct {
}

// TransferChecked type
type TransferChecked struct {
	Accounts *TransferCheckedAccounts
}

// ApproveCheckedAccounts type
type ApproveCheckedAccounts struct {
}

// ApproveChecked type
type ApproveChecked struct {
	Accounts *ApproveCheckedAccounts
}

// MintToCheckedAccounts type
type MintToCheckedAccounts struct {
}

// MintToChecked type
type MintToChecked struct {
	Accounts *MintToCheckedAccounts
}

// BurnCheckedAccounts type
type BurnCheckedAccounts struct {
}

// BurnChecked type
type BurnChecked struct {
	Accounts *BurnCheckedAccounts
}

// InitializeAccountAccounts type
type InitializeAccountAccounts struct {
	Account    *types.AccountMeta `text:"linear,notype"`
	Mint       *types.AccountMeta `text:"linear,notype"`
	Owner      *types.AccountMeta `text:"linear,notype"`
	RentSysvar *types.AccountMeta `text:"linear,notype"`
}

// InitializeAccount type
type InitializeAccount struct {
	Accounts *InitializeAccountAccounts `bin:"-"`
}

// SetAccounts set accounts
func (i *InitializeAccount) SetAccounts(accounts []*types.AccountMeta) error {
	i.Accounts = &InitializeAccountAccounts{
		Account:    accounts[0],
		Mint:       accounts[1],
		Owner:      accounts[2],
		RentSysvar: accounts[3],
	}
	return nil
}
