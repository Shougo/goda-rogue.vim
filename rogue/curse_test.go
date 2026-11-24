package rogue

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// TestCurse: a cursed item, once equipped, does not come off: no taking
// off, no other in its place, no dropping, throwing nor putting in a pot.
func TestCurse(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	w := carry(g, "bokutou", -1)
	w.Cursed, w.Known = true, false
	other := carry(g, "katana", 0)
	p := pot(g, "hozon", 3)
	if strings.Contains(g.describe(w), "呪") {
		t.Errorf("label %q before equipping", g.describe(w))
	}
	command(t, g, w, commandEquip)
	if !w.Equipped || !strings.Contains(g.Message, "木刀は呪われていた！") || g.describe(w) != "木刀-1（呪）" {
		t.Fatalf("equipped %v, %q, label %q", w.Equipped, g.Message, g.describe(w))
	}
	for _, c := range []string{commandUnequip, commandDrop, commandThrow} {
		turn := g.Turn
		command(t, g, w, c)
		if !w.Equipped || !slices.Contains(g.Inventory, w) || g.Turn != turn || !strings.Contains(g.Message, "木刀は呪われていて外せない！") {
			t.Fatalf("%s: equipped %v, turn %d to %d, %q", c, w.Equipped, turn, g.Turn, g.Message)
		}
	}
	command(t, g, other, commandEquip)
	if other.Equipped || !w.Equipped || !strings.Contains(g.Message, "木刀は呪われていて外せない！") {
		t.Errorf("another weapon: equipped %v, %q", other.Equipped, g.Message)
	}
	command(t, g, p, commandPutIn)
	if slices.Contains(g.Prompt.Choices, g.Entry(w)) {
		t.Errorf("the pot takes it: %q", g.Prompt.Choices)
	}
	g.Prompt = nil
	if !slices.Contains(g.info(w), "呪われている（装備すると外せない）") {
		t.Errorf("info %q", g.info(w))
	}

	// Not equipped, it is an item as any other.
	g, _ = roomGame(t, 4, 3)
	w = carry(g, "bokutou", 0)
	w.Cursed = true
	command(t, g, w, commandDrop)
	if slices.Contains(g.Inventory, w) {
		t.Error("a cursed weapon not equipped cannot be put down")
	}
}

// TestUncurse: the scroll takes the curse of an item carried away (of
// everything, in pots too, when lucky); a blessing that of what it
// upgrades.
func TestUncurse(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	w := carry(g, "bokutou", -1)
	s := carry(g, "kitate", 0)
	b := carry(g, "chikara", 0)
	in := carry(g, "katana", 0)
	p := pot(g, "hozon", 3)
	for _, it := range []*Item{w, s, b, in} {
		it.Cursed = true
	}
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == in })
	p.Contents = []*Item{in}
	command(t, g, w, commandEquip)
	command(t, g, s, commandEquip)

	used(t, g, "ten")
	if w.Cursed || w.Upgrade != 0 || !s.Cursed || !strings.Contains(g.Message, "木刀の呪いが解けた。") {
		t.Fatalf("blessing: weapon cursed %v %+d, shield cursed %v, %q", w.Cursed, w.Upgrade, s.Cursed, g.Message)
	}
	command(t, g, w, commandUnequip)
	if w.Equipped {
		t.Error("the weapon does not come off without its curse")
	}

	// The scroll takes the curse of the item chosen away, and of what is
	// in it.
	sc := carry(g, "oharai", 0)
	command(t, g, sc, commandRead)
	if g.Prompt == nil || g.Prompt.Title != "どれの呪いを解く？" {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	chooseLabel(t, g, g.Entry(p))
	if in.Cursed || !s.Cursed || !b.Cursed || !strings.Contains(g.Message, "保存の壺の呪いが解けた。") {
		t.Errorf("scroll on the pot: cursed %v %v %v, %q", in.Cursed, s.Cursed, b.Cursed, g.Message)
	}
	command(t, g, carry(g, "oharai", 0), commandRead)
	chooseLabel(t, g, g.Entry(p))
	if !strings.Contains(g.Message, "保存の壺は呪われていなかった。") {
		t.Errorf("nothing cursed: %q", g.Message)
	}

	// Lucky, of everything carried.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.Lucky = 1 })
	used(t, g, "oharai")
	if s.Cursed || b.Cursed || g.Prompt != nil || !strings.Contains(strings.Join(g.Log, ""), "ラッキー！持ち物の呪いが解けた。") {
		t.Errorf("lucky scroll: cursed %v %v, %q", s.Cursed, b.Cursed, g.Message)
	}
}

// TestCurseRoll: one in so many weapons and shields, and one in so many
// bracelets, are cursed, with no upgrade value above 0; nothing else is,
// nor anything with no chance.
func TestCurseRoll(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	rng := rand.New(rand.NewPCG(1, 2))
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.CurseGear, r.CurseBracelet, r.CurseItem, r.CurseFood = 4, 2, 8, 8 })
	cursed := map[string]int{}
	for range 800 {
		for _, id := range []string{"bokutou", "kitate", "chikara", "yakusou", "hozon", "onigiri", "kiya"} {
			it := g.newItemOf(rng, g.Dungeon.Item(id), Point{})
			if !it.Cursed {
				continue
			}
			cursed[id]++
			if !it.cursable() || it.Upgrade > 0 {
				t.Fatalf("%s cursed, %+d", id, it.Upgrade)
			}
		}
	}
	for id, want := range map[string]int{"bokutou": 200, "kitate": 200, "chikara": 400, "yakusou": 100, "hozon": 100, "onigiri": 100, "kiya": 0} {
		if n := cursed[id]; n < want*3/4 || n > want*5/4 {
			t.Errorf("%s: %d cursed of 800, want about %d", id, n, want)
		}
	}
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.CurseGear, r.CurseBracelet, r.CurseItem, r.CurseFood = 0, 0, 0, 0 })
	for range 100 {
		if g.newItemOf(rng, g.Dungeon.Item("bokutou"), Point{}).Cursed || g.newItemOf(rng, g.Dungeon.Item("chikara"), Point{}).Cursed ||
			g.newItemOf(rng, g.Dungeon.Item("yakusou"), Point{}).Cursed || g.newItemOf(rng, g.Dungeon.Item("onigiri"), Point{}).Cursed {
			t.Fatal("cursed with no chance")
		}
	}
}

// TestCurseSynthesis: a curse comes along into the base.
func TestCurseSynthesis(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "gousei", 3)
	base := carry(g, "katana", 0)
	other := carry(g, "bokutou", -1)
	other.Cursed = true
	for _, it := range []*Item{base, other} {
		command(t, g, p, commandPutIn)
		chooseLabel(t, g, g.Entry(it))
	}
	if !base.Cursed || base.Upgrade != -1 {
		t.Errorf("base cursed %v, %+d", base.Cursed, base.Upgrade)
	}
}

// TestCursedUse: a cursed item that is not worn cannot be used: the turn
// passes, nothing happens, and the player knows the curse from then on.
// It can still be put down, thrown (without its effect) and put in a pot.
func TestCursedUse(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 10, 100
	g.Facing = "right"
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	for id, c := range map[string]string{"yakusou": commandDrink, "akari": commandRead, "fukitobashi": commandWave, "onigiri": commandEat} {
		it := carry(g, id, 0)
		it.Cursed, it.Known, it.Charges = true, false, 3
		if strings.Contains(g.describe(it), "呪") || !g.unknown(it) {
			t.Errorf("%s: label %q, unknown %v before use", id, g.describe(it), g.unknown(it))
		}
		turn, full := g.Turn, g.Fullness
		command(t, g, it, c)
		if g.Turn != turn+1 || !slices.Contains(g.Inventory, it) || g.HP != 10 || g.Fullness > full || it.Charges != 3 ||
			!strings.Contains(g.Message, it.Def.Name+"は呪われていて使えない！") {
			t.Errorf("%s: turn %d to %d, HP %d, charges %d, %q", id, turn, g.Turn, g.HP, it.Charges, g.Message)
		}
		// (The charges of a staff stay unknown.)
		if !strings.HasSuffix(g.describe(it), "（呪）") || g.unknown(it) != (c == commandWave) || !slices.Contains(g.info(it), "呪われている（使えない）") {
			t.Errorf("%s: label %q, unknown %v, info %q after use", id, g.describe(it), g.unknown(it), g.info(it))
		}
		g.Inventory = nil
	}

	// A cursed pot takes nothing.
	p := pot(g, "hozon", 3)
	p.Cursed, p.Known = true, false
	grass := carry(g, "yakusou", 0)
	grass.Cursed = true
	turn := g.Turn
	command(t, g, p, commandPutIn)
	if g.Prompt != nil || g.Turn != turn+1 || len(p.Contents) != 0 || !strings.Contains(g.Message, "保存の壺は呪われていて使えない！") {
		t.Errorf("cursed pot: prompt %v, turn %d to %d, %q", g.Prompt, turn, g.Turn, g.Message)
	}
	// A cursed item goes in a pot as any other.
	p.Cursed = false
	command(t, g, p, commandPutIn)
	chooseLabel(t, g, g.Entry(grass))
	if !slices.Contains(p.Contents, grass) {
		t.Errorf("a cursed grass does not go in a pot: %q", g.Message)
	}

	// Thrown, it does not have its effect: it hurts a little and falls.
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP = m.MaxHP - 10
	thrown := carry(g, "yakusou", 0)
	thrown.Cursed = true
	command(t, g, thrown, commandThrow)
	if m.HP >= m.MaxHP-10 || !slices.Contains(g.Items, thrown) {
		t.Errorf("thrown: HP %d/%d, items %v, %q", m.HP, m.MaxHP, g.Items, g.Message)
	}
}

// TestCursedPrice: a shop sells and buys a cursed item at 87% of its
// price, and its goods show their curse.
func TestCursedPrice(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	it := carry(g, "yakusou", 0)
	it.Known = false
	full := price(it)
	it.Cursed = true
	if price(it) != full*87/100 || sellPrice(it) != full*87/100/2 {
		t.Errorf("price %d, sold for %d, of %d", price(it), sellPrice(it), full)
	}
	if strings.Contains(g.describe(it), "呪") {
		t.Errorf("label %q", g.describe(it))
	}
	it.ForSale = true
	if !strings.Contains(g.describe(it), "（呪）") {
		t.Errorf("label of goods %q", g.describe(it))
	}
}
