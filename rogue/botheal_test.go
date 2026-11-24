package rogue

import (
	"slices"
	"testing"
)

// TestBotHealWorth: a healing grass is worth the share of the max HP it
// heals of a full heal.
func TestBotHealWorth(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	b := NewBot(BotCareful, 1)
	herb, big := g.Dungeon.Item("yakusou"), g.Dungeon.Item("otogiri")
	full := b.healPrice(g)
	g.MaxHP = 40
	if b.kindPrice(g, herb) != full || b.kindPrice(g, big) != full {
		t.Errorf("max HP 40: %v %v, want %v", b.kindPrice(g, herb), b.kindPrice(g, big), full)
	}
	g.MaxHP = 200
	if b.kindPrice(g, herb) != full/4 || b.kindPrice(g, big) != full/2 {
		t.Errorf("max HP 200: %v %v", b.kindPrice(g, herb), b.kindPrice(g, big))
	}
}

// TestBotBestHeal: the bot heals with what fits what is missing: the
// cheapest of what heals it all, or what heals most; a pot of healing
// with the item worth least in it.
func TestBotBestHeal(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	b := NewBot(BotCareful, 1)
	g.MaxHP = 200
	herb, big := carry(g, "yakusou", 0), carry(g, "otogiri", 0)
	pot := pot(g, "kaifukutsubo", 2)
	junk := carry(g, "yakusou", 0)
	junk.Cursed, junk.CurseTold = true, true
	for _, c := range []struct {
		hp   int
		want *Item
	}{{160, herb}, {120, big}, {50, pot}} {
		g.HP = c.hp
		h, ok := b.bestHeal(g)
		if !ok || h.item != c.want {
			t.Errorf("HP %d: %v", c.hp, h.item)
		}
	}
	h, _ := b.bestHeal(g)
	if h.fuel != junk {
		t.Errorf("fuel %v", h.fuel)
	}
	// Played: the pot heals, the cursed grass is gone.
	g.HP = 50
	b.useHeal(h)
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	for g.Prompt != nil {
		a, args := b.Next(g)
		if err := g.Do(a, args); err != nil {
			t.Fatal(err)
		}
	}
	if g.HP != g.MaxHP || slices.Contains(g.Inventory, junk) || pot.Charges != 1 {
		t.Errorf("HP %d/%d, charges %d, %q", g.HP, g.MaxHP, pot.Charges, g.Message)
	}
}

// TestBotAnswersNewMonsters: in danger, the bot throws food at a hungry
// ghost next to it, and reads おはらい near a monster that curses.
func TestBotAnswersNewMonsters(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotReckless, 1)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("gaki2"), Point{r.X + 2, r.Y + 1})
	m.HP, m.MaxHP = 999, 999
	g.Inventory = nil
	food := carry(g, "onigiri", 0)
	g.HP = 3
	g.Facing = "right"
	if a, _ := b.Next(g); a != "inventory" || b.plan[0].item != food || b.plan[1].label != commandThrow {
		t.Errorf("ghost: %s %+v", a, b.plan)
	}

	g.Monsters, b.plan = nil, nil
	m = place(g, kind("noroi2"), Point{r.X + 2, r.Y + 1})
	m.HP, m.MaxHP = 999, 999
	g.Inventory = nil
	sc := carry(g, "oharai", 0)
	if a, _ := b.Next(g); a != "inventory" || b.plan[0].item != sc || b.plan[1].label != commandRead {
		t.Errorf("curser: %s %+v", a, b.plan)
	}
}
