package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestRevive: defeated the first time, it gets up; the second time it
// is gone, and gives its EXP.
func TestRevive(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	m := place(g, kind("shikabane1"), Point{r.X + 1, r.Y})
	exp := g.EXP
	g.hurt(m, 999)
	if !slices.Contains(g.Monsters, m) || m.HP != m.MaxHP/2 || g.EXP != exp || !strings.Contains(g.Message, "起き上がった") {
		t.Fatalf("HP %d, EXP %d, %q", m.HP, g.EXP, g.Message)
	}
	g.hurt(m, 999)
	if slices.Contains(g.Monsters, m) || g.EXP == exp {
		t.Errorf("not gone: EXP %d", g.EXP)
	}
}

// TestExplode: hurt down to its share, it explodes at its next action:
// the player and a monster next to it lose half of their HP, items
// around burn, and it gives nothing.
func TestExplode(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 100, 100
	m := place(g, kind("bakudan1"), Point{r.X + 1, r.Y})
	f := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	f.HP = 40
	it := &Item{Def: g.Dungeon.Item("yakusou"), Pos: Point{r.X + 2, r.Y + 1}}
	g.Items = append(g.Items, it)
	exp := g.EXP
	g.hurt(m, m.HP-1)
	if !m.Fused || !strings.Contains(g.Message, "今にも爆発しそうだ") {
		t.Fatalf("not fused: %q", g.Message)
	}
	g.act(m)
	if slices.Contains(g.Monsters, m) || g.HP != 50 || f.HP != 20 || slices.Contains(g.Items, it) || g.EXP != exp {
		t.Errorf("HP %d, monster %d, items %v, EXP %d", g.HP, f.HP, g.Items, g.EXP)
	}
}
