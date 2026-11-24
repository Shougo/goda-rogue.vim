package rogue

import (
	"strings"
	"testing"
)

// TestLongHit: a giant hits the player two tiles away on its line, not
// past a creature, nor off its lines.
func TestLongHit(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("kyojin1"), Point{r.X + 3, r.Y + 1})
	if !g.longHit(m) {
		t.Error("no hit two tiles away")
	}
	f := place(g, kind("nezumi1"), Point{r.X + 2, r.Y + 1})
	if g.longHit(m) {
		t.Error("a hit past a creature")
	}
	g.remove(f)
	m.Pos = Point{r.X + 3, r.Y + 2}
	if g.longHit(m) {
		t.Error("a hit off its lines")
	}
}

// TestBlink: a ninja that sees the player appears next to it.
func TestBlink(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("shinobi4"), Point{r.X + 7, r.Y + 3})
	for range 100 {
		if g.blink(m) {
			break
		}
	}
	if chebyshev(m.Pos, g.Player) != 1 || !strings.Contains(g.Message, "目の前に現れた") {
		t.Errorf("at %v, %q", m.Pos, g.Message)
	}
}

// TestKnight: an eagle swoops on the player a knight's move away, and
// jumps as a knight toward it.
func TestKnight(t *testing.T) {
	g, r := roomGame(t, 10, 6)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("washi1"), Point{r.X + 2, r.Y + 3})
	if !g.swoop(m) || g.HP != 999-8 {
		t.Errorf("HP %d", g.HP)
	}
	m.Pos = Point{r.X + 7, r.Y + 5}
	from := m.Pos
	if !g.stepToward(m, g.Player) {
		t.Fatal("did not move")
	}
	d := m.Pos.Sub(from)
	if abs(d.X)+abs(d.Y) != 3 || d.X == 0 || d.Y == 0 {
		t.Errorf("moved by %v", d)
	}
}
