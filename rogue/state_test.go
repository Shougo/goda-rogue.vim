package rogue

import (
	"slices"
	"testing"
)

// TestStates: the bolts of states bring the same states to the player
// and to a monster, named in a fixed order.
func TestStates(t *testing.T) {
	var s States
	s.Hasted = 3
	for _, e := range []MagicEffect{MagicSeal, MagicSleep, MagicSlow, MagicConfuse} {
		if !s.take(e) {
			t.Errorf("%s is no state", e)
		}
	}
	if s.take(MagicWarp) {
		t.Error("ワープ is a state")
	}
	// (Slowing ends the haste.)
	if got, want := s.names("封印"), []string{"混乱", "鈍足", "睡眠", "封印"}; !slices.Equal(got, want) {
		t.Errorf("names %q, want %q", got, want)
	}
	// Asleep, no action; then slowed, every other turn.
	for range sleepTurns {
		if n := s.actions(2, 1); n != 0 {
			t.Fatalf("%d actions asleep", n)
		}
	}
	if a, b := s.actions(2, 1), s.actions(2, 2); a != 1 || b != 0 {
		t.Errorf("slowed: %d and %d actions, want 1 and 0", a, b)
	}
}
