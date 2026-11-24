package rogue

import "testing"

// forget makes the kinds of the category c unknown.
func forget(g *Game, c Category) {
	for _, def := range g.Dungeon.Items {
		if def.Category == c {
			delete(g.identified, def.ID)
		}
	}
}

// TestBotPriceIdentify: in a shop, the bot learns what an unknown item
// may be by its price; a price of one kind tells it.
func TestBotPriceIdentify(t *testing.T) {
	g := shopGame(t, "yorozu")
	forget(g, CategoryGrass)
	b := NewBot(BotCareful, 1)
	herb := carry(g, "yakusou", 0)
	if !b.unsure(g, herb) || b.known(g, herb, EffectHeal) {
		t.Fatal("known before the shop")
	}
	r := g.shopRoom()
	g.Player = Point{r.X + r.W/2, r.Y + r.H/2}
	b.Next(g)
	if b.unsure(g, herb) || !b.known(g, herb, EffectHeal) {
		t.Errorf("not known by its price: %v", b.mayBe(g, herb))
	}
	// A price of several kinds narrows them.
	eye := carry(g, "megusuri", 0)
	b.Next(g)
	for _, def := range b.mayBe(g, eye) {
		if def.Price != eye.Def.Price {
			t.Errorf("%s may be %s", eye.Def.Name, def.Name)
		}
	}
}

// TestBotEarlyIdentify: on the first floors the bot tries an unknown
// grass at 70% of its HP and reads an unknown scroll anywhere; deeper,
// only at full HP and on the stairs.
func TestBotEarlyIdentify(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	forget(g, CategoryGrass)
	forget(g, CategoryScroll)
	b := NewBot(BotCareful, 1)
	g.Inventory = nil
	grass := carry(g, "konran", 0)
	g.MaxHP, g.HP = 100, 80
	g.Depth = 2
	if a, _ := b.identify(g); a != "inventory" || b.plan[0].item != grass {
		t.Errorf("early, 80%%: %s %+v", a, b.plan)
	}
	g.Depth = g.Dungeon.Floors
	if a, _ := b.identify(g); a != "" {
		t.Errorf("deep, 80%%: %s %+v", a, b.plan)
	}
	g.Inventory = nil
	sc := carry(g, "akari", 0)
	g.Depth = 2
	if a, _ := b.identify(g); a != "inventory" || b.plan[0].item != sc {
		t.Errorf("early scroll: %s %+v", a, b.plan)
	}
	g.Depth = g.Dungeon.Floors
	if a, _ := b.identify(g); a != "" {
		t.Errorf("deep scroll off the stairs: %s %+v", a, b.plan)
	}
}

// TestBotUnsureLast: in danger, the bot uses an item it does not know
// only when nothing it knows helps.
func TestBotUnsureLast(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	forget(g, CategoryGrass)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("nezumi2"), Point{r.X + 2, r.Y + 1})
	m.HP, m.MaxHP = 999, 999
	g.Inventory = nil
	g.HP = 2
	grass := carry(g, "yakusou", 0)
	staff := carry(g, "fukitobashi", 0)
	staff.Charges = 3
	if a, _ := b.Next(g); a == "inventory" && b.plan[0].item == grass {
		t.Errorf("the unknown grass before the staff: %+v", b.plan)
	}
	g.Inventory = []*Item{grass}
	b.plan = nil
	if a, _ := b.Next(g); a != "inventory" || b.plan[0].item != grass {
		t.Errorf("the unknown grass alone: %s %+v", a, b.plan)
	}
}

// TestBotToShop: with an unknown item whose price it has not seen, the
// bot walks into a shop it has seen; then it goes on.
func TestBotToShop(t *testing.T) {
	g := shopGame(t, "yorozu")
	forget(g, CategoryGrass)
	b := NewBot(BotCareful, 1)
	r := g.shopRoom()
	carry(g, "yakusou", 0)
	for i := range g.Seen {
		g.Seen[i] = true
	}
	if g.inShop(g.Player) {
		t.Skip("starts in the shop")
	}
	if a, _ := b.toShop(g); a == "" {
		t.Fatal("not to the shop")
	}
	g.Player = Point{r.X + r.W/2, r.Y + r.H/2}
	b.Next(g)
	if a, _ := b.toShop(g); a != "" {
		t.Errorf("to the shop again: %s", a)
	}
}
