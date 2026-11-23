package rogue

import "testing"

// TestFits: what a tile must be free of, by the needs.
func TestFits(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	p := Point{r.X + 3, r.Y + 1}
	if !g.fits(p, needHidden) || !g.free(p) {
		t.Fatal("an empty tile does not fit")
	}
	item(g, "yakusou", p)
	if g.fits(p, needNoItem) || g.free(p) || !g.fits(p, needNoCreature|needNoTrap) {
		t.Error("a tile with an item")
	}
	q := Point{r.X + 4, r.Y + 1}
	trap(g, "harahe", q)
	if g.fits(q, needNoTrap) || !g.free(q) {
		t.Error("a tile with a trap")
	}
	g.Decoys = []Decoy{{Pos: Point{r.X + 5, r.Y + 1}}}
	if g.fits(Point{r.X + 5, r.Y + 1}, needNoStairs) || g.fits(g.Player, needNoCreature) {
		t.Error("a decoy or the player's tile fits")
	}
	// In the player's sight, or near it: not for what appears unseen.
	if g.fits(Point{r.X + 6, r.Y + 2}, needUnseen) {
		t.Error("a tile of the player's room fits unseen")
	}
	if g.fits(Point{r.X - 1, r.Y - 1}, 0) && !g.Level.At(Point{r.X - 1, r.Y - 1}).Walkable() {
		t.Error("rock fits")
	}
}

// TestFind: the first tile drawn that fits, and none after too many
// tries.
func TestFind(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	n := 0
	p, ok := g.find(func() Point { n++; return Point{r.X + n - 1, r.Y} }, needNoCreature)
	if !ok || p != (Point{r.X + 1, r.Y}) {
		t.Errorf("found %v (%v): want the tile after the player's", p, ok)
	}
	n = 0
	if _, ok := g.find(func() Point { n++; return g.Player }, needNoCreature); ok || n != findTries {
		t.Errorf("found the player's tile, or %d tries", n)
	}
	p, ok = g.find(func() Point { return Point{r.X + 2, r.Y} }, 0, func(Point) bool { return false })
	if ok {
		t.Errorf("found %v against the extra condition", p)
	}
}
