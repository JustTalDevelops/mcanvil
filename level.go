package mcanvil

import (
	"errors"
	"fmt"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/klauspost/compress/gzip"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

// Level represents a Minecraft level for the Anvil format.
type Level struct {
	dat     map[string]any
	regions []*Region
}

// LoadLevel loads a level from the given path.
func LoadLevel(folderPath string) (*Level, error) {
	datPath, regionsPath := path.Join(folderPath, "level.dat"), path.Join(folderPath, "region")
	if _, err := os.Stat(datPath); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("level.dat not found in %s", folderPath)
	}
	if _, err := os.Stat(regionsPath); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("regions not found in %s", folderPath)
	}

	var data map[string]map[string]any
	r, err := os.Open(datPath)
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	err = nbt.NewDecoderWithEncoding(z, nbt.BigEndian).Decode(&data)
	if err != nil {
		return nil, err
	}
	_, _ = z.Close(), r.Close()

	level := &Level{dat: data["Data"]}
	regionFiles, err := ioutil.ReadDir(regionsPath)
	if err != nil {
		return nil, err
	}
	for _, file := range regionFiles {
		if regionExp.MatchString(file.Name()) {
			regionPath := path.Join(regionsPath, file.Name())
			region, err := LoadRegion(regionPath)
			if err != nil {
				return nil, err
			}
			level.regions = append(level.regions, region)
		}
	}
	return level, nil
}

// WriteBedrock converts and writes an anvil level to a Bedrock world provider.
func (l *Level) WriteBedrock(prov *mcdb.DB) error {
	settings := prov.Settings()
	settings.Name = l.dat["LevelName"].(string)
	settings.Time = l.dat["DayTime"].(int64)
	settings.Spawn = cube.Pos{
		int(l.dat["SpawnX"].(int32)),
		int(l.dat["SpawnY"].(int32)),
		int(l.dat["SpawnZ"].(int32)),
	}
	prov.SaveSettings(settings)

	var wg sync.WaitGroup
	for _, region := range l.regions {
		wg.Add(1)

		region := region
		go func() {
			err := region.WriteBedrock(prov)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return nil
}
