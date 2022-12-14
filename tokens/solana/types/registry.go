package types

import (
	"fmt"
)

// InstructionDecoder receives the AccountMeta FOR THAT INSTRUCTION,
// and not the accounts of the *Message object. Resolve with
// CompiledInstruction.ResolveInstructionAccounts(message) beforehand.
type InstructionDecoder func(instructionAccounts []*AccountMeta, data []byte) (interface{}, error)

// InstructionDecoderRegistry instruction decoder registry
var InstructionDecoderRegistry = map[string]InstructionDecoder{}

// RegisterInstructionDecoder register instruction decoder
func RegisterInstructionDecoder(programID PublicKey, decoder InstructionDecoder) {
	p := programID.String()
	if _, found := InstructionDecoderRegistry[p]; found {
		return
	}

	InstructionDecoderRegistry[p] = decoder
}

// DecodeInstruction decode instruction
func DecodeInstruction(programID PublicKey, accounts []*AccountMeta, data []byte) (interface{}, error) {
	p := programID.String()

	decoder, found := InstructionDecoderRegistry[p]
	if !found {
		return nil, fmt.Errorf("unknown programID, cannot find any instruction decoder %q", p)
	}

	return decoder(accounts, data)
}
