package mcanvil

import (
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"testing"
)

func TestLevel(t *testing.T) {
	level, err := LoadLevel("Survival")
	if err != nil {
		t.Fatal(err)
	}
	prov, err := mcdb.Open("world/")
	if err != nil {
		t.Fatal(err)
	}
	err = level.WriteBedrock(prov)
	if err != nil {
		t.Fatal(err)
	}
	_ = prov.Close()
}
