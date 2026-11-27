package rogue

import (
	"strings"
	"testing"
)

// TestLucky: a scroll that luck makes better does more by the chance of
// the dungeon, and always when blessed.
func TestLucky(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	w := carry(g, "bokutou", 0)
	w.Equipped = true

	// No luck: +1.
	used(t, g, "ten")
	if w.Upgrade != 1 {
		t.Fatalf("no luck: %+d", w.Upgrade)
	}
	// Blessed: +3, and the scroll stays, for +1 more.
	sc := carry(g, "ten", 0)
	sc.Blessed = true
	command(t, g, sc, commandRead)
	if w.Upgrade != 4 || !strings.Contains(strings.Join(g.Log, ""), "ラッキー！") {
		t.Fatalf("blessed: %+d, %q", w.Upgrade, g.Message)
	}
	command(t, g, sc, commandRead)
	if w.Upgrade != 5 {
		t.Fatalf("the scroll again: %+d", w.Upgrade)
	}
	// By chance: about one in 4.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.Lucky = 4 })
	s := carry(g, "kitate", 0)
	s.Equipped = true
	for range 400 {
		g.useItem(carry(g, "chi", 0))
	}
	if n := (s.Upgrade - 400) / luckyUpgrade; n < 70 || n > 130 {
		t.Errorf("lucky %d times of 400", n)
	}

	// Identification: everything carried, without asking; not when
	// there is nothing to identify.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.Lucky = 1 })
	a, b := carry(g, "katana", 2), carry(g, "fukitobashi", 0)
	a.Known, b.Known = false, false
	used(t, g, "shikibetsu")
	if g.Prompt != nil || !a.Known || !b.Known || !strings.Contains(g.Message, "持ち物がすべて識別された。") {
		t.Errorf("lucky identification: prompt %v, known %v %v, %q", g.Prompt, a.Known, b.Known, g.Message)
	}
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) {
		r.Lucky, r.Bless, r.CurseItem, r.CurseGear, r.CurseFood, r.CurseBracelet = 1, 0, 0, 0, 0, 0
	})
	g.Inventory = nil
	used(t, g, "shikibetsu")
	if !strings.Contains(g.Message, "識別するものはなかった。") || strings.Contains(g.Message, "ラッキー") {
		t.Errorf("nothing to identify: %q", g.Message)
	}
}
