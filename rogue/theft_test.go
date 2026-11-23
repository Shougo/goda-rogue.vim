package rogue

import (
	"slices"
	"strings"
	"testing"
)

// thiefGame is a room with a thief of the kind id next to the player,
// awake.
func thiefGame(t *testing.T, id string) (*Game, *Monster) {
	t.Helper()
	g, r := roomGame(t, 12, 5)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 5, r.Y + 2}
	m := place(g, kind(id), Point{r.X + 6, r.Y + 2})
	return g, m
}

// TestStealGold: a thief of rank 1 takes half the purse, warps away,
// keeps away, and drops the money when defeated (not its own drop).
func TestStealGold(t *testing.T) {
	g, m := thiefGame(t, "touzoku1")
	g.Gold = 101
	g.act(m)
	if g.Gold != 50 || !m.Fled || !strings.Contains(g.Message, "盗人に51G を盗まれた！") {
		t.Fatalf("gold %d, fled %v, %q", g.Gold, m.Fled, g.Message)
	}
	// Away, and it keeps away.
	// (Where it can: in a corner it has no step away.)
	d := chebyshev(m.Pos, g.Player)
	if g.flee(m) && chebyshev(m.Pos, g.Player) <= d {
		t.Errorf("it did not get away: %d to %d", d, chebyshev(m.Pos, g.Player))
	}
	g.remove(m)
	g.defeated(m)
	if len(g.Items) != 1 || g.Items[0].Amount != 51 {
		t.Errorf("items %v: want the 51G", g.Items)
	}

	// An empty purse: it hits instead.
	g, m = thiefGame(t, "touzoku1")
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
	g.act(m)
	if m.Fled || g.HP == 999 {
		t.Errorf("fled %v, HP %d: want a blow", m.Fled, g.HP)
	}
}

// TestStealItems: a thief of rank 2 takes one carried item, not an
// equipped one; rank 4 takes two, equipped ones too; a sealed one does
// not steal.
func TestStealItems(t *testing.T) {
	g, m := thiefGame(t, "touzoku2")
	w := carry(g, "bokutou", 0)
	command(t, g, w, commandEquip)
	grass := carry(g, "yakusou", 0)
	g.act(m)
	if !slices.Contains(g.Inventory, w) || slices.Contains(g.Inventory, grass) || len(m.stolen) != 1 {
		t.Fatalf("inventory %v, stolen %v", g.Inventory, m.stolen)
	}
	// Defeated by another monster, it drops it all the same.
	g.kill(m, place(g, kind("oni1"), Point{0, 0}), false, false)
	if !slices.Contains(g.Items, grass) {
		t.Errorf("items %v: want the grass", g.Items)
	}

	g, m = thiefGame(t, "touzoku4")
	w = carry(g, "bokutou", 0)
	command(t, g, w, commandEquip)
	g.act(m)
	if len(g.Inventory) != 0 || len(m.stolen) != 1 || w.Equipped {
		t.Errorf("inventory %v, stolen %v: want the equipped sword taken", g.Inventory, m.stolen)
	}

	g, m = thiefGame(t, "touzoku2")
	carry(g, "yakusou", 0)
	m.Sealed = true
	g.act(m)
	if len(g.Inventory) != 1 || m.Fled {
		t.Errorf("a sealed thief stole: %v", m.stolen)
	}
}

// TestStealWhileIdentifying: a thief that takes an item while the prompt
// of a scroll of identification is open leaves the prompt safe.
func TestStealWhileIdentifying(t *testing.T) {
	g, m := thiefGame(t, "touzoku2")
	grass := carry(g, "yakusou", 0)
	g.identified["yakusou"] = false
	use(t, g, "shikibetsu")
	if g.Prompt == nil || slices.Contains(g.Inventory, grass) {
		t.Fatalf("prompt %+v, inventory %v: want the grass stolen in the turn", g.Prompt, g.Inventory)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.identified["yakusou"] || !strings.Contains(g.Message, "それはもう手元にない。") {
		t.Errorf("identified %v, %q", g.identified["yakusou"], g.Message)
	}
	_ = m
}

// TestStolenBumped: a thief blown into a wall by no one (a charge's
// fling) still drops what it stole as it dies.
func TestStolenBumped(t *testing.T) {
	g, m := thiefGame(t, "touzoku2")
	grass := carry(g, "yakusou", 0)
	g.act(m)
	if !slices.Contains(m.stolen, grass) {
		t.Fatalf("stolen %v", m.stolen)
	}
	m.HP = 1
	g.fling(m, Point{1, 0}, false)
	g.fling(m, Point{1, 0}, false)
	if slices.Contains(g.Monsters, m) || !slices.Contains(g.Items, grass) {
		t.Errorf("monsters %v, items %v: want the thief dead and the grass on the floor", g.Monsters, g.Items)
	}
}
