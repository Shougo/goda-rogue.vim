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
	// (Every other way out taken: a doorway may be next to the corner.)
	for _, d := range around {
		if p := m.Pos.Add(d); g.Level.At(p).Walkable() && !g.occupied(p) {
			place(g, kind("nezumi1"), p)
		}
	}
	hp := g.HP
	for range 20 {
		g.act(m)
	}
	if m.Pos != (Point{r.X, r.Y}) || g.HP == hp || !slices.Contains(g.Monsters, m) {
		t.Errorf("cornered: at %v, HP %d", m.Pos, g.HP)
	}
}

// TestSetTrap: a monster that sets traps hides one where it stands, out
// of the player's reach, and leaves one where it is defeated.
func TestSetTrap(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Depth = 10
	m := place(g, kind("karakuri1"), Point{r.X + 5, r.Y + 2})
	g.HP, g.MaxHP = 999, 999
	for range 200 {
		m.Pos = Point{r.X + 5, r.Y + 2}
		if g.act(m); len(g.Traps) > 0 {
			break
		}
	}
	if len(g.Traps) == 0 || g.Traps[0].Found {
		t.Fatalf("traps %v", g.Traps)
	}
	g.Traps = nil
	m.Pos = Point{r.X + 6, r.Y + 3}
	g.hurt(m, 999)
	if g.TrapAt(Point{r.X + 6, r.Y + 3}) == nil {
		t.Error("no trap where it fell")
	}
}

// TestHunt: a monster that sees a rabbit goes for it before the player,
// and ranks up when it kills it; the item of the rabbit is lost.
func TestHunt(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X, r.Y}
	rat := place(g, kind("nezumi1"), Point{r.X + 2, r.Y + 2})
	rabbit := place(g, kind("usagi1"), Point{r.X + 5, r.Y + 2})
	g.act(rat)
	if chebyshev(rat.Pos, rabbit.Pos) != 2 {
		t.Errorf("the rat at %v, not toward the rabbit at %v", rat.Pos, rabbit.Pos)
	}
	rabbit.HP = 1
	rabbit.Pos = Point{r.X + 4, r.Y + 2}
	items := len(g.Items)
	for range 50 {
		if !slices.Contains(g.Monsters, rabbit) {
			break
		}
		g.act(rat)
	}
	if slices.Contains(g.Monsters, rabbit) || rat.Rank != 2 || len(g.Items) != items {
		t.Errorf("rabbit alive %v, rat rank %d, items %d (were %d)", slices.Contains(g.Monsters, rabbit), rat.Rank, len(g.Items), items)
	}
	// Next to the player, a monster with no rabbit next to it attacks
	// the player.
	rabbit = place(g, kind("usagi1"), Point{r.X + 7, r.Y + 3})
	rat.Pos = Point{r.X + 1, r.Y}
	hp := g.HP
	for range 20 {
		g.act(rat)
	}
	if g.HP == hp || rat.Pos != (Point{r.X + 1, r.Y}) {
		t.Errorf("next to the player: HP %d, at %v", g.HP, rat.Pos)
	}
	// A rabbit does not hunt a rabbit.
	if g.prey(rabbit) != nil {
		t.Error("a rabbit hunts")
	}
}
