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

// TestUncurse: the scroll takes the curses of everything carried away,
// in pots too; a blessing that of what it upgrades.
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

	used(t, g, "oharai")
	if s.Cursed || b.Cursed || in.Cursed || !strings.Contains(g.Message, "持ち物の呪いが解けた。") {
		t.Errorf("scroll: cursed %v %v %v, %q", s.Cursed, b.Cursed, in.Cursed, g.Message)
	}
	used(t, g, "oharai")
	if !strings.Contains(g.Message, "呪われたものはなかった。") {
		t.Errorf("nothing cursed: %q", g.Message)
	}
}

// TestCurseRoll: by the chance of the dungeon, weapons, shields and
// bracelets are cursed, with no upgrade value above 0; nothing else is,
// nor anything without a chance.
func TestCurseRoll(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	rng := rand.New(rand.NewPCG(1, 2))
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.CurseRate = 50 })
	cursed := 0
	for range 400 {
		for _, id := range []string{"bokutou", "kitate", "chikara", "yakusou", "hozon"} {
			it := g.newItemOf(rng, g.Dungeon.Item(id), Point{})
			if !it.Cursed {
				continue
			}
			cursed++
			if !it.cursable() || it.Upgrade > 0 {
				t.Fatalf("%s cursed, %+d", id, it.Upgrade)
			}
		}
	}
	if cursed < 500 || cursed > 700 {
		t.Errorf("%d cursed of 1200 at 50%%", cursed)
	}
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.CurseRate = 0 })
	for range 100 {
		if g.newItemOf(rng, g.Dungeon.Item("bokutou"), Point{}).Cursed {
			t.Fatal("cursed without a chance")
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
