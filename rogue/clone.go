package rogue

import (
	"maps"
	"math/rand/v2"
	"slices"
)

// Clone returns a copy of g that goes on on its own: the same state, the
// same random source at the same point, so that the same actions give
// the same run, and nothing done to one shows in the other. The dungeon
// (read only) is shared. For the bots to try a move ahead (look-ahead).
func (g *Game) Clone() *Game {
	c := *g
	cl := cloner{items: map[*Item]*Item{}, monsters: map[*Monster]*Monster{}}

	lv := *g.Level
	lv.Tiles = slices.Clone(g.Level.Tiles)
	lv.Rooms = slices.Clone(g.Level.Rooms)
	lv.Made = maps.Clone(g.Level.Made)
	c.Level = &lv
	c.distCache = distCache{}
	c.sights = nil

	c.Items = cl.itemList(g.Items)
	c.Inventory = cl.itemList(g.Inventory)
	c.selling = cl.item(g.selling)
	c.Monsters = make([]*Monster, len(g.Monsters))
	for i, m := range g.Monsters {
		c.Monsters[i] = cl.monster(m)
	}
	c.heldBy = cl.monster(g.heldBy)
	c.ordered = cl.monster(g.ordered)
	if g.Shop != nil {
		s := *g.Shop
		s.Keepers = make([]*Monster, len(g.Shop.Keepers))
		for i, k := range g.Shop.Keepers {
			s.Keepers[i] = cl.monster(k)
		}
		c.Shop = &s
	}
	if g.Prompt != nil {
		p := *g.Prompt
		p.Choices = slices.Clone(p.Choices)
		p.items = slices.Clone(p.items)
		p.targets = cl.itemList(p.targets)
		c.Prompt = &p
	}
	c.Traps = make([]*Trap, len(g.Traps))
	for i, t := range g.Traps {
		tt := *t
		c.Traps[i] = &tt
	}
	c.Decoys = slices.Clone(g.Decoys)
	c.Seen = slices.Clone(g.Seen)
	c.Safe = slices.Clone(g.Safe)
	c.Log = slices.Clone(g.Log)
	c.events = slices.Clone(g.events)
	c.identified = maps.Clone(g.identified)
	c.fakeNames = maps.Clone(g.fakeNames)
	c.met = maps.Clone(g.met)
	c.priced = maps.Clone(g.priced)
	c.glinted = maps.Clone(g.glinted)
	c.names = maps.Clone(g.names)
	c.guaranteed = map[int][]string{}
	for k, v := range g.guaranteed {
		c.guaranteed[k] = slices.Clone(v)
	}
	pcg := *g.pcg
	c.pcg = &pcg
	c.rng = rand.New(c.pcg)
	return &c
}

// cloner copies items and monsters once each, so that what points to the
// same one in the game points to the same copy.
type cloner struct {
	items    map[*Item]*Item
	monsters map[*Monster]*Monster
}

func (cl cloner) itemList(its []*Item) []*Item {
	if its == nil {
		return nil
	}
	out := make([]*Item, len(its))
	for i, it := range its {
		out[i] = cl.item(it)
	}
	return out
}

func (cl cloner) item(it *Item) *Item {
	if it == nil {
		return nil
	}
	if c, ok := cl.items[it]; ok {
		return c
	}
	c := *it
	cl.items[it] = &c
	c.Seals = slices.Clone(it.Seals)
	c.Contents = cl.itemList(it.Contents)
	return &c
}

func (cl cloner) monster(m *Monster) *Monster {
	if m == nil {
		return nil
	}
	if c, ok := cl.monsters[m]; ok {
		return c
	}
	c := *m
	cl.monsters[m] = &c
	c.loot = cl.item(m.loot)
	c.stolen = cl.itemList(m.stolen)
	c.belly = cl.itemList(m.belly)
	return &c
}
