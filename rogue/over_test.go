package rogue

import (
	"slices"
	"testing"
)

// TestOverRank: a monster of rank 4 that defeats another ranks up beyond
// its rank: its attack and defense grow, its max HP stays, its name gets
// the level.
func TestOverRank(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	bear := place(g, kind("kuma4"), Point{r.X + 4, r.Y + 2})
	def := bear.Def
	hp := bear.MaxHP
	for n := 2; n <= 3; n++ {
		rat := place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 2})
		g.kill(rat, bear, false, true)
		want := def.Name + string(rune('0'+n))
		if bear.Name != want || bear.Over != n-1 || bear.Rank != 4 || bear.MaxHP != hp {
			t.Fatalf("level %d: %s, over %d, rank %d, max HP %d", n, bear.Name, bear.Over, bear.Rank, bear.MaxHP)
		}
	}
	if bear.Attack != overStat(def.Attack, 130, 2) || bear.Defense != overStat(def.Defense, 120, 2) ||
		bear.Attack <= def.Attack || bear.Defense <= def.Defense {
		t.Errorf("attack %d (was %d), defense %d (was %d)", bear.Attack, def.Attack, bear.Defense, def.Defense)
	}
	if !slices.Contains(g.Log, def.Name+"2は"+def.Name+"3になった！") {
		t.Errorf("not said: %v", g.Log)
	}
	// On the screen: its own highlight.
	if _, hl := g.glyph(bear.Pos); hl != hlOver {
		t.Errorf("highlight %q", hl)
	}
	// A monster of a lower rank ranks up as before.
	rat := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	g.rankUp(rat)
	if rat.Rank != 2 || rat.Over != 0 {
		t.Errorf("a rat: rank %d, over %d", rat.Rank, rat.Over)
	}
}

// TestOverStat: the growth by levels.
func TestOverStat(t *testing.T) {
	for _, c := range []struct{ base, pct, levels, want int }{
		{10, 130, 0, 10}, {10, 130, 1, 13}, {10, 130, 2, 17}, {20, 120, 3, 35},
	} {
		if got := overStat(c.base, c.pct, c.levels); got != c.want {
			t.Errorf("overStat(%d, %d, %d) = %d, want %d", c.base, c.pct, c.levels, got, c.want)
		}
	}
}
