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
	// waterloggedBlocks is a set of all waterlogged Java block IDs.
	waterloggedBlocks = make(map[blockHash]struct{})
)

func init() {
	parsedData := gjson.ParseBytes(blockMappingData)
	parsedData.ForEach(func(key, value gjson.Result) bool {
		k, v := key.String(), value.String()
		bedrockState := parseBedrockBlockJSON(v)
		javaState := parseJavaCompressedBlock(k)
		id := int32(len(idToJavaState))
		h := hashBlock(javaState)

		javaToBedrockState[h] = bedrockState
		javaStateToID[h] = id
		idToJavaState[id] = javaState
		if javaState.Name == "minecraft:bubble_column" || javaState.Name == "minecraft:kelp" || strings.Contains(k, "waterlogged=true") || strings.Contains(k, "seagrass") {
			waterloggedBlocks[h] = struct{}{}
		}
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

// ConvertToBedrock converts a Java state to a Bedrock state. The second boolean is true if the state is waterlogged.
func ConvertToBedrock(state Block) (Block, bool, bool) {
	h := hashBlock(state)
	converted, ok := javaToBedrockState[h]
	_, waterlogged := waterloggedBlocks[h]
	return converted, waterlogged, ok
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
