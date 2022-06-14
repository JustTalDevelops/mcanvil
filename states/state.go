package states

import (
	"fmt"
	"sort"
	"strings"
	"unsafe"
)

// Block is the Java edition state.
type Block struct {
	// Name is the name of the state.
	Name string `json:"bedrock_identifier"`
	// Properties is the state's properties.
	Properties map[string]any `json:"bedrock_states" nbt:"Properties,omitempty"`
}

// blockHash is a hash of a Block, to be used in map keys.
type blockHash struct {
	name, properties string
}

// hashBlock produces a hash for the Block given.
func hashBlock(block Block) blockHash {
	if block.Properties == nil {
		return blockHash{name: block.Name}
	}
	keys := make([]string, 0, len(block.Properties))
	for k := range block.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		switch v := block.Properties[k].(type) {
		case bool:
			if v {
				b.WriteByte(1)
			} else {
				b.WriteByte(0)
			}
		case uint8:
			b.WriteByte(v)
		case int32:
			a := *(*[4]byte)(unsafe.Pointer(&v))
			b.Write(a[:])
		case string:
			b.WriteString(v)
		default:
			// If block encoding is broken, we want to find out as soon as possible. This saves a lot of time
			// debugging in-game.
			panic(fmt.Sprintf("invalid block property type %T for property %v", v, k))
		}
	}
	return blockHash{name: block.Name, properties: b.String()}
}
