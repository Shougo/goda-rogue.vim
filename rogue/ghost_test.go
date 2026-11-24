package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestHungryGhost: next to the player, it takes fullness, or max
// fullness, instead of a blow; food thrown at it defeats it; eating more
// than the player holds brings some of the max back.
func TestHungryGhost(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("gaki1"), Point{r.X + 2, r.Y})
	rule := g.Dungeon.Rules.Fullness
	a := Ability{Name: AbilityHunger, Values: []int{100, 10, 0}}
	if !g.starve(m, a) || g.Fullness != rule-10 || g.MaxFullness != rule || !strings.Contains(g.Message, "餓鬼に満腹度を吸われた！") {
		t.Fatalf("fullness %d/%d, %q", g.Fullness, g.MaxFullness, g.Message)
	}
	g.Fullness = 0
	if g.starve(m, a) {
		t.Error("it takes fullness from none")
	}
	g.Fullness = rule
	a.Values[1], a.Values[2] = 0, 20
	if !g.starve(m, a) || g.MaxFullness != rule-20 || g.Fullness != rule-20 || !strings.Contains(g.fullnessLabel(), "（最大 80%）") {
		t.Fatalf("max: fullness %d/%d, status %q", g.Fullness, g.MaxFullness, g.fullnessLabel())
	}
	for range 10 {
		g.starve(m, a)
	}
	if g.MaxFullness != rule*minMaxFullness/100 {
		t.Errorf("max fullness %d at least", g.MaxFullness)
	}
	// Eating more than it holds brings some back.
	g.MaxFullness, g.Fullness = rule-20, rule-20
	command(t, g, carry(g, "onigiri", 0), commandEat)
	if g.MaxFullness != rule-20+fullnessBack || g.Fullness != g.MaxFullness {
		t.Errorf("after eating: %d/%d", g.Fullness, g.MaxFullness)
	}
	for id, want := range map[string][]int{"gaki1": {25, 10, 0}, "gaki2": {25, 30, 0}} {
		got, ok := newMonster(kind(id)).ability(AbilityHunger)
		if !ok || got.Values[0] != want[0] || got.Values[1] != want[1] || got.Values[2] != want[2] {
			t.Errorf("%s: %v", id, got.Values)
		}
	}

	// Food thrown at it: it rests in peace; another item hits it.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.Facing = "right"
	m.HP, m.MaxHP = 500, 500
	command(t, g, carry(g, "yakusou", 0), commandThrow)
	if !slices.Contains(g.Monsters, m) {
		t.Fatal("a grass defeated it")
	}
	g.Monsters = nil
	place(g, kind("gaki1"), Point{r.X + 2, r.Y})
	exp := g.EXP
	command(t, g, carry(g, "onigiri", 0), commandThrow)
	if g.MonsterAt(Point{r.X + 2, r.Y}) != nil || g.EXP <= exp || !strings.Contains(strings.Join(g.Log, ""), "餓鬼はおにぎりを食べて成仏した。") {
		t.Errorf("fed: %v, EXP %d to %d, %q", g.MonsterAt(Point{r.X + 2, r.Y}), exp, g.EXP, g.Message)
	}
}
