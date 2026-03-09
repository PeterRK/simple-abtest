package core

import (
	"fmt"
	"testing"
)

func fullBitmap() []byte {
	bitmap := make([]byte, 125)
	for i := 0; i < len(bitmap); i++ {
		bitmap[i] = 0xff
	}
	return bitmap
}

func findKey(seed uint32, begin, end uint64) (string, uint64) {
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("k%d", i)
		slot := Hash64(uint64(seed), []byte(key)) % 100
		if slot >= begin && slot < end {
			return key, slot
		}
	}
	return "", 0
}

func TestGetExpConfigRichSegment(t *testing.T) {
	seed := uint32(1)
	key, _ := findKey(seed, 50, 100)
	if key == "" {
		t.FailNow()
	}

	layer := Layer{
		Name: "layer1",
		Segments: []Segment{
			{
				Seed: 1,
				Groups: []Group{
					{Name: "A", Bitmap: fullBitmap(), Config: "cfgA"},
				},
			},
			{
				Seed: 2,
				Groups: []Group{
					{Name: "B", Bitmap: fullBitmap(), Config: "cfgB"},
				},
			},
		},
		ForceHit: map[string]HitIndex{},
	}
	layer.Segments[0].Range.Begin = 0
	layer.Segments[0].Range.End = 50
	layer.Segments[1].Range.Begin = 50
	layer.Segments[1].Range.End = 100
	exps := []Experiment{
		{
			Seed:   seed,
			Layers: []Layer{layer},
		},
	}

	cfg, tags := GetExpConfig(exps, key, map[string]string{})
	if cfg["layer1"] != "cfgB" {
		t.FailNow()
	}
	if len(tags) != 1 || tags[0] != "layer1:B" {
		t.FailNow()
	}
}

func TestGetExpConfigNaiveAndForceHit(t *testing.T) {
	seg := Segment{
		Seed: 1,
		Groups: []Group{
			{Name: "A", Bitmap: fullBitmap(), Config: "cfgA"},
		},
	}
	layer := Layer{
		Name:     "L1",
		Segments: []Segment{seg},
		ForceHit: map[string]HitIndex{},
	}
	exps := []Experiment{
		{
			Layers: []Layer{layer},
		},
	}

	cfg, tags := GetExpConfig(exps, "u1", map[string]string{})
	if cfg["L1"] != "cfgA" || len(tags) != 1 || tags[0] != "L1:A" {
		t.FailNow()
	}

	layer.Segments[0].Groups = append(layer.Segments[0].Groups, Group{
		Name:   "F",
		Bitmap: fullBitmap(),
		Config: "cfgForce",
	})
	layer.ForceHit["u2"] = HitIndex{Seg: 0, Grp: 1}
	exps[0].Layers[0] = layer

	cfg, tags = GetExpConfig(exps, "u2", map[string]string{})
	if cfg["L1"] != "cfgForce" || len(tags) != 1 || tags[0] != "L1:F" {
		t.FailNow()
	}
}

func TestGetExpConfigWithFilter(t *testing.T) {
	filterCfg := []byte(`[{"op":6,"dtype":1,"key":"country","s":"CN"}]`)
	nodes, err := ParseExpr(filterCfg)
	if err != nil {
		t.FailNow()
	}

	layer1 := Layer{
		Name: "L1",
		Segments: []Segment{
			{
				Seed: 1,
				Groups: []Group{
					{Name: "G1", Bitmap: fullBitmap(), Config: "cfgCN"},
				},
			},
		},
		ForceHit: map[string]HitIndex{},
	}
	layer2 := Layer{
		Name: "L2",
		Segments: []Segment{
			{
				Seed: 1,
				Groups: []Group{
					{Name: "G2", Bitmap: fullBitmap(), Config: "cfgAll"},
				},
			},
		},
		ForceHit: map[string]HitIndex{},
	}

	exps := []Experiment{
		{
			Filter: nodes,
			Layers: []Layer{layer1},
		},
		{
			Layers: []Layer{layer2},
		},
	}

	cfg, _ := GetExpConfig(exps, "user", map[string]string{"country": "CN"})
	if cfg["L1"] != "cfgCN" || cfg["L2"] != "cfgAll" {
		t.FailNow()
	}

	cfg, _ = GetExpConfig(exps, "user", map[string]string{"country": "US"})
	if _, ok := cfg["L1"]; ok {
		t.FailNow()
	}
	if cfg["L2"] != "cfgAll" {
		t.FailNow()
	}
}
