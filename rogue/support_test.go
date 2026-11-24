package rogue

import (
	"strings"
	"testing"
)

// TestSupport: a priest heals a hurt monster it sees; a shrine maiden
// ranks one up, not another shrine maiden.
func TestSupport(t *testing.T) {
	g, r := roomGame(t, 10, 4)
	g.Dungeon, _ = LoadBuiltin("far")
	p := place(g, kind("sou1"), Point{r.X + 6, r.Y})
	f := place(g, kind("nezumi1"), Point{r.X + 8, r.Y})
	a, _ := p.ability(AbilityHeal)
	if g.support(p) {
		t.Fatal("healed a monster not hurt")
	}
	f.HP = 1
	for range 100 {
		if g.support(p) {
			break
		}
	}
	if f.HP != min(1+a.Values[1], f.MaxHP) || !strings.Contains(g.Message, "小僧は鼠のHPを") {
		t.Errorf("HP %d, %q", f.HP, g.Message)
	}

	g.Monsters = nil
	m := place(g, kind("miko1"), Point{r.X + 6, r.Y})
	place(g, kind("miko1"), Point{r.X + 7, r.Y + 1})
	f = place(g, kind("nezumi1"), Point{r.X + 8, r.Y})
	for range 100 {
		if g.support(m) {
			break
		}
	}
	if f.Def.ID != "nezumi2" || !strings.Contains(g.Message, "巫女の祈りで鼠は大鼠になった！") {
		t.Errorf("%s, %q", f.Def.ID, g.Message)
	}
}
