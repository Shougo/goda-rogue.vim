package rogue

import (
	"math/rand/v2"
	"testing"
)

// TestClone: a clone goes on as the game would, with the same actions,
// and what is done to it does not show in the game.
func TestClone(t *testing.T) {
	for i := range 6 {
		g := NewGame(cave(), "clone"+string(rune('a'+i)))
		b := NewBot(BotCareful, uint64(i))
		for n := 0; n < 300*(i+1) && g.Over == ""; n++ {
			a, args := b.Next(g)
			if err := g.Do(a, args); err != nil {
				t.Fatal(err)
			}
		}
		if g.Over != "" {
			continue
		}
		c := g.Clone()
		before := state(g)
		if state(c) != before {
			t.Fatalf("run %d: the clone differs at once", i)
		}
		// The clone plays on: the game does not change.
		rc := rand.New(rand.NewPCG(uint64(i), 9))
		var actions [][]string
		for n := 0; n < 400 && c.Over == ""; n++ {
			a, args := randomAction(c, rc)
			if err := c.Do(a, args); err != nil {
				t.Fatal(err)
			}
			actions = append(actions, append([]string{a}, args...))
		}
		if state(g) != before {
			t.Fatalf("run %d: playing the clone changed the game", i)
		}
		// The game, given the same actions, ends as the clone did.
		for _, a := range actions {
			if err := g.Do(a[0], a[1:]); err != nil {
				t.Fatal(err)
			}
		}
		if got, want := state(g), state(c); got != want {
			t.Errorf("run %d: the game and its clone part:\n%s\n---\n%s", i, got, want)
		}
	}
}
