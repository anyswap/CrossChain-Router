package cosmosSDK

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gogo/protobuf/proto"
)

const (
	SEI_PREFIX        = "sei"
	MaxSubdenomLength = 44
	MaxHrpLength      = 16
	MaxCreatorLength  = 59 + MaxHrpLength
	ModuleDenomPrefix = "factory"
)

type MsgCreateDenom struct {
	Sender string `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty" yaml:"sender"`
	// subdenom can be up to 44 "alphanumeric" characters long.
	Subdenom string `protobuf:"bytes,2,opt,name=subdenom,proto3" json:"subdenom,omitempty" yaml:"subdenom"`
}

func (m MsgCreateDenom) GetSigners() []types.AccAddress {
	sender, _ := types.AccAddressFromBech32(m.Sender)
	return []types.AccAddress{sender}
}

func (*MsgCreateDenom) ProtoMessage() {}

func (m *MsgCreateDenom) Reset() { *m = MsgCreateDenom{} }

func (m *MsgCreateDenom) String() string { return proto.CompactTextString(m) }

func (m MsgCreateDenom) ValidateBasic() error {
	_, err := types.AccAddressFromBech32(m.Sender)
	if err != nil {
		return err
	}

	_, err = GetTokenDenom(m.Sender, m.Subdenom)
	if err != nil {
		return err
	}

	return nil
}

func GetTokenDenom(creator, subdenom string) (string, error) {
	if len(subdenom) > MaxSubdenomLength {
		return "", errors.New(fmt.Sprintf("subdenom too long, max length is %d bytes", MaxSubdenomLength))
	}
	if len(subdenom) > MaxCreatorLength {
		return "", errors.New(fmt.Sprintf("creator too long, max length is %d bytes", MaxCreatorLength))
	}
	if strings.Contains(creator, "/") {
		return "", errors.New("invalid creator")
	}
	denom := strings.Join([]string{ModuleDenomPrefix, creator, subdenom}, "/")
	return denom, types.ValidateDenom(denom)
}
