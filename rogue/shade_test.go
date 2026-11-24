package rogue

import (
	"strings"
	"testing"
)

// TestInvisible: the player does not see a shade, but its blows; eye
// drops show it on the floor.
func TestInvisible(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
	m := place(g, kind("kage1"), Point{r.X + 3, r.Y + 1})
	if c, _ := g.glyph(m.Pos); c == m.Glyph || len(g.visibleEnemies()) != 0 {
		t.Errorf("seen: %q", c)
	}
	g.hitPlayer(m)
	if !strings.Contains(g.Message, "何者かの攻撃") {
		t.Errorf("%q", g.Message)
	}
	g.eyeDrops()
	if c, _ := g.glyph(m.Pos); c != m.Glyph || len(g.visibleEnemies()) != 1 {
		t.Errorf("not seen after eye drops: %q", c)
	}
}

// TestThroughWalls: a monster of the underworld goes through rock to
// the player, and comes out of it to fight.
func TestThroughWalls(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X, r.Y + 1}
	m := place(g, kind("mei1"), Point{r.X - 4, r.Y + 1})
	if g.Level.At(m.Pos).Walkable() {
		t.Skip("not rock")
	}
	hp := g.HP
	for range 10 {
		g.act(m)
	}
	if chebyshev(m.Pos, g.Player) != 1 || !g.Level.At(m.Pos).Walkable() || g.HP == hp {
		t.Errorf("at %v (walkable %v), HP %d", m.Pos, g.Level.At(m.Pos).Walkable(), g.HP)
	}
}
