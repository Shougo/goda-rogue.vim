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

// TestAllyHitRate: an ally attacks at the dungeon's 仲間の命中, and an
// enemy attacks the ally at 敵の命中.
func TestAllyHitRate(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	d := *g.Dungeon
	d.Rules.AllyHitRate, d.Rules.MonsterHitRate = 0, 100
	g.Dungeon = &d
	a := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	a.Ally = true
	a.HP, a.MaxHP = 9999, 9999
	e := place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 1})
	e.HP, e.MaxHP = 9999, 9999
	for range 20 {
		g.hitMonster(a, e)
		g.hitMonster(e, a)
	}
	if e.HP != 9999 {
		t.Errorf("the ally hit at 仲間の命中 0: enemy HP %d", e.HP)
	}
	if a.HP == 9999 {
		t.Error("the enemy never hit the ally at 敵の命中 100")
	}
	d.Rules.AllyHitRate = 100
	g.hitMonster(a, e)
	if e.HP == 9999 {
		t.Error("the ally missed at 仲間の命中 100")
	}
}

// AllyHitRate defaults to 敵の命中 in a dungeon without 仲間の命中.
func TestAllyHitRateDefault(t *testing.T) {
	if r := frozen(t).Rules; r.AllyHitRate != r.MonsterHitRate {
		t.Errorf("仲間の命中 %d, want 敵の命中 %d", r.AllyHitRate, r.MonsterHitRate)
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
