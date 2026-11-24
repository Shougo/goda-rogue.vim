package rogue

import (
	"strings"
	"testing"
)

// TestCurser: the monster that curses, next to the player, curses what
// its rank reaches instead of a blow; with nothing to curse, it attacks.
func TestCurser(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("noroi1"), Point{r.X + 1, r.Y})
	a := Ability{Name: AbilityCurse, Values: []int{100, curseArms, 1}}
	grass := carry(g, "yakusou", 0)
	grass.Known = false
	br := carry(g, "chikara", 0)
	br.Equipped = true
	if g.curser(m, a) || grass.Cursed || br.Cursed {
		t.Fatalf("rank 1 with no weapon nor shield: %q", g.Message)
	}
	w := carry(g, "bokutou", 0)
	w.Equipped = true
	if !g.curser(m, a) || !w.Cursed || grass.Cursed || br.Cursed || !strings.Contains(g.Message, "木刀が呪われてしまった！") {
		t.Fatalf("rank 1: weapon %v, grass %v, bracelet %v, %q", w.Cursed, grass.Cursed, br.Cursed, g.Message)
	}
	// Rank 2: the bracelet too (the weapon is cursed already).
	a.Values[1] = curseEquipped
	if !g.curser(m, a) || !br.Cursed || grass.Cursed {
		t.Fatalf("rank 2: bracelet %v, grass %v", br.Cursed, grass.Cursed)
	}
	if g.curser(m, a) {
		t.Error("rank 2 with everything equipped cursed already")
	}
	// Rank 4: two of any items; the player knows.
	a.Values[1], a.Values[2] = curseAny, 2
	another := carry(g, "yakusou", 0)
	if !g.curser(m, a) || !grass.Cursed || !another.Cursed || !strings.HasSuffix(g.describe(grass), "（呪）") {
		t.Errorf("rank 4: grasses %v %v, label %q", grass.Cursed, another.Cursed, g.describe(grass))
	}
	// A blessing takes a curse.
	a.Values[2] = 1
	food := carry(g, "onigiri", 0)
	food.Blessed = true
	if !g.curser(m, a) || food.Cursed || food.Blessed {
		t.Errorf("blessed food: cursed %v blessed %v", food.Cursed, food.Blessed)
	}
	// The monsters of the dungeon have the ability.
	for id, want := range map[string][]int{"noroi1": {25, 0, 1}, "noroi2": {25, 1, 1}} {
		got, ok := newMonster(kind(id)).ability(AbilityCurse)
		if !ok || got.Values[0] != want[0] || got.Values[1] != want[1] || got.Values[2] != want[2] {
			t.Errorf("%s: %v", id, got.Values)
		}
	}

	// A scroll of おはらい read in its room destroys it.
	other := place(g, kind("nezumi1"), Point{r.X + 3, r.Y})
	exp := g.EXP
	command(t, g, carry(g, "oharai", 0), commandRead)
	g.Prompt = nil
	if g.MonsterAt(Point{r.X + 1, r.Y}) != nil || g.MonsterAt(other.Pos) == nil || g.EXP <= exp {
		t.Errorf("exorcised: %v, the other %v, EXP %d to %d", g.MonsterAt(Point{r.X + 1, r.Y}), g.MonsterAt(other.Pos), exp, g.EXP)
	}
}

// TestCurseTrap: an item carried that is no equipment is cursed.
func TestCurseTrap(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	w := carry(g, "bokutou", 0)
	g.spring(&Trap{Def: g.Dungeon.Trap("noroi"), Pos: g.Player}, true)
	if w.Cursed || !strings.Contains(g.Message, "何も起こらなかった。") {
		t.Errorf("with equipment only: %v, %q", w.Cursed, g.Message)
	}
	grass := carry(g, "yakusou", 0)
	g.spring(&Trap{Def: g.Dungeon.Trap("noroi"), Pos: g.Player}, true)
	if w.Cursed || !grass.Cursed || !strings.Contains(g.Message, "薬草が呪われてしまった！") {
		t.Errorf("weapon %v, grass %v, %q", w.Cursed, grass.Cursed, g.Message)
	}
}
