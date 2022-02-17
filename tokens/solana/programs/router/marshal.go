package router

import (
	"fmt"
	"reflect"

	bin "github.com/streamingfast/binary"
)

// VariantType type
type VariantType struct {
	ID   SigHash
	Name string
	Type interface{}
}

// VariantDefinition definition
type VariantDefinition struct {
	typeIDToType map[SigHash]reflect.Type
	typeIDToName map[SigHash]string
	typeNameToID map[string]SigHash
}

// NewVariantDefinition creates a variant definition based on the *ordered* provided types.
func NewVariantDefinition(types []VariantType) *VariantDefinition {
	if len(types) < 0 {
		panic("it's not valid to create a variant definition without any types")
	}

	typeCount := len(types)
	out := &VariantDefinition{
		typeIDToType: make(map[SigHash]reflect.Type, typeCount),
		typeIDToName: make(map[SigHash]string, typeCount),
		typeNameToID: make(map[string]SigHash, typeCount),
	}

	for _, typeDef := range types {
		typeID := typeDef.ID

		realType := reflect.TypeOf(typeDef.Type)
		if realType.Kind() != reflect.Ptr {
			panic("it's not valid to create a variant definition with non-pointer type")
		}

		out.typeIDToType[typeID] = realType
		out.typeIDToName[typeID] = typeDef.Name
		out.typeNameToID[typeDef.Name] = typeID
	}

	return out
}

// BaseVariant base variant
type BaseVariant struct {
	TypeID SigHash
	Impl   interface{}
}

// UnmarshalBinaryVariant unmarshal binary variant
func (v *BaseVariant) UnmarshalBinaryVariant(decoder *bin.Decoder, def *VariantDefinition) error {
	num, err := decoder.ReadUint64(bin.BE())
	if err != nil {
		return fmt.Errorf("unable to read type id: %w", err)
	}
	typeID := *new(SigHash).SetUint64(num)

	typeGo := def.typeIDToType[typeID]
	if typeGo == nil {
		return fmt.Errorf("no known type for type %d", typeID)
	}

	if typeGo.Kind() != reflect.Ptr {
		return fmt.Errorf("unable to decode variant type %d: non-pointer type", typeID)
	}

	v.TypeID = typeID
	v.Impl = reflect.New(typeGo.Elem()).Interface()
	if err = decoder.Decode(v.Impl); err != nil {
		return fmt.Errorf("unable to decode variant type %d: %w", typeID, err)
	}

	return nil
}
