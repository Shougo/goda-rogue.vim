package rogue

import (
	"slices"
	"strings"
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

// TestGatherFoesScroll: 引き寄せ brings the enemies of the floor next to
// the player, not a shopkeeper.
func TestGatherFoesScroll(t *testing.T) {
	g, _ := trapGame(t)
	far := place(g, kind("nezumi1"), Point{})
	k := place(g, kind("oni1"), Point{})
	k.Keeper, k.Angry = true, true
	for i, m := range []*Monster{far, k} {
		p, ok := g.find(g.wanderTarget, needNoCreature, func(p Point) bool { return chebyshev(p, g.Player) > 5+i })
		if !ok {
			t.Fatal("no tile")
		}
		m.Pos = p
	}
	kp := k.Pos
	read(g, "hikiyose")
	if chebyshev(far.Pos, g.Player) != 1 || k.Pos != kp {
		t.Errorf("the rat at %v, the keeper %v -> %v (player %v)", far.Pos, kp, k.Pos, g.Player)
	}
}

// TestTroubleScroll: 困った時 heals in full and solves one trouble: with
// only hunger, the belly is full; surrounded, those next are bound.
func TestTroubleScroll(t *testing.T) {
	g, r := trapGame(t)
	g.HP = 1
	g.Fullness = 10
	read(g, "komatta")
	if g.HP != g.MaxHP || g.Fullness != g.fullnessCap() {
		t.Errorf("HP %d/%d, fullness %d", g.HP, g.MaxHP, g.Fullness)
	}
	g.Player = Point{r.X + 2, r.Y + 1}
	a := place(g, kind("oni1"), Point{r.X + 1, r.Y + 1})
	b := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	g.Invincible = true
	read(g, "komatta")
	if a.Bound == 0 || b.Bound == 0 {
		t.Fatalf("bound %d %d", a.Bound, b.Bound)
	}
	// Bound, an enemy does nothing, for good, until a blow frees it
	// (Shougo, 2026-11-27, as in Shiren).
	hp := g.HP
	for range 40 {
		if err := g.Do("wait", nil); err != nil {
			t.Fatal(err)
		}
	}
	if g.HP < hp || a.Bound == 0 || b.Bound == 0 {
		t.Fatalf("HP %d -> %d, bound %d %d", hp, g.HP, a.Bound, b.Bound)
	}
	g.Facing = "left"
	for range 100 {
		if a.Bound == 0 || !slices.Contains(g.Monsters, a) {
			break
		}
		if err := g.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
	}
	if a.Bound != 0 && slices.Contains(g.Monsters, a) {
		t.Fatal("not freed by blows")
	}
	if b.Bound == 0 {
		t.Error("the other freed too")
	}
}

// TestBoundAlly: an ally bound is free after its turns.
func TestBoundAlly(t *testing.T) {
	g, r := trapGame(t)
	m := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	m.Ally = true
	m.Bound = boundTurns
	for range boundTurns {
		g.Do("wait", nil)
	}
	if m.Bound != 0 {
		t.Errorf("still bound: %d", m.Bound)
	}
}

// TestPowerUpScroll: パワーアップ: blows of 1.5 times the damage, for its
// turns.
func TestPowerUpScroll(t *testing.T) {
	g, _ := trapGame(t)
	read(g, "powerup")
	if g.PowerUp != 50 || !strings.Contains(g.Status(), "パワーアップ") {
		t.Fatalf("power up %d: %q", g.PowerUp, g.Status())
	}
	for range 50 {
		g.Do("wait", nil)
	}
	if g.PowerUp != 0 {
		t.Errorf("still %d", g.PowerUp)
	}
}

// TestPreventScroll: 予防 cures, and wards the bad states off for the
// floor.
func TestPreventScroll(t *testing.T) {
	g, _ := trapGame(t)
	g.Confused = 5
	read(g, "yobou")
	if g.Confused != 0 || !g.Prevent {
		t.Fatalf("confused %d, prevent %v", g.Confused, g.Prevent)
	}
	g.afflict(MagicSleep)
	g.afflict(MagicBlind)
	if g.Asleep > 0 || g.Blind > 0 {
		t.Errorf("asleep %d, blind %d", g.Asleep, g.Blind)
	}
	g.descend()
	if g.Prevent {
		t.Error("prevent on the next floor")
	}
}

// TestBindScroll: 金縛り binds the enemies of the room.
func TestBindScroll(t *testing.T) {
	g, r := trapGame(t)
	m := place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 2})
	read(g, "kanashibari")
	if m.Bound == 0 {
		t.Error("not bound")
	}
}

// TestBottomlessPot: what is put in a pot of 底抜け is gone; thrown at a
// monster, it breaks and the monster falls through the pit it leaves.
func TestBottomlessPot(t *testing.T) {
	g, r := trapGame(t)
	pot := carry(g, "sokonuke", 0)
	pot.Charges = 3
	grass := carry(g, "otogiri", 0)
	g.putIn(pot, grass)
	if len(pot.Contents) != 0 || pot.Charges != 2 || slices.Contains(g.Inventory, grass) {
		t.Fatalf("contents %d, charges %d", len(pot.Contents), pot.Charges)
	}
	m := place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 2})
	g.smash(pot, m.Pos)
	if tr := g.TrapAt(Point{r.X + 4, r.Y + 2}); tr == nil || tr.Def.Effect != TrapPit || !tr.Found {
		t.Fatalf("trap %+v", tr)
	}
	if slices.Contains(g.Monsters, m) {
		t.Error("the rat did not fall")
	}
	// Broken under the player: it falls.
	depth := g.Depth
	pot = carry(g, "sokonuke", 0)
	g.smash(pot, g.Player)
	if g.Depth != depth+1 {
		t.Errorf("depth %d -> %d", depth, g.Depth)
	}
}
