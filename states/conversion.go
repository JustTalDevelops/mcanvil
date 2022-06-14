package states

import (
	_ "embed"
	"encoding/json"
	"github.com/tidwall/gjson"
	"strings"
)

var (
	//go:embed blocks.json
	blockMappingData []byte
	// javaToBedrockState is a map between a Java state hash and a Bedrock state.
	javaToBedrockState = make(map[blockHash]Block)
	// idToJavaState is a map between a Java state ID and a Java state.
	idToJavaState = make(map[int32]Block)
	// javaStateToID is a map between a Java state and a Java state ID.
	javaStateToID = make(map[blockHash]int32)
)

func init() {
	parsedData := gjson.ParseBytes(blockMappingData)
	parsedData.ForEach(func(key, value gjson.Result) bool {
		bedrockState := parseBedrockBlockJSON(value.String())
		javaState := parseJavaCompressedBlock(key.String())
		id := int32(len(idToJavaState))

		javaToBedrockState[hashBlock(javaState)] = bedrockState
		javaStateToID[hashBlock(javaState)] = id
		idToJavaState[id] = javaState
		return true
	})
}

// IDToJavaState converts a Java state ID to a Java state.
func IDToJavaState(id int32) (Block, bool) {
	state, ok := idToJavaState[id]
	return state, ok
}

// JavaStateToID converts a Java state to a Java state ID.
func JavaStateToID(state Block) (int32, bool) {
	id, ok := javaStateToID[hashBlock(state)]
	return id, ok
}

// ConvertToBedrock converts a Java state to a Bedrock state.
func ConvertToBedrock(state Block) (Block, bool) {
	converted, ok := javaToBedrockState[hashBlock(state)]
	return converted, ok
}

// parseBedrockBlockJSON parses a JSON block state string and returns a Block.
func parseBedrockBlockJSON(data string) Block {
	var state Block
	err := json.Unmarshal([]byte(data), &state)
	if err != nil {
		panic(err)
	}

	// The standard JSON package automatically converts numbers to floats, but we need them as integers.
	for k, v := range state.Properties {
		if v, ok := v.(float64); ok {
			state.Properties[k] = int32(v)
		}
	}
	return state
}

// parseJavaCompressedBlock parses a compressed block state string and returns a Block.
func parseJavaCompressedBlock(compressed string) Block {
	data := strings.Split(strings.TrimSuffix(compressed, "]"), "[")
	name, properties := data[0], map[string]any{}
	if len(data) > 1 {
		for _, entry := range strings.Split(data[1], ",") {
			values := strings.Split(entry, "=")
			properties[values[0]] = values[1]
		}
	}
	return Block{Name: name, Properties: properties}
}
