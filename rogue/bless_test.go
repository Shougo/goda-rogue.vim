package rogue

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// TestBlessRoll: an item that can be cursed is blessed by the chance of
// the dungeon, never with a curse.
func TestBlessRoll(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	rng := rand.New(rand.NewPCG(3, 4))
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.CurseGear, r.CurseItem, r.Bless = 2, 2, 4 })
	blessed := map[string]int{}
	for range 800 {
		for _, id := range []string{"bokutou", "yakusou", "kiya"} {
			it := g.newItemOf(rng, g.Dungeon.Item(id), Point{})
			if it.Blessed && it.Cursed {
				t.Fatalf("%s both blessed and cursed", id)
			}
			if it.Blessed {
				blessed[id]++
			}
		}
	}
	// (One in 4 of those not cursed, half of them.)
	for id, want := range map[string]int{"bokutou": 100, "yakusou": 100, "kiya": 0} {
		if n := blessed[id]; n < want*3/4 || n > want*5/4 {
			t.Errorf("%s: %d blessed of 800, want about %d", id, n, want)
		}
	}
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.Bless = 0 })
	for range 100 {
		if g.newItemOf(rng, g.Dungeon.Item("yakusou"), Point{}).Blessed {
			t.Fatal("blessed with no chance")
		}
	}
}

// TestBlessedUse: a blessed grass, scroll or food is used without being
// used up, once; a blessed staff keeps its charges.
func TestBlessedUse(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 10, 200
	it := carry(g, "yakusou", 0)
	it.Blessed, it.Known = true, false
	if strings.Contains(g.describe(it), "祝") {
		t.Errorf("label %q before use", g.describe(it))
	}
	command(t, g, it, commandDrink)
	if g.HP != 60 || !slices.Contains(g.Inventory, it) || it.Blessed || !strings.Contains(g.Message, "祝福の力で薬草はなくならなかった。") {
		t.Errorf("first: HP %d, blessed %v, %q", g.HP, it.Blessed, g.Message)
	}
	command(t, g, it, commandDrink)
	if g.HP != 110 || slices.Contains(g.Inventory, it) {
		t.Errorf("second: HP %d, inventory %v", g.HP, g.Inventory)
	}

	food := carry(g, "onigiri", 0)
	food.Blessed = true
	if g.describe(food) != "おにぎり（祝）" || !slices.ContainsFunc(g.info(food), func(l string) bool { return strings.HasPrefix(l, "祝福されている") }) {
		t.Errorf("label %q, info %q", g.describe(food), g.info(food))
	}
	g.Fullness = 0
	command(t, g, food, commandEat)
	if g.Fullness == 0 || !slices.Contains(g.Inventory, food) || food.Blessed {
		t.Errorf("food: fullness %d, blessed %v", g.Fullness, food.Blessed)
	}
	g.Inventory = nil

	// A staff: its charges stay, and the blessing goes half of the time.
	g.Facing = "right"
	lost := 0
	for range 200 {
		staff := carry(g, "fukitobashi", 0)
		staff.Charges, staff.Blessed = 3, true
		g.useItem(staff)
		if staff.Charges != 3 {
			t.Fatalf("charges %d", staff.Charges)
		}
		if !staff.Blessed {
			lost++
		}
		g.Inventory = nil
	}
	if lost < 70 || lost > 130 {
		t.Errorf("the blessing went %d times of 200", lost)
	}

	// Thrown, a blessed grass has its effect and falls by the monster.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP = m.MaxHP - 10
	thrown := carry(g, "yakusou", 0)
	thrown.Blessed = true
	command(t, g, thrown, commandThrow)
	if m.HP != m.MaxHP || !slices.Contains(g.Items, thrown) || thrown.Blessed {
		t.Errorf("thrown: HP %d/%d, items %v, blessed %v", m.HP, m.MaxHP, g.Items, thrown.Blessed)
	}
}

// TestBlessingTakes: a blessing takes rust, rot and a curse in the place
// of its item, once.
func TestBlessingTakes(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	w := carry(g, "bokutou", 2)
	w.Blessed, w.Equipped = true, true
	s := carry(g, "kitate", 2)
	s.Equipped = true
	g.spring(&Trap{Def: g.Dungeon.Trap("sabi"), Pos: g.Player}, true)
	if w.Upgrade != 2 || w.Blessed || s.Upgrade != 1 || !strings.Contains(strings.Join(g.Log, ""), "木刀の祝福が身代わりになった。") {
		t.Errorf("rust: sword %+d (blessed %v), shield %+d, %q", w.Upgrade, w.Blessed, s.Upgrade, g.Message)
	}
	g.spring(&Trap{Def: g.Dungeon.Trap("sabi"), Pos: g.Player}, true)
	if w.Upgrade != 1 {
		t.Errorf("rust again: sword %+d", w.Upgrade)
	}

	a, b := carry(g, "onigiri", 0), carry(g, "onigiri", 0)
	a.Blessed = true
	g.rotTrap()
	if !a.onigiri() || a.Blessed || b.onigiri() {
		t.Errorf("rot: %s (blessed %v), %s", a.Def.Name, a.Blessed, b.Def.Name)
	}

	h := carry(g, "yakusou", 0)
	h.Blessed = true
	if g.curse(h) || h.Cursed || h.Blessed {
		t.Errorf("curse on a blessed item: cursed %v, blessed %v", h.Cursed, h.Blessed)
	}
	if !g.curse(h) || !h.Cursed {
		t.Error("curse on an item no more blessed")
	}
}

// TestBlessedPrice: twice the price, and the goods of a shop show it.
func TestBlessedPrice(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	it := carry(g, "yakusou", 0)
	it.Known = false
	full := price(it)
	it.Blessed = true
	if price(it) != 2*full || sellPrice(it) != full {
		t.Errorf("price %d, sold for %d, of %d", price(it), sellPrice(it), full)
	}
	it.ForSale = true
	if !strings.Contains(g.describe(it), "（祝）") {
		t.Errorf("label of goods %q", g.describe(it))
	}
}

// TestBlessedSynthesis: the blessing of either takes the curse of the
// other, and does not come along.
func TestBlessedSynthesis(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	base, it := carry(g, "bokutou", 0), carry(g, "bokutou", 0)
	base.Blessed, it.Cursed = true, true
	if !g.merge(base, it, false) || base.Cursed || base.Blessed {
		t.Errorf("cursed %v, blessed %v", base.Cursed, base.Blessed)
	}
}
