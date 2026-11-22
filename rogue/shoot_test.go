package rogue

import (
	"strings"
	"testing"
)

// shootGame is a room of one row with the player at its left end, every
// attack of the monsters hitting (or none, with hit false).
func shootGame(t *testing.T, hit bool) (*Game, Room) {
	t.Helper()
	g, r := roomGame(t, 12, 1)
	g.HP, g.MaxHP = 999, 999
	d := *g.Dungeon
	d.Rules.MonsterThrowRate = 0
	if hit {
		d.Rules.MonsterThrowRate = 100
	}
	g.Dungeon = &d
	return g, r
}

// fallenArrows returns the arrows on the floor.
func fallenArrows(g *Game) []*Item {
	var its []*Item
	for _, it := range g.Items {
		if it.Def.Category == CategoryArrow {
			its = append(its, it)
		}
	}
	return its
}

// TestShootAt: an archer on the player's line shoots it from afar; out of
// its line it walks.
func TestShootAt(t *testing.T) {
	g, r := shootGame(t, true)
	a := place(g, kind("shashu1"), Point{r.X + 4, r.Y})
	g.act(a)
	if a.Pos != (Point{r.X + 4, r.Y}) || g.HP >= 999 || !strings.Contains(g.Message, "木の矢を撃った") {
		t.Errorf("at %v, HP %d, %q: want a shot from where it stood", a.Pos, g.HP, g.Message)
	}
	if its := fallenArrows(g); len(its) != 0 {
		t.Errorf("%d arrows on the floor, want the one that hit used up", len(its))
	}

	g, r = roomGame(t, 12, 3)
	a = place(g, kind("shashu1"), Point{r.X + 4, r.Y + 2})
	g.act(a)
	if a.Pos == (Point{r.X + 4, r.Y + 2}) {
		t.Errorf("off the line, it stayed at %v", a.Pos)
	}
}

// TestShootMisses: an arrow that misses flies on and falls where it stops,
// unless it pierces.
func TestShootMisses(t *testing.T) {
	g, r := shootGame(t, false)
	g.Player = Point{r.X + 3, r.Y}
	a := place(g, kind("shashu1"), Point{r.X + 6, r.Y})
	g.act(a)
	its := fallenArrows(g)
	if g.HP != 999 || len(its) != 1 || its[0].Count != 1 || its[0].Def.ID != "kiya" {
		t.Fatalf("HP %d, arrows %v: want one 木の矢 fallen", g.HP, its)
	}
	if its[0].Pos.X >= g.Player.X {
		t.Errorf("the arrow fell at %v, want past the player at %v", its[0].Pos, g.Player)
	}

	g, r = shootGame(t, false)
	a = place(g, kind("shashu3"), Point{r.X + 6, r.Y})
	g.act(a)
	if its := fallenArrows(g); len(its) != 0 {
		t.Errorf("a piercing arrow fell: %v", its)
	}
}

// TestShootThrough: the arrow hits a monster in the way; one killed ranks
// the archer up. A piercing one hits the monster and the player.
func TestShootThrough(t *testing.T) {
	g, r := shootGame(t, true)
	a := place(g, kind("shashu1"), Point{r.X + 5, r.Y})
	m := place(g, kind("kame4"), Point{r.X + 2, r.Y})
	hp := m.HP
	g.act(a)
	if g.HP != 999 || m.HP >= hp {
		t.Errorf("HP %d, the turtle %d/%d: want the turtle hurt and not the player", g.HP, m.HP, hp)
	}

	m.HP = 1
	g.act(a)
	if contains(g.Monsters, m) || a.Def.ID != "shashu2" || !strings.Contains(g.Message, "射手は弓兵になった") {
		t.Errorf("the archer is %s (%q): want it ranked up after the kill", a.Def.ID, g.Message)
	}

	g, r = shootGame(t, true)
	a = place(g, kind("shashu3"), Point{r.X + 5, r.Y})
	m = place(g, kind("kame4"), Point{r.X + 2, r.Y})
	hp = m.HP
	g.act(a)
	if g.HP >= 999 || m.HP >= hp {
		t.Errorf("HP %d, the turtle %d/%d: want both hurt", g.HP, m.HP, hp)
	}
}

// TestArrowDrops: an archer drops arrows only.
func TestArrowDrops(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	d := *g.Dungeon
	def := *d.Monster("shashu1")
	def.Drop = &Drop{What: string(CategoryArrow), Rate: 100}
	for range 50 {
		before := len(g.Items)
		m := place(g, &def, Point{r.X + 2, r.Y + 1})
		g.remove(m)
		g.defeated(m)
		its := g.Items[before:]
		if len(its) != 1 || its[0].Def.Category != CategoryArrow {
			t.Fatalf("dropped %v, want an arrow", its)
		}
		g.Items = g.Items[:before]
	}
}
