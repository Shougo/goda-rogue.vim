package rogue

import "testing"

// TestAfflict: the monsters of states blind, put to sleep, confuse and
// weaken; ranks 1 and 2 next to the player only; the bracelets ward
// sleep and confusion off.
func TestAfflict(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Player = Point{r.X, r.Y}
	g.HP, g.MaxHP = 999, 999
	a := func(what, sight int) Ability {
		return Ability{Name: AbilityAfflict, Values: []int{100, sight, what, 7}}
	}
	far := place(g, kind("fukurou1"), Point{r.X + 3, r.Y + 2})
	if g.afflictBy(far, a(afflictBlind, 0)) || g.Blind != 0 {
		t.Error("blinded from afar at rank 1")
	}
	if !g.afflictBy(far, a(afflictBlind, 1)) || g.Blind != 7 {
		t.Errorf("blind %d", g.Blind)
	}
	// Blind, the player sees next to it only.
	if g.Visible(far.Pos) || !g.Visible(g.Player.Add(Point{1, 0})) {
		t.Error("what a blind player sees")
	}
	g.Blind = 0
	near := place(g, kind("kiri1"), g.Player.Add(Point{1, 0}))
	if !g.afflictBy(near, a(afflictSleep, 0)) || g.Asleep != 7 {
		t.Errorf("asleep %d", g.Asleep)
	}
	g.Asleep = 0
	carry(g, "suiminyoke", 0).Equipped = true
	if g.afflictBy(near, a(afflictSleep, 0)); g.Asleep != 0 {
		t.Error("asleep with the bracelet")
	}
	g.equipped(CategoryBracelet).Equipped = false
	carry(g, "konranyoke", 0).Equipped = true
	if g.afflictBy(near, a(afflictConfuse, 0)); g.Confused != 0 {
		t.Error("confused with the bracelet")
	}
	// Weakened: 30% less dealt or more taken, longer when again, gone on
	// the next floor.
	for range 6 {
		g.afflictBy(near, a(afflictWeaken, 0))
	}
	if g.AttackDown+g.DefenseDown != 42 {
		t.Errorf("weak %d + %d", g.AttackDown, g.DefenseDown)
	}
	g.AttackDown, g.DefenseDown = 5, 5
	if g.weakening(10) != 7 || g.weakened(10) != 13 {
		t.Errorf("dealt %d, taken %d", g.weakening(10), g.weakened(10))
	}
	g.descend()
	if g.AttackDown+g.DefenseDown != 0 {
		t.Error("weak on the next floor")
	}
}

// TestBlindMonster: a blind monster walks at random and attacks no one.
func TestBlindMonster(t *testing.T) {
	g, _ := roomGame(t, 6, 4)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("nezumi4"), g.Player.Add(Point{1, 0}))
	m.Blind = 30
	for range 30 {
		g.act(m)
	}
	if g.HP != 999 || m.Blind != 0 {
		t.Errorf("HP %d, blind %d", g.HP, m.Blind)
	}
}
