package rogue

import (
	"slices"
	"strings"
	"testing"
)

// gold puts n G on p.
func gold(g *Game, n int, p Point) *Item {
	def := g.Dungeon.Item("okane")
	it := &Item{Def: def, Glyph: def.Glyph, Pos: p, Amount: n}
	g.Items = append(g.Items, it)
	return it
}

// TestGoldWalkedOn: money walked on goes to the purse, as before.
func TestGoldWalkedOn(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	gold(g, 30, Point{r.X + 1, r.Y})
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Gold != 30 || len(g.Inventory) != 0 || len(g.Items) != 0 {
		t.Errorf("gold %d, inventory %v, items %v", g.Gold, g.Inventory, g.Items)
	}
}

// TestGoldCarried: a dash stops on money without taking it; picked up (or
// swapped) from under the player, it is carried, and put in the purse with
// no turn.
func TestGoldCarried(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := gold(g, 30, Point{r.X + 2, r.Y})
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != it.Pos || g.Gold != 0 {
		t.Fatalf("at %v (gold at %v), gold %d", g.Player, it.Pos, g.Gold)
	}
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.Prompt.Choices, []string{"金   30G [足元]"}) {
		t.Fatalf("choices %q", g.Prompt.Choices)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if want := []string{commandPickUp, commandPurse, commandThrow, commandInfo}; !slices.Equal(g.Prompt.Choices, want) {
		t.Fatalf("choices %q, want %q", g.Prompt.Choices, want)
	}
	chooseLabel(t, g, commandPickUp)
	if g.Gold != 0 || len(g.Inventory) != 1 || g.Inventory[0] != it || len(g.Items) != 0 {
		t.Fatalf("gold %d, inventory %v, items %v", g.Gold, g.Inventory, g.Items)
	}

	// Carried: put in the purse, for no turn.
	turn := g.Turn
	command(t, g, it, commandPurse)
	if g.Gold != 30 || len(g.Inventory) != 0 || g.Turn != turn || !strings.Contains(g.Message, "30G を財布に入れた。") {
		t.Errorf("gold %d, inventory %v, turn %d, message %q", g.Gold, g.Inventory, g.Turn, g.Message)
	}

	// The action "pickup" carries it too; and from under the player it
	// can go to the purse directly.
	g.Gold = 0
	gold(g, 40, g.Player)
	if err := g.Do("pickup", nil); err != nil {
		t.Fatal(err)
	}
	if g.Gold != 0 || len(g.Inventory) != 1 {
		t.Fatalf("gold %d, inventory %v", g.Gold, g.Inventory)
	}
	gold(g, 50, g.Player)
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("choose", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	chooseLabel(t, g, commandPurse)
	if g.Gold != 50 || len(g.Items) != 0 || len(g.Inventory) != 1 {
		t.Errorf("gold %d, items %v, inventory %v", g.Gold, g.Items, g.Inventory)
	}

	// Swapped with an item carried.
	gold(g, 60, g.Player)
	grass := carry(g, "yakusou", 0)
	command(t, g, grass, commandSwap)
	if !slices.ContainsFunc(g.Inventory, func(x *Item) bool { return x.Amount == 60 }) || len(g.Items) != 1 || g.Items[0] != grass {
		t.Errorf("inventory %v, items %v", g.Inventory, g.Items)
	}
}

// TestGoldThrow: money thrown deals its amount / お金投げ, whatever the
// defense, and is used up; at least 1.
func TestGoldThrow(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	for _, c := range []struct{ amount, divisor, want int }{
		{300, 10, 30},
		{300, 5, 60},
		{5, 10, 1},
	} {
		g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate, r.GoldThrow = 100, c.divisor })
		m.HP, m.MaxHP, m.Pos = 9999, 9999, Point{r.X + 2, r.Y}
		it := gold(g, c.amount, g.Player)
		g.Items = nil
		g.Inventory = []*Item{it}
		command(t, g, it, commandThrow)
		if n := 9999 - m.HP; n != c.want || len(g.Inventory) != 0 || len(g.Items) != 0 {
			t.Errorf("%dG / %d: damage %d, want %d; inventory %v, items %v", c.amount, c.divisor, n, c.want, g.Inventory, g.Items)
		}
	}
}

// TestGoldInfo: 説明 tells the damage.
func TestGoldInfo(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	it := gold(g, 120, g.Player)
	lines := g.info(it)
	if lines[0] != "120G（お金）" || !strings.Contains(strings.Join(lines, "\n"), "12 のダメージ") {
		t.Errorf("%q", lines)
	}
}
