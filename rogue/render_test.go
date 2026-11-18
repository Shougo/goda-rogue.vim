package rogue

import "testing"

// TestAxis shows the line the player faces after turning, to the wall,
// until another action.
func TestAxis(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	g.look()
	if err := g.Do("turn", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	s := g.Render(Size{200, 60})
	// From the player's tile to the east wall of the room (rooms may have
	// a door or a corridor on that side: count the walkable seen tiles).
	want := 0
	for p := g.Player.Add(Directions["right"]); g.Level.At(p).Walkable() && g.IsSeen(p); p = p.Add(Directions["right"]) {
		want++
	}
	if want < r.W-1 {
		t.Fatalf("only %d tiles to the right", want)
	}
	if got := len(s.Highlights[hlFacing]); got != want {
		t.Errorf("axis of %d tiles, want %d", got, want)
	}
	// Gone at the next action.
	if err := g.Do("wait", nil); err != nil {
		t.Fatal(err)
	}
	if got := len(g.Render(Size{200, 60}).Highlights[hlFacing]); got != 1 {
		t.Errorf("after waiting: %d tiles", got)
	}
}
