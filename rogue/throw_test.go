package rogue

import (
	"strings"
	"testing"
)

// arrows gives the player n arrows of id.
func arrows(g *Game, id string, n int) *Item {
	a := carry(g, id, 0)
	a.Count = n
	return a
}

func TestShoot(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	g.Facing = "right"
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	// A row with rock on the right of the room (not a doorway), for the
	// arrows to fall in the room. (The room comes from the data.)
	y := r.Y
	for g.Level.At(Point{r.X + r.W, y}).Walkable() {
		y++
	}
	g.Player = Point{r.X, y}
	if err := g.Do("shoot", nil); err != nil {
		t.Fatal(err)
	}
	if g.Message != "矢を装備していない。" || g.Turn != 0 {
		t.Errorf("message %q, turn %d", g.Message, g.Turn)
	}

	a := arrows(g, "kiya", 3)
	if g.Label(a) != "矢 木の矢[3]" {
		t.Errorf("label %q", g.Label(a))
	}
	// Equipping arrows takes no turn.
	command(t, g, a, commandEquip)
	if !a.Equipped || g.Turn != 0 {
		t.Fatalf("equipped %v, turn %d", a.Equipped, g.Turn)
	}
	m := place(g, kind("nezumi4"), Point{r.X + 3, y})
	m.HP = 999
	if err := g.Do("shoot", nil); err != nil {
		t.Fatal(err)
	}
	if a.Count != 2 || m.HP == 999 || g.Turn != 1 {
		t.Errorf("count %d, HP %d, turn %d", a.Count, m.HP, g.Turn)
	}
	if !strings.HasPrefix(g.Message, "木の矢を撃った。鉄鼠に") {
		t.Errorf("message %q", g.Message)
	}
	// An arrow that hits is used up.
	if len(g.Items) != 0 {
		t.Errorf("items %v", g.Items)
	}

	// Nothing to hit: it falls at the end of its flight.
	g.Monsters = nil
	for range 2 {
		if err := g.Do("shoot", nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.Inventory) != 0 || len(g.Items) != 2 || g.equipped(CategoryArrow) != nil {
		t.Errorf("inventory %v, items %v", g.Inventory, g.Items)
	}
	for _, it := range g.Items {
		// (The second falls next to the first: on its row or one
		// next to it.)
		if it.Count != 1 || abs(it.Pos.Y-y) > 1 || it.Pos.X <= r.X {
			t.Errorf("arrow at %v, count %d", it.Pos, it.Count)
		}
	}
	// Picking them up gathers them.
	arrows(g, "kiya", 5)
	g.Player = g.Items[0].Pos
	g.pickUp(true)
	g.Player = g.Items[0].Pos
	g.pickUp(true)
	if len(g.Inventory) != 1 || g.Inventory[0].Count != 7 || !strings.Contains(g.Message, "木の矢を1本拾った。") {
		t.Errorf("inventory %v, message %q", g.Inventory, g.Message)
	}
}

func TestThrow(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	g.Facing = "right"
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP = 50

	// A grass has its effect on the monster.
	command(t, g, carry(g, "yakusou", 0), commandThrow)
	if m.HP != m.MaxHP || !strings.Contains(g.Message, "鉄鼠のHPが") || len(g.Items) != 0 {
		t.Errorf("HP %d/%d, message %q", m.HP, m.MaxHP, g.Message)
	}

	// Another item hurts it a little, and falls by it.
	sword := carry(g, "tetsuken", 0)
	m.Pos, m.Dozing = Point{r.X + 2, r.Y}, true
	command(t, g, sword, commandEquip)
	hp := m.HP
	command(t, g, sword, commandThrow)
	if n := hp - m.HP; n < 1 || n > 2 || sword.Equipped || len(g.Inventory) != 0 {
		t.Errorf("damage %d, equipped %v", n, sword.Equipped)
	}
	if len(g.Items) != 1 || chebyshev(g.Items[0].Pos, m.Pos) > 2 {
		t.Errorf("items %v", g.Items)
	}
}

func TestLand(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	p := Point{r.X + 1, r.Y + 1}
	for range 2 {
		g.land(&Item{Def: g.Dungeon.Item("yakusou")}, p)
	}
	if len(g.Items) != 2 || g.Items[0].Pos != p || chebyshev(g.Items[1].Pos, p) != 1 {
		t.Errorf("items at %v, %v", g.Items[0].Pos, g.Items[1].Pos)
	}
}

// withRules returns a copy of d with its rules changed by f.
func withRules(d *Dungeon, f func(*Rules)) *Dungeon {
	c := *d
	f(&c.Rules)
	return &c
}

// TestThrowHitRate: what the player throws hits at 主の投擲命中, not at
// 主の命中; a dungeon without it throws at 主の命中.
func TestThrowHitRate(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Facing = "right"
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate, r.PlayerThrowRate = 100, 0 })
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP, m.MaxHP = 9999, 9999
	for range 10 {
		g.throwItem(g.newItemOf(g.rng, g.Dungeon.Item("kiya"), Point{}))
	}
	if m.HP != 9999 {
		t.Errorf("an arrow hit at 主の投擲命中 0: HP %d", m.HP)
	}
	f := frozen(t).Rules
	if f.PlayerThrowRate != f.PlayerHitRate || f.MonsterThrowRate != f.MonsterHitRate {
		t.Errorf("throw rates %d/%d, want the hit rates %d/%d", f.PlayerThrowRate, f.MonsterThrowRate, f.PlayerHitRate, f.MonsterHitRate)
	}
	if c := cave().Rules; c.PlayerThrowRate != 84 || c.MonsterThrowRate != 84 {
		t.Errorf("the cave throws at %d/%d, want 84/84", c.PlayerThrowRate, c.MonsterThrowRate)
	}
}
