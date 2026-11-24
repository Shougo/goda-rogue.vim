package rogue

import (
	"slices"
	"testing"
)

// TestPack: a monster of a pack comes with its fellows next to it.
func TestPack(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	m := place(g, kind("ari1"), Point{r.X + 4, r.Y + 2})
	m.Dozing = true
	ms := g.pack(m)
	if len(ms) != 2 || len(g.Monsters) != 3 {
		t.Fatalf("%d more, %d monsters", len(ms), len(g.Monsters))
	}
	for _, f := range ms {
		if f.Def != m.Def || !f.Dozing || chebyshev(f.Pos, m.Pos) != 1 {
			t.Errorf("%s at %v, dozing %v", f.Def.ID, f.Pos, f.Dozing)
		}
	}
	if g.pack(place(g, kind("nezumi1"), Point{r.X, r.Y})) != nil {
		t.Error("a rat with a pack")
	}
}

// TestFlee: a rabbit keeps away from the player it sees, and fights when
// it cannot.
func TestFlee(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("usagi1"), Point{r.X + 4, r.Y + 2})
	g.Player = Point{r.X + 3, r.Y + 2}
	g.act(m)
	if chebyshev(m.Pos, g.Player) != 2 {
		t.Errorf("at %v, the player at %v", m.Pos, g.Player)
	}
	// Cornered.
	g.Monsters = nil
	m = place(g, kind("usagi1"), Point{r.X, r.Y})
	g.Player = Point{r.X + 1, r.Y + 1}
	for _, p := range []Point{{r.X + 1, r.Y}, {r.X, r.Y + 1}} {
		place(g, kind("nezumi1"), p)
	}
	hp := g.HP
	for range 20 {
		g.act(m)
	}
	if m.Pos != (Point{r.X, r.Y}) || g.HP == hp || !slices.Contains(g.Monsters, m) {
		t.Errorf("cornered: at %v, HP %d", m.Pos, g.HP)
	}
}
