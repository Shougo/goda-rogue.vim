package rogue

import (
	"strings"
	"testing"
)

// used uses a carried item of the kind id (a staff with charges).
func used(t *testing.T, g *Game, id string) *Item {
	t.Helper()
	it := carry(g, id, 0)
	it.Charges = 3
	command(t, g, it, useCommands[it.Def.Category])
	return it
}

// TestGrasses: the new grasses on who drinks them.
func TestGrasses(t *testing.T) {
	g, _ := roomGame(t, 8, 3)
	hp := g.MaxHP
	used(t, g, "inochi")
	if g.MaxHP != hp+5 {
		t.Errorf("max HP %d, want %d", g.MaxHP, hp+5)
	}
	from := g.Player
	used(t, g, "takatobi")
	if g.Player == from {
		t.Error("高飛び草: no warp")
	}
	used(t, g, "subayasa")
	if g.Hasted == 0 {
		t.Error("すばやさ草: not fast")
	}
	g, _ = roomGame(t, 8, 3)
	used(t, g, "konran")
	if g.Confused == 0 {
		t.Error("混乱草: not confused")
	}
	g, _ = roomGame(t, 8, 3)
	turn := g.Turn
	used(t, g, "suimin")
	if g.Turn != turn+1+sleepTurns {
		t.Errorf("睡眠草: %d turns, want %d", g.Turn-turn, 1+sleepTurns)
	}
}

// TestGrassThrown: a grass thrown has its effect on the monster it
// hits.
func TestGrassThrown(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.Facing = "right"
	m := place(g, kind("oni1"), Point{r.X + 3, r.Y})
	g.throwItem(carry(g, "suimin", 0))
	if m.Asleep == 0 || !strings.Contains(g.Message, "鬼は眠ってしまった。") && !strings.Contains(g.Message, "は眠ってしまった。") {
		t.Errorf("asleep %d, %q", m.Asleep, g.Message)
	}
	g.throwItem(carry(g, "konran", 0))
	if m.Confused == 0 {
		t.Error("混乱草 thrown: not confused")
	}
}

// TestScrolls: the scrolls of the room reach its enemies, not an ally;
// the scrolls of heaven and earth raise the equipped weapon and shield.
func TestScrolls(t *testing.T) {
	g, r := roomGame(t, 10, 3)
	a := place(g, kind("oni1"), Point{r.X + 3, r.Y})
	b := place(g, kind("oni1"), Point{r.X + 8, r.Y + 2})
	ally := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	ally.Ally, ally.Order = true, OrderWait
	used(t, g, "bakusui")
	if a.Asleep == 0 || b.Asleep == 0 || ally.Asleep != 0 {
		t.Errorf("asleep %d, %d, the ally %d", a.Asleep, b.Asleep, ally.Asleep)
	}
	used(t, g, "konranmaki")
	if a.Confused == 0 || b.Confused == 0 || ally.Confused != 0 {
		t.Errorf("confused %d, %d, the ally %d", a.Confused, b.Confused, ally.Confused)
	}

	g, _ = roomGame(t, 8, 3)
	used(t, g, "ten")
	if !strings.Contains(g.Message, "何も起こらなかった。") {
		t.Errorf("with no weapon: %q", g.Message)
	}
	w, s := carry(g, "bokutou", 0), carry(g, "kitate", 1)
	w.Known = false
	command(t, g, w, commandEquip)
	command(t, g, s, commandEquip)
	used(t, g, "ten")
	used(t, g, "chi")
	if w.Upgrade != 1 || !w.Known || s.Upgrade != 2 {
		t.Errorf("weapon %+d (known %v), shield %+d: want +1 and +2", w.Upgrade, w.Known, s.Upgrade)
	}
}

// TestStaffs: the new staffs on the monster their bolt hits.
func TestStaffs(t *testing.T) {
	g, r := roomGame(t, 10, 3)
	g.HP, g.MaxHP = 999, 999
	g.Facing = "right"
	m := place(g, kind("kaeru2"), Point{r.X + 4, r.Y})
	// (Asleep, it stays on the line of the bolts.)
	m.Dozing, m.Asleep = true, 99
	used(t, g, "donsoku")
	if m.Slowed == 0 || m.Dozing {
		t.Errorf("鈍足の杖: slowed %d, dozing %v", m.Slowed, m.Dozing)
	}
	used(t, g, "fuuin")
	if _, ok := m.ability(AbilityPull); ok || !m.Sealed {
		t.Error("封印の杖: the frog still pulls")
	}
	used(t, g, "fukou")
	if m.Def.ID != "kaeru1" {
		t.Errorf("不幸の杖: %s, want kaeru1", m.Def.ID)
	}

	g, r = roomGame(t, 10, 3)
	g.HP, g.MaxHP = 999, 999
	g.Facing = "right"
	m = place(g, kind("nezumi1"), Point{r.X + 4, r.Y})
	from := g.Player
	used(t, g, "bashogae")
	if g.Player != (Point{r.X + 4, r.Y}) || !strings.Contains(g.Message, "鼠と場所が入れ替わった。") {
		t.Errorf("the player at %v, %q", g.Player, g.Message)
	}
	// (The rat has moved since: it was put where the player stood.)
	if chebyshev(m.Pos, from) > 1 {
		t.Errorf("the rat at %v, want it near %v", m.Pos, from)
	}
}

// TestCastable: an item of the data has an effect of its kind or one of
// a bolt; a grass or a scroll does not blow away.
func TestCastable(t *testing.T) {
	if !castable(CategoryStaff, MagicSeal) || castable(CategoryGrass, MagicBlow) || castable(CategoryPot, MagicSleep) || castable(CategoryGrass, "石化") {
		t.Error("castable")
	}
}
