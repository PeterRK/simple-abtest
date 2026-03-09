// Package core contains the in-memory experiment model and dispatch logic.
package core

import (
	"strings"

	"github.com/peterrk/simple-abtest/utils"
)

// Group represents a single traffic group within a segment.
// Bitmap encodes the 0-999 slots that belong to this group.
type Group struct {
	Name   string `json:"name,omitempty"`
	Bitmap []byte `json:"bm,omitempty"` //125
	Config string `json:"cfg,omitempty"`
}

// Segment represents a contiguous traffic range and its groups.
// Seed is used to hash keys into the 0-999 bitmap space.
type Segment struct {
	Range struct {
		Begin uint32 `json:"a,omitempty"`
		End   uint32 `json:"b,omitempty"`
	} `json:"r,omitempty"`
	Seed   uint32  `json:"seed,omitempty"`
	Groups []Group `json:"grp,omitempty"`
}

type HitIndex struct {
	Seg uint32 `json:"s,omitempty"`
	Grp uint32 `json:"g,omitempty"`
}

// Layer is a logical experiment layer composed of multiple segments.
// ForceHit can override the normal dispatch result for specific keys.
type Layer struct {
	Name     string              `json:"name,omitempty"`
	Segments []Segment           `json:"seg,omitempty"`
	ForceHit map[string]HitIndex `json:"force_hit,omitempty"`
}

// Experiment describes a full experiment including an optional filter,
// a global seed for slotting and one or more layers.
type Experiment struct {
	Filter []ExprNode `json:"filter,omitempty"`
	Seed   uint32     `json:"seed,omitempty"`
	Layers []Layer    `json:"lyr,omitempty"`
}

// locate returns the first group whose bitmap contains the hashed slot of key.
func (s *Segment) locate(key string) *Group {
	slot := Hash64(uint64(s.Seed), utils.UnsafeStringToBytes(key)) % 1000
	blk, sft := slot>>3, slot&7
	m := byte(1) << sft
	for i := 0; i < len(s.Groups); i++ {
		if (s.Groups[i].Bitmap[blk] & m) != 0 {
			return &s.Groups[i]
		}
	}
	return nil
}

// GetExpConfig evaluates experiments for a given key and context and returns
// a per-layer configuration map and a list of debug tags.
func GetExpConfig(exps []Experiment, key string, ctx map[string]string) (config map[string]string, tags []string) {
	config = make(map[string]string)
	mark := func(l *Layer, g *Group) {
		config[l.Name] = g.Config
		tags = append(tags, strings.Join([]string{l.Name, g.Name}, ":"))
	}

	for i := 0; i < len(exps); i++ {
		exp := &exps[i]
		if !EvalExpr(exp.Filter, ctx) {
			continue
		}
		if len(exp.Layers) == 1 && len(exp.Layers[0].Segments) == 1 {
			// naive experiment
			layer := &exp.Layers[0]
			if idx, got := layer.ForceHit[key]; got {
				mark(layer, &layer.Segments[idx.Seg].Groups[idx.Grp])
				continue
			}
			if g := layer.Segments[0].locate(key); g != nil {
				mark(layer, g)
			}
			continue
		}

		// rich experiment
		slot := uint32(Hash64(uint64(exp.Seed), utils.UnsafeStringToBytes(key)) % 100)
		for j := 0; j < len(exp.Layers); j++ {
			layer := &exp.Layers[j]
			if idx, got := layer.ForceHit[key]; got {
				mark(layer, &layer.Segments[idx.Seg].Groups[idx.Grp])
				continue
			}
			for k := 0; k < len(layer.Segments); k++ {
				s := &layer.Segments[k]
				if s.Range.Begin <= slot && slot < s.Range.End {
					if g := s.locate(key); g != nil {
						mark(layer, g)
					}
					break
				}
			}
		}
	}
	return config, tags
}
