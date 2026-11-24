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
