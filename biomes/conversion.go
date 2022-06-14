package biomes

import (
	_ "embed"
	"github.com/tidwall/gjson"
)

var (
	//go:embed biomes.json
	blockMappingData []byte
	// javaToBedrockBiome is a map between a Java biome name and a Bedrock biome ID.
	javaToBedrockBiome = make(map[string]uint32)
)

func init() {
	parsedData := gjson.ParseBytes(blockMappingData)
	parsedData.ForEach(func(key, value gjson.Result) bool {
		javaToBedrockBiome[key.String()] = uint32(value.Get("bedrock_id").Uint())
		return true
	})
}

// ConvertToBedrock converts a Java biome name to a Bedrock biome ID.
func ConvertToBedrock(name string) (uint32, bool) {
	if name == "minecraft:the_void" {
		name = "minecraft:ocean" // The void biome doesn't exist in Bedrock, default to ocean.
	}
	converted, ok := javaToBedrockBiome[name]
	return converted, ok
}
