package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestStill: a still monster never steps, but hits the player next to
// it, and shoots.
func TestStill(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	m := place(g, kind("ki1"), Point{r.X + 5, r.Y})
	m.Dozing = false
	for range 5 {
		g.act(m)
	}
	if m.Pos != (Point{r.X + 5, r.Y}) {
		t.Errorf("moved to %v", m.Pos)
	}
}

// TestHold: a blow of a monster that holds keeps the player from
// stepping away until the turns pass or the monster is gone.
func TestHold(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	g.holdBy(m)
	if g.Stuck != 0 {
		t.Fatal("held by a monster without the ability")
	}
	g.Monsters = nil
	m = place(g, kind("ki1"), Point{r.X + 1, r.Y})
	for range 100 {
		if g.holdBy(m); g.Stuck > 0 {
			break
		}
	}
	if g.Stuck != 3 || g.heldBy != m || !strings.Contains(g.Message, "若木に捕まった！") {
		t.Fatalf("stuck %d, %q", g.Stuck, g.Message)
	}
	from := g.Player
	if err := g.Do("move", []string{"down"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != from || g.Stuck == 0 || !strings.Contains(g.Message, "若木に捕まっていて動けない。") {
		t.Errorf("moved to %v, stuck %d, %q", g.Player, g.Stuck, g.Message)
	}
	// Gone: free.
	g.remove(m)
	if err := g.Do("move", []string{"down"}); err != nil {
		t.Fatal(err)
	}
	if g.Player == from || g.Stuck != 0 {
		t.Errorf("not free: %v, stuck %d", g.Player, g.Stuck)
	}
}

// TestBurn: what the player throws at a monster that burns things is
// gone, with no effect; a monster's arrow too.
func TestBurn(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate, r.MonsterThrowRate = 100, 100 })
	m := place(g, kind("honoo1"), Point{r.X + 2, r.Y})
	g.Facing = "right"
	hp := m.HP
	it := carry(g, "kiya", 0)
	it.Count = 1
	command(t, g, it, commandThrow)
	if m.HP != hp || slices.Contains(g.Items, it) || !strings.Contains(g.Message, "鬼火に燃やされた！") {
		t.Errorf("HP %d of %d, items %v, %q", m.HP, hp, g.Items, g.Message)
	}
	// A monkey's stone at it, the player behind.
	g.Player = Point{r.X + 4, r.Y}
	s := place(g, kind("saru1"), Point{r.X, r.Y})
	a, _ := s.ability(AbilityShoot)
	a.Values[1] = 100
	g.shootAt(s, a)
	if m.HP != hp || len(g.Items) != 0 {
		t.Errorf("stone: HP %d, items %v", m.HP, g.Items)
	}
}
