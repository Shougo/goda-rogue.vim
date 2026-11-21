package rogue

import "testing"

// TestAllyFights: an ally goes for an awake enemy it sees near the
// player, lets a sleeping one sleep, and stays with the player otherwise.
func TestAllyFights(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	a := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	a.Ally = true
	e := place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 1})
	g.act(a)
	if d := chebyshev(a.Pos, e.Pos); d != 3 {
		t.Errorf("ally at %v, %d from the enemy: want a step toward it", a.Pos, d)
	}
	a.Pos = Point{r.X + 4, r.Y + 1}
	hp := e.HP
	for range 20 {
		if e.HP < hp || !contains(g.Monsters, e) {
			break
		}
		g.act(a)
	}
	if e.HP == hp && contains(g.Monsters, e) {
		t.Errorf("the ally never hit the enemy next to it")
	}

	// A sleeping enemy: the ally stays with the player.
	g, r = roomGame(t, 8, 3)
	a = place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	a.Ally = true
	place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 1}).Dozing = true
	g.act(a)
	if a.Pos != (Point{r.X + 1, r.Y + 1}) {
		t.Errorf("ally at %v, want it staying next to the player", a.Pos)
	}
}

func contains(ms []*Monster, m *Monster) bool {
	for _, x := range ms {
		if x == m {
			return true
		}
	}
	return false
}
