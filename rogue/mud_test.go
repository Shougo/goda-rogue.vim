package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestSplit: a blow that leaves a mud with 2 HP or more may split it:
// half its HP each, the new one next to it gives nothing.
func TestSplit(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Facing = "right"
	m := place(g, kind("doro1"), Point{r.X + 1, r.Y})
	def := *m.Def
	def.Abilities = []Ability{{Name: AbilitySplit, Values: []int{100}}}
	m.Def = &def
	m.HP, m.MaxHP = 41, 41
	g.split(m)
	if len(g.Monsters) != 2 || m.HP != 20 || g.Monsters[1].HP != 20 || !g.Monsters[1].Spawned {
		t.Fatalf("%d monsters, HP %d", len(g.Monsters), m.HP)
	}
	if c := g.Monsters[1]; chebyshev(c.Pos, m.Pos) != 1 || c.Pos == g.Player {
		t.Errorf("the new one at %v, the mud at %v", c.Pos, m.Pos)
	}
	m.HP = 1
	g.split(m)
	if len(g.Monsters) != 2 {
		t.Error("split with 1 HP")
	}
}

// TestRot: a mud of rank 2 rots an onigiri carried (not one in a pot);
// rank 4 with no onigiri turns an item into a rotten one; with nothing
// to rot it hits.
func TestRot(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("doro2"), Point{r.X + 1, r.Y})
	a := Ability{Name: AbilityRot, Values: []int{100, 1, 0}}
	o := carry(g, "onigiri", 0)
	pot := carry(g, "hozon", 0)
	kept := &Item{Def: g.Dungeon.Item("onigiri")}
	pot.Contents = []*Item{kept}
	if !g.spoil(m, a) || o.Def.ID != "kusatta" || kept.Def.ID != "onigiri" || !strings.Contains(g.Message, "おにぎりが腐ってしまった！") {
		t.Fatalf("%s, kept %s, %q", o.Def.ID, kept.Def.ID, g.Message)
	}
	if g.spoil(m, a) {
		t.Error("rotted with no onigiri")
	}
	a.Values[2] = 1
	grass := carry(g, "yakusou", 0)
	if !g.spoil(m, a) || grass.Def.ID != "kusatta" && pot.Def.ID != "kusatta" {
		t.Errorf("rank 4: %s, %s", grass.Def.ID, pot.Def.ID)
	}
}

// TestRotten: eating a rotten onigiri fills and brings a bad state; the
// trap of rot rots every onigiri carried; the bots eat it last.
func TestRotten(t *testing.T) {
	g, _ := roomGame(t, 8, 3)
	g.Fullness = 10
	it := carry(g, "kusatta", 0)
	command(t, g, it, commandEat)
	bad := false
	for _, e := range rottenStates {
		bad = bad || strings.Contains(g.Message, stateTexts[e].player)
	}
	if g.Fullness < 39 || !strings.Contains(g.Message, "腐っていた") || !bad {
		t.Errorf("fullness %d, %q", g.Fullness, g.Message)
	}

	g, r := roomGame(t, 8, 3)
	a, b := carry(g, "onigiri", 0), carry(g, "ookiionigiri", 0)
	trap(g, "kusari", Point{r.X + 1, r.Y})
	move(t, g, "right")
	if a.Def.ID != "kusatta" || b.Def.ID != "kusatta" {
		t.Errorf("%s, %s: want both rotten", a.Def.ID, b.Def.ID)
	}

	foods := keepOnigiri([]*Item{{Def: g.Dungeon.Item("kusatta")}, {Def: g.Dungeon.Item("onigiri")}})
	if foods[0].Def.ID != "onigiri" {
		t.Error("the bots eat the rotten one first")
	}
	if !slices.ContainsFunc(cave().Items, func(d *ItemDef) bool { return d.Effect == EffectRotten }) {
		t.Error("no rotten onigiri in the cave")
	}
}
