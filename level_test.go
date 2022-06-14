package mcanvil

import (
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"testing"
)

func TestLevel(t *testing.T) {
	level, err := LoadLevel("Survival")
	if err != nil {
		t.Fatal(err)
	}
	prov, err := mcdb.New("world/", opt.FlateCompression)
	if err != nil {
		t.Fatal(err)
	}
	err = level.WriteBedrock(prov)
	if err != nil {
		t.Fatal(err)
	}
	_ = prov.Close()
}
