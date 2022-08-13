package cosmosHub

// Tx tx
type Tx struct {
	Body TxBody `protobuf:"bytes,1,opt,name=body,proto3" json:"body,omitempty"`
}

type TxBody struct {
	// memo is any arbitrary note/comment to be added to the transaction.
	// WARNING: in clients, any publicly exposed text should not be called memo,
	// but should be called `note` instead (see https://github.com/cosmos/cosmos-sdk/issues/9122).
	Memo string `protobuf:"bytes,2,opt,name=memo,proto3" json:"memo,omitempty"`
}
