package rogue

import (
	"slices"
	"testing"
)

// read reads the scroll of the kind id, carried.
func read(g *Game, id string) {
	g.useItem(carry(g, id, 0))
}

// TestSlashScroll: 真空斬り hurts the enemies of the room, and kills
// those it can.
func TestSlashScroll(t *testing.T) {
	g, r := trapGame(t)
	weak := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	strong := place(g, kind("oni1"), Point{r.X + 4, r.Y + 2})
	hp := strong.HP
	exp := g.EXP
	read(g, "shinkuu")
	if slices.Contains(g.Monsters, weak) || g.EXP == exp {
		t.Errorf("the rat lives (EXP %d -> %d)", exp, g.EXP)
	}
	if strong.HP != hp-35 {
		t.Errorf("the oni's HP %d -> %d", hp, strong.HP)
	}
}

// TestWipeScroll: 全滅 kills the enemies of the room, not a shopkeeper.
func TestWipeScroll(t *testing.T) {
	g, r := trapGame(t)
	m := place(g, kind("oni1"), Point{r.X + 3, r.Y + 1})
	k := place(g, kind("oni1"), Point{r.X + 4, r.Y + 1})
	k.Keeper, k.Angry = true, true
	read(g, "zenmetsu")
	if slices.Contains(g.Monsters, m) || !slices.Contains(g.Monsters, k) {
		t.Errorf("monsters left: %v", g.Monsters)
	}
}

// TestSealScroll: 敵封印 seals the enemies of the room (a scroll of a
// bolt), and says so when looked at.
func TestSealScroll(t *testing.T) {
	g, r := trapGame(t)
	m := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	read(g, "tekifuuin")
	if !m.Sealed {
		t.Error("not sealed")
	}
	if got := effectText(g.Dungeon.Item("tekifuuin")); got != "読むと、部屋の敵（通路では隣の敵）を封印する（敵は特技が、自分は口が封じられる）。" {
		t.Errorf("info %q", got)
	}
}

// TestTrapsAndSeer: 罠消し takes the traps away; 千里眼 shows the monsters
// for the floor.
func TestTrapsAndSeer(t *testing.T) {
	g, r := trapGame(t)
	trap(g, "otoshiana", Point{r.X + 3, r.Y + 1})
	read(g, "wanakeshi")
	if len(g.Traps) != 0 {
		t.Error("traps left")
	}
	far := Point{g.Level.W - 2, g.Level.H - 2}
	if g.sensesMonster(far) {
		t.Fatal("sensed before")
	}
	read(g, "senrigan")
	if !g.sensesMonster(far) {
		t.Error("not sensed")
	}
	g.descend()
	if g.Seer {
		t.Error("still sensed on the next floor")
	}
}

// TestGatherScroll: 道具寄せ brings the items of the floor around the
// player, not the goods of a shop.
func TestGatherScroll(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X + 1, r.Y + 1}
	def := g.Dungeon.Item("otogiri")
	var free *Item
	for i, p := range []Point{{r.X + r.W - 1, r.Y + r.H - 1}, {r.X + r.W - 1, r.Y}} {
		it := &Item{Def: def, Glyph: def.Glyph, Pos: p, ForSale: i == 1}
		g.Items = append(g.Items, it)
		if i == 0 {
			free = it
		}
	}
	read(g, "douguyose")
	for _, it := range g.Items {
		if near := chebyshev(it.Pos, g.Player) <= 1; near == it.ForSale {
			t.Errorf("%v at %v (for sale %v)", it.Def.ID, it.Pos, it.ForSale)
		}
	}
	if free.Pos == (Point{r.X + r.W - 1, r.Y + r.H - 1}) {
		t.Error("not moved")
	}
}

// TestLostScroll: 迷子 sends the player away.
func TestLostScroll(t *testing.T) {
	g, _ := trapGame(t)
	from := g.Player
	read(g, "maigo")
	if g.Player == from {
		t.Error("not sent away")
	}
}

// TestItemScrolls: おにぎり, 壺増大, 吸い出し and 祝福 ask for an item and
// change it.
func TestItemScrolls(t *testing.T) {
	g, _ := trapGame(t)
	pot := carry(g, "hozon", 0)
	pot.Charges = 3
	pot.Contents = []*Item{{Def: g.Dungeon.Item("otogiri"), Glyph: "草"}, {Def: g.Dungeon.Item("otogiri"), Glyph: "草"}}
	sword := carry(g, "doutanuki", 0)
	sword.Cursed, sword.Equipped = true, true
	rotten := carry(g, "kusatta", 0)

	use := func(id string, target *Item) {
		t.Helper()
		read(g, id)
		choose(t, g, target)
	}

	use("onigirimaki", rotten)
	if rotten.Def.ID != "onigiri" {
		t.Errorf("おにぎり: %s", rotten.Def.ID)
	}
	use("onigirimaki", sword)
	if sword.Def.ID != "doutanuki" {
		t.Error("おにぎり: the equipped sword turned")
	}

	use("tsubozoudai", pot)
	if pot.Charges != 4 {
		t.Errorf("壺増大: %d", pot.Charges)
	}
	use("suidashi", pot)
	if len(pot.Contents) != 0 || len(g.Items) != 2 {
		t.Errorf("吸い出し: %d in the pot, %d on the floor", len(pot.Contents), len(g.Items))
	}

	use("shukufuku", sword)
	if sword.Cursed || sword.Blessed {
		t.Errorf("祝福 on a cursed sword: cursed %v, blessed %v", sword.Cursed, sword.Blessed)
	}
	use("shukufuku", sword)
	if !sword.Blessed {
		t.Error("祝福: not blessed")
	}
}

// TestPlateScroll: メッキ asks for any item, and puts 錆 on a weapon or a
// shield with room and no サビよけ yet; with it, nothing rusts. Unknown,
// it asks "どれに使う？" and is identified once it worked.
func TestPlateScroll(t *testing.T) {
	g, _ := trapGame(t)
	sword := carry(g, "doutanuki", 0)
	sword.Equipped = true
	gold := carry(g, "kintate", 0)
	delete(g.identified, "mekki")
	read(g, "mekki")
	if g.Prompt == nil || g.Prompt.Title != "どれに使う？" || g.identified["mekki"] {
		t.Fatalf("prompt %+v, identified %v", g.Prompt, g.identified["mekki"])
	}
	if len(g.Prompt.targets) != len(g.Inventory) {
		t.Fatalf("offered %d of %d", len(g.Prompt.targets), len(g.Inventory))
	}
	choose(t, g, gold)
	if len(gold.Seals) != 0 || g.identified["mekki"] {
		t.Fatalf("the gold shield (born with 錆): %v, identified %v", gold.Seals, g.identified["mekki"])
	}
	read(g, "mekki")
	choose(t, g, sword)
	if !slices.Equal(sword.Seals, []string{"錆"}) || !g.identified["mekki"] {
		t.Fatalf("seals %v, identified %v", sword.Seals, g.identified["mekki"])
	}
	for range 20 {
		g.rust([]Category{CategoryWeapon}, 1)
	}
	if sword.Upgrade != 0 {
		t.Errorf("rusted: %+d", sword.Upgrade)
	}
	// 錆 does not add up: not by メッキ again, nor merged twice.
	read(g, "mekki")
	choose(t, g, sword)
	g.addSeal(sword, "錆")
	if !slices.Equal(sword.Seals, []string{"錆"}) {
		t.Errorf("seals %v", sword.Seals)
	}
}

// TestChoosingScrollUnknown: a scroll that chooses, unknown and
// cancelled, stays unknown.
func TestChoosingScrollUnknown(t *testing.T) {
	g, _ := trapGame(t)
	carry(g, "hozon", 0)
	delete(g.identified, "tsubozoudai")
	read(g, "tsubozoudai")
	if g.Prompt == nil || g.Prompt.Title != "どれに使う？" {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("cancel", nil); err != nil {
		t.Fatal(err)
	}
	if g.identified["tsubozoudai"] {
		t.Error("identified, cancelled")
	}
}
