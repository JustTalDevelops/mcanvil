package mcanvil

import (
	"bytes"
	"fmt"
	"github.com/Tnze/go-mc/save/region"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/klauspost/compress/zlib"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"path"
	"regexp"
	"strconv"
)

// regionExp is a regular expression that matches the region file name.
var regionExp = regexp.MustCompile(`^r\.(-?\d+)\.(-?\d+)\.mca$`)

// Region is an extension of the go-mc region implementation.
type Region struct {
	raw *region.Region
	x, z int
}

// LoadRegion creates a new Region from a region file.
func LoadRegion(file string) (*Region, error) {
	stringPos := regionExp.FindStringSubmatch(path.Base(file))
	regionX, err := strconv.Atoi(stringPos[1])
	regionZ, otherErr := strconv.Atoi(stringPos[2])
	if err != nil || otherErr != nil {
		return nil, fmt.Errorf("invalid region file position: %v", stringPos)
	}
	raw, err := region.Open(file)
	if err != nil {
		return nil, err
	}
	return &Region{raw: raw, x: regionX, z: regionZ}, nil
}

// Chunks returns a slice of maps representing encoded chunks in this region.
func (r *Region) Chunks() ([]map[string]interface{}, error) {
	chunks := make([]map[string]interface{}, 0)

	boundX, boundZ := r.x << 5, r.z << 5
	for chunkX := boundX; chunkX < boundX + 32; chunkX++ {
		for chunkZ := boundZ; chunkZ < boundZ + 32; chunkZ++ {
			c, err := r.raw.ReadSector(chunkX & 0x1f, chunkZ & 0x1f)
			if err != nil {
				continue
			}
			z, err := zlib.NewReader(bytes.NewReader(c[1:]))
			if err != nil {
				return nil, err
			}

			var data map[string]map[string]interface{}
			err = nbt.NewDecoderWithEncoding(z, nbt.BigEndian).Decode(&data)
			if err != nil {
				return nil, err
			}
			err = z.Close()
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, data["Level"])
		}
	}
	return chunks, nil
}

// WriteBedrock converts and writes a region file to a Bedrock world provider.
func (r *Region) WriteBedrock(prov *mcdb.Provider) error {
	chunks, err := r.Chunks()
	if err != nil {
		return err
	}
	air, ok := chunk.StateToRuntimeID("minecraft:air", nil)
	if !ok {
		return fmt.Errorf("could not find air runtime id")
	}

	for _, level := range chunks {
		ch := chunk.New(air)
		sections, biomes := level["Sections"].([]interface{}), level["Biomes"].([256]byte)
		chunkX, chunkZ := int(level["xPos"].(int32)), int(level["zPos"].(int32))

		offsetX, offsetZ := chunkX << 4, chunkZ << 4
		for _, v := range sections {
			section := v.(map[string]interface{})
			if section["Add"] != nil {
				return fmt.Errorf("add not nil but isn't implemented")
			}

			blocks, metadata := section["Blocks"].([4096]byte), section["Data"].([2048]byte)
			offsetY := int(section["Y"].(byte)) << 4

			for blockX := 0; blockX < 16; blockX++ {
				for blockY := 0; blockY < 16; blockY++ {
					for blockZ := 0; blockZ < 16; blockZ++ {
						ind := blockY << 8 | blockZ << 4 | blockX

						block := fullBlock(ind, &blocks, &metadata)
						if converted, ok := editionConversion[block]; ok {
							block = converted
						}

						paletteBlock := conversion[block]
						rid, ok := chunk.StateToRuntimeID(paletteBlock.name, paletteBlock.properties)
						if !ok {
							rid = air
						}

						if block.id != 0 {
							ch.SetRuntimeID(uint8(offsetX + blockX), int16(offsetY + blockY), uint8(offsetZ + blockZ), 0, rid)
						}
					}
				}
			}
		}

		// Make our 2D biomes 3D.
		for biomeX := 0; biomeX < 16; biomeX++ {
			for biomeZ := 0; biomeZ < 16; biomeZ++ {
				biomeID := biomes[(biomeX & 15) | (biomeZ & 15)<<4]
				for biomeY := cube.MinY; biomeY <= cube.MaxY; biomeY++ {
					ch.SetBiome(uint8(offsetX+biomeX), int16(biomeY), uint8(offsetZ+biomeZ), uint32(biomeID))
				}
			}
		}

		err = prov.SaveChunk(world.ChunkPos{int32(chunkX), int32(chunkZ)}, ch)
		if err != nil {
			return err
		}
	}
	return nil
}

// fullBlock returns an oldBlock from an index and a slice of blocks and metadata.
func fullBlock(ind int, blocks *[4096]byte, metadata *[2048]byte) oldBlock {
    return oldBlock{
        id:       blocks[ind],
        metadata: uint8(int(metadata[ind>> 1]) >> 4 * (ind & 1) & 15),
    }
}
