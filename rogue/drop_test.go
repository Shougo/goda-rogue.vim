package rogue

import "testing"

// TestDrops: a thief of rank 1 always drops money, of rank 2 an item;
// another monster drops an item as often as its row says (in the cave a
// rat of rank 1 2%, a dragon of rank 3 16%).
func TestDrops(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	kill := func(id string) []*Item {
		t.Helper()
		before := len(g.Items)
		m := place(g, kind(id), Point{r.X + 2, r.Y + 1})
		g.remove(m)
		g.defeated(m)
		dropped := g.Items[before:]
		g.Items = g.Items[:before]
		return dropped
	}
	if its := kill("touzoku1"); len(its) != 1 || its[0].Def.Category != CategoryMoney || its[0].Amount <= 0 {
		t.Errorf("a 盗人 dropped %v, want money", its)
	}
	if its := kill("touzoku2"); len(its) != 1 || its[0].Def.Category == CategoryMoney {
		t.Errorf("a 盗賊 dropped %v, want an item", its)
	}
	for _, tc := range []struct {
		id      string
		low, hi int
	}{
		{"nezumi1", 5, 40},
		{"ryu3", 120, 200},
	} {
		n := 0
		for range 1000 {
			n += len(kill(tc.id))
		}
		if n < tc.low || n > tc.hi {
			t.Errorf("%s dropped %d items in 1000, want %d-%d", tc.id, n, tc.low, tc.hi)
		}
	}
}
