package mcanvil

import (
	"bytes"
	"fmt"
	"github.com/Tnze/go-mc/save/region"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/justtaldevelops/mcanvil/biomes"
	"github.com/justtaldevelops/mcanvil/column"
	"github.com/justtaldevelops/mcanvil/states"
	"github.com/klauspost/compress/zlib"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"math/bits"
	"path"
	"regexp"
	"strconv"
)

// Chunk represents a 16x16x16 chunk of blocks. In Java, these are known as columns.
type Chunk struct {
	DataVersion   int32
	XPos          int32            `nbt:"xPos"`
	YPos          int32            `nbt:"yPos"`
	ZPos          int32            `nbt:"zPos"`
	BlockEntities []map[string]any `nbt:"block_entities"`
	Structures    map[string]any   `nbt:"structures"`
	Heightmaps    struct {
		MotionBlocking         any `nbt:"MOTION_BLOCKING"`
		MotionBlockingNoLeaves any `nbt:"MOTION_BLOCKING_NO_LEAVES"`
		OceanFloor             any `nbt:"OCEAN_FLOOR"`
		OceanFloorWg           any `nbt:"OCEAN_FLOOR_WG"`
		WorldSurface           any `nbt:"WORLD_SURFACE"`
		WorldSurfaceWg         any `nbt:"WORLD_SURFACE_WG"`
	}
	Sections       []SubChunk `nbt:"sections"`
	Lights         any        `nbt:"Lights"`
	Entities       any        `nbt:"entities"`
	BlockTicks     any        `nbt:"block_ticks"`
	FluidTicks     any        `nbt:"fluid_ticks"`
	PostProcessing any
	CarvingMasks   any
	InhabitedTime  int64
	IsLightOn      byte `nbt:"isLightOn"`
	LastUpdate     int64
	Status         string
}

// SubChunk represents a 16x16 sub-chunk of a chunk. In Java, these are known as chunks or sections.
type SubChunk struct {
	Y           byte
	BlockStates struct {
		Palette []states.Block `nbt:"palette"`
		Data    []int64        `nbt:"data,omitempty"`
	} `nbt:"block_states"`
	Biomes struct {
		Palette []string `nbt:"palette"`
		Data    []int64  `nbt:"data,omitempty"`
	} `nbt:"biomes"`
	SkyLight   any `nbt:"SkyLight,omitempty"`
	BlockLight any `nbt:"BlockLight,omitempty"`
}

// regionExp is a regular expression that matches the region file name.
var regionExp = regexp.MustCompile(`^r\.(-?\d+)\.(-?\d+)\.mca$`)

// Region is an extension of the go-mc region implementation.
type Region struct {
	raw  *region.Region
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
func (r *Region) Chunks() ([]Chunk, error) {
	chunks := make([]Chunk, 0, 1024)
	boundX, boundZ := r.x<<5, r.z<<5
	for chunkX := boundX; chunkX < boundX+32; chunkX++ {
		for chunkZ := boundZ; chunkZ < boundZ+32; chunkZ++ {
			c, err := r.raw.ReadSector(chunkX&0x1f, chunkZ&0x1f)
			if err != nil {
				continue
			}
			z, err := zlib.NewReader(bytes.NewReader(c[1:]))
			if err != nil {
				return nil, err
			}

			var data Chunk
			err = nbt.NewDecoderWithEncoding(z, nbt.BigEndian).Decode(&data)
			if err != nil {
				return nil, err
			}
			err = z.Close()
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, data)
		}
	}
	return chunks, nil
}

// WriteBedrock converts and writes a region file to a Bedrock world provider.
func (r *Region) WriteBedrock(prov *mcdb.Provider) error {
	chunks, err := r.Chunks()
	if err != nil {
		return fmt.Errorf("could not load chunk structures: %v", err)
	}
	airRuntimeID, ok := chunk.StateToRuntimeID("minecraft:air", nil)
	if !ok {
		return fmt.Errorf("could not find air runtime id")
	}
	waterRuntimeID, ok := chunk.StateToRuntimeID("minecraft:water", map[string]any{"liquid_depth": int32(0)})
	if !ok {
		return fmt.Errorf("could not find water runtime id")
	}

	for _, c := range chunks {
		if c.Status != "full" {
			// Don't convert incomplete chunks, to be consistent with Bedrock.
			continue
		}
		ch := chunk.New(airRuntimeID, world.Overworld.Range())
		offsetX, offsetZ := c.XPos<<4, c.ZPos<<4
		for _, s := range c.Sections {
			rawBlockPalette := make([]int32, 0, len(s.BlockStates.Palette))
			for _, state := range s.BlockStates.Palette {
				id, ok := states.JavaStateToID(state)
				if !ok {
					return fmt.Errorf("could not find block id for state %v", state)
				}
				rawBlockPalette = append(rawBlockPalette, id)
			}

			n := int32(bits.Len(uint(len(rawBlockPalette) - 1)))
			p, t := column.Palette(column.NewGlobalPalette()), column.ChunkPaletteType()
			if n == 0 {
				p = column.NewSingletonPalette(rawBlockPalette[0])
			} else if n <= t.MinimumBitsPerEntry {
				p, n = column.NewFilledListPalette(4, rawBlockPalette), 4
			} else if n <= t.MaximumBitsPerEntry {
				p = column.NewFilledMapPalette(n, rawBlockPalette)
			}

			storage := column.NewEmptyBitStorage(n, 4096)
			if len(s.BlockStates.Data) > 0 {
				storage, err = column.NewFilledBitStorage(n, storage.Capacity(), s.BlockStates.Data)
				if err != nil {
					return err
				}
			}

			ind := int8(s.Y) - int8(c.YPos)
			offsetY := ch.SubY(int16(ind))

			sub := ch.Sub()[ind]
			dataPalette := column.NewFilledDataPalette(t, n, p, storage)
			for blockX := int32(0); blockX < 16; blockX++ {
				for blockY := int32(0); blockY < 16; blockY++ {
					for blockZ := int32(0); blockZ < 16; blockZ++ {
						id, err := dataPalette.Get(column.BlockPos{blockX, blockY, blockZ})
						if err != nil {
							return err
						}
						if id == 0 {
							// Skip airRuntimeID.
							continue
						}

						javaState, ok := states.IDToJavaState(id)
						if !ok {
							return fmt.Errorf("could not find state for id: %d", id)
						}

						bedrockState, waterlogged, ok := states.ConvertToBedrock(javaState)
						if !ok {
							return fmt.Errorf("could not find bedrock state for java state: %v", javaState)
						}
						rid, ok := chunk.StateToRuntimeID(bedrockState.Name, bedrockState.Properties)
						if !ok {
							return fmt.Errorf("could not find bedrock runtime id for state: %v", bedrockState)
						}

						sub.SetBlock(byte(blockX), byte(blockY), byte(blockZ), 0, rid)
						if waterlogged {
							sub.SetBlock(byte(blockX), byte(blockY), byte(blockZ), 1, waterRuntimeID)
						}
					}
				}
			}

			rawBiomePalette := make([]int32, 0, len(s.Biomes.Palette))
			for _, name := range s.Biomes.Palette {
				id, ok := biomes.JavaNameToID(name)
				if !ok {
					return fmt.Errorf("could not find biome id for name: %v", name)
				}
				rawBiomePalette = append(rawBiomePalette, id)
			}

			n = int32(bits.Len(uint(len(rawBiomePalette) - 1)))
			p, t = column.Palette(column.NewGlobalPalette()), column.BiomePaletteType()
			if n == 0 {
				p = column.NewSingletonPalette(rawBiomePalette[0])
			} else if n <= t.MaximumBitsPerEntry {
				p = column.NewFilledListPalette(n, rawBiomePalette)
			}

			storage = column.NewEmptyBitStorage(n, 64)
			if len(s.Biomes.Data) > 0 {
				storage, err = column.NewFilledBitStorage(n, storage.Capacity(), s.Biomes.Data)
				if err != nil {
					return err
				}
			}

			for i := int32(0); i < storage.Capacity(); i++ {
				paletteID, err := storage.Get(i)
				if err != nil {
					return err
				}
				id := p.IDToState(paletteID)
				name, ok := biomes.IDToJavaName(id)
				if !ok {
					return fmt.Errorf("could not find biome name for id: %d", id)
				}
				bedrockID, ok := biomes.ConvertToBedrock(name)
				if !ok {
					return fmt.Errorf("could not find bedrock id for biome name: %v", name)
				}

				baseX := i & 3
				baseY := (i >> 4) & 3
				baseZ := (i >> 2) & 3

				for blockX := baseX << 2; blockX < (baseX<<2)+4; blockX++ {
					for blockZ := baseZ << 2; blockZ < (baseZ<<2)+4; blockZ++ {
						for blockY := baseY << 2; blockY < (baseY<<2)+4; blockY++ {
							ch.SetBiome(byte(offsetX+blockX), int16(blockY)+offsetY, byte(offsetZ+blockZ), bedrockID)
						}
					}
				}
			}
		}

		ch.Compact()

		err = prov.SaveChunk(world.ChunkPos{c.XPos, c.ZPos}, ch, world.Overworld)
		if err != nil {
			return err
		}
	}
	return nil
}
