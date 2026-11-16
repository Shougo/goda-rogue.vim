package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// item returns a new item id of the built-in dungeon at p.
func item(g *Game, id string, p Point) *Item {
	def := g.Dungeon.Item(id)
	it := &Item{Def: def, Pos: p, Glyph: def.Glyph}
	g.Items = append(g.Items, it)
	return it
}

func TestPickUpByWalking(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "yakusou", Point{r.X + 1, r.Y})
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 0 || !slices.Equal(g.Inventory, []*Item{it}) || !strings.Contains(g.Message, "薬草を拾った。") {
		t.Errorf("floor %v, inventory %v, %q", g.Items, g.Inventory, g.Message)
	}
}

func TestDashDoesNotPickUp(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "yakusou", Point{r.X + 2, r.Y})
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != it.Pos || len(g.Inventory) != 0 || !strings.Contains(g.Message, "薬草が落ちている。") {
		t.Fatalf("player %v, inventory %v, %q", g.Player, g.Inventory, g.Message)
	}
	turn := g.Turn
	if err := g.Do("pickup", nil); err != nil {
		t.Fatal(err)
	}
	if len(g.Inventory) != 1 || g.Turn != turn+1 {
		t.Errorf("pickup: inventory %v, turn %d", g.Inventory, g.Turn)
	}
	if err := g.Do("pickup", nil); err != nil {
		t.Fatal(err)
	}
	if g.Message != "足元には何もない。" || g.Turn != turn+1 {
		t.Errorf("pickup of nothing: %q, turn %d", g.Message, g.Turn)
	}
}

func TestMoney(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "okane", Point{r.X + 1, r.Y})
	it.Amount = 123
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Gold != 123 || len(g.Inventory) != 0 {
		t.Errorf("gold %d, inventory %v", g.Gold, g.Inventory)
	}
	if top := g.Render(Size{80, 24}).Lines[0]; !strings.HasSuffix(top, "  123G") {
		t.Errorf("top line %q", top)
	}
}

func TestCapacity(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	for range g.Dungeon.Rules.Capacity {
		g.Inventory = append(g.Inventory, &Item{Def: g.Dungeon.Item("yakusou"), Glyph: "草"})
	}
	item(g, "onigiri", Point{r.X + 1, r.Y})
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 1 || len(g.Inventory) != g.Dungeon.Rules.Capacity || !strings.Contains(g.Message, "持ち物がいっぱい") {
		t.Errorf("floor %d, inventory %d, %q", len(g.Items), len(g.Inventory), g.Message)
	}
}

func TestInventorySorted(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	for i, id := range []string{"onigiri", "yakusou", "tetsuken", "kitate", "bokutou"} {
		item(g, id, Point{r.X + 1 + i, r.Y})
	}
	for range 5 {
		if err := g.Do("move", []string{"right"}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, it := range g.Inventory {
		got = append(got, it.Label())
	}
	want := []string{"剣 木刀", "剣 鉄の剣", "盾 木の盾", "草 薬草", "食 おにぎり"}
	if !slices.Equal(got, want) {
		t.Errorf("inventory %q, want %q", got, want)
	}
}

func TestInventoryPrompt(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if g.Prompt != nil || g.Message != "何も持っていない。" {
		t.Fatalf("empty: %+v, %q", g.Prompt, g.Message)
	}
	item(g, "yakusou", Point{r.X + 1, r.Y})
	item(g, "onigiri", Point{r.X + 2, r.Y})
	for range 2 {
		if err := g.Do("move", []string{"right"}); err != nil {
			t.Fatal(err)
		}
	}

	turn := g.Turn
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	s := g.Render(Size{80, 24})
	if s.Prompt == nil || s.Prompt.Title != "持ち物" || !slices.Equal(s.Prompt.Choices, []string{"草 薬草", "食 おにぎり"}) {
		t.Fatalf("prompt %+v", s.Prompt)
	}
	if err := g.Do("choose", []string{"9"}); err == nil {
		t.Error("a choice out of range was accepted")
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Prompt == nil || g.Prompt.Title != "草 薬草" || !slices.Contains(g.Prompt.Choices, commandDrop) {
		t.Fatalf("item prompt %+v", g.Prompt)
	}
	// Under the player, there is nothing (it picked the onigiri up).
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Prompt != nil || len(g.Inventory) != 1 || g.Turn != turn+1 {
		t.Fatalf("drop: %+v, inventory %v, turn %d", g.Prompt, g.Inventory, g.Turn)
	}
	if it, ok := g.ItemAt(g.Player); !ok || it.Def.ID != "yakusou" {
		t.Errorf("under the player: %v", it)
	}

	// Cannot drop where something lies.
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := g.Do("choose", []string{"0"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.Inventory) != 1 || g.Message != "ここには置けない。" || g.Turn != turn+1 {
		t.Errorf("drop on an item: %v, %q, turn %d", g.Inventory, g.Message, g.Turn)
	}

	// Cancel, and any other action, drop the prompt.
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("cancel", nil); err != nil || g.Prompt != nil {
		t.Errorf("cancel: %v, %+v", err, g.Prompt)
	}
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("wait", nil); err != nil || g.Prompt != nil {
		t.Errorf("wait: %v, %+v", err, g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err == nil {
		t.Error("choose without a prompt was accepted")
	}
}

// carry gives the player the item id.
func carry(g *Game, id string, upgrade int) *Item {
	def := g.Dungeon.Item(id)
	it := &Item{Def: def, Glyph: def.Glyph, Upgrade: upgrade}
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	return it
}

// command chooses the item it of the inventory, and then the command c.
func command(t *testing.T, g *Game, it *Item, c string) {
	t.Helper()
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("choose", []string{fmt.Sprint(slices.Index(g.Inventory, it))}); err != nil {
		t.Fatal(err)
	}
	i := slices.Index(g.Prompt.Choices, c)
	if i < 0 {
		t.Fatalf("no %q in %q", c, g.Prompt.Choices)
	}
	if err := g.Do("choose", []string{fmt.Sprint(i)}); err != nil {
		t.Fatal(err)
	}
}

func TestEquip(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	bare := g.Attack()
	sword := carry(g, "tetsuken", 2)
	shield := carry(g, "kitate", 0)
	if g.Defense() != 0 {
		t.Fatalf("defense %d with nothing equipped", g.Defense())
	}

	turn := g.Turn
	command(t, g, sword, commandEquip)
	if !sword.Equipped || g.Turn != turn+1 {
		t.Fatalf("equipped %v, turn %d", sword.Equipped, g.Turn)
	}
	// At strength 8, a point of the sword is 0.75 + 8/32 = 1 of attack.
	if got, want := g.Attack(), bare+9; got != want {
		t.Errorf("attack %d, want %d", got, want)
	}
	if sword.Label() != "剣 鉄の剣+2 [装備]" {
		t.Errorf("label %q", sword.Label())
	}
	command(t, g, shield, commandEquip)
	if g.Defense() != 3 {
		t.Errorf("defense %d", g.Defense())
	}
	if !strings.HasPrefix(g.Status(), fmt.Sprintf("攻撃 %d  防御 3  ", bare+9)) {
		t.Errorf("status %q", g.Status())
	}

	// Another sword replaces the first.
	wood := carry(g, "bokutou", 0)
	command(t, g, wood, commandEquip)
	if sword.Equipped || !wood.Equipped || g.Attack() != bare+3 {
		t.Errorf("swap: %v %v, attack %d", sword.Equipped, wood.Equipped, g.Attack())
	}
	command(t, g, wood, commandUnequip)
	if wood.Equipped || g.Attack() != bare {
		t.Errorf("unequip: %v, attack %d", wood.Equipped, g.Attack())
	}

	// Dropping an equipped item unequips it.
	command(t, g, shield, commandDrop)
	if shield.Equipped || g.Defense() != 0 {
		t.Errorf("dropped: %v, defense %d", shield.Equipped, g.Defense())
	}
}

func TestShieldDefends(t *testing.T) {
	hurt := func(defense int) int {
		g, r := roomGame(t, 4, 3)
		g.HP, g.MaxHP = 99999, 99999
		if defense > 0 {
			carry(g, "tetsutate", defense-7).Equipped = true
		}
		place(g, kind("nezumi4"), Point{r.X + 1, r.Y})
		wait(t, g, 200)
		return 99999 + 200/g.Dungeon.Rules.RegenTurns - g.HP
	}
	if a, b := hurt(0), hurt(20); b*3 > a*2 {
		t.Errorf("damage %d with no shield, %d with 20", a, b)
	}
}

func TestUpgradeRoll(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	seen := map[int]bool{}
	for range 1000 {
		seen[rollUpgrade(rng)] = true
	}
	for _, n := range []int{-1, 0, 1, 2, 3} {
		if !seen[n] {
			t.Errorf("never %+d", n)
		}
	}
}

func TestBracelets(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	base, attack := g.CurrentStrength(), g.Attack()
	power := carry(g, "chikara", 0)
	command(t, g, power, commandEquip)
	if g.CurrentStrength() != base+3 || g.Attack() <= attack {
		t.Errorf("strength %d, attack %d with the bracelet", g.CurrentStrength(), g.Attack())
	}
	if !strings.Contains(g.Status(), fmt.Sprintf("  ちから %d  ", base+3)) {
		t.Errorf("status %q", g.Status())
	}
	if power.Label() != "腕 力の腕輪 [装備]" {
		t.Errorf("label %q", power.Label())
	}

	// One bracelet at a time.
	regen := carry(g, "kaifuku", 0)
	command(t, g, regen, commandEquip)
	if power.Equipped || g.CurrentStrength() != base {
		t.Errorf("both bracelets: strength %d", g.CurrentStrength())
	}

	// HP comes back twice as fast.
	restored := func() int {
		g.HP, g.MaxHP = 1, 999
		wait(t, g, 10*g.Dungeon.Rules.RegenTurns)
		return g.HP - 1
	}
	with := restored()
	command(t, g, regen, commandUnequip)
	if without := restored(); with != 2*without {
		t.Errorf("restored %d with the bracelet, %d without", with, without)
	}
}

func TestSensingBracelets(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	// A tile the player has not seen, outside the room.
	far := Point{-1, -1}
	for i, seen := range g.Seen {
		p := Point{i % g.Level.W, i / g.Level.W}
		if !seen && g.Level.At(p).Walkable() && g.Level.RoomAt(p) != g.Level.RoomAt(Point{r.X, r.Y}) {
			far = p
			break
		}
	}
	if far.X < 0 {
		t.Fatal("no unseen tile")
	}
	m := place(g, kind("nezumi1"), far)
	m.Dozing = true
	glyph := func() string { s, _ := g.glyph(far); return s }
	if glyph() != glyphUnknown {
		t.Fatalf("%q before", glyph())
	}
	command(t, g, carry(g, "kehai", 0), commandEquip)
	if glyph() != m.Glyph {
		t.Errorf("%q with 気配察知", glyph())
	}
	if c, _ := g.miniGlyph(far); c != miniMonster {
		t.Errorf("mini-map %q", c)
	}

	g.Monsters = nil
	it := item(g, "yakusou", far)
	if glyph() != glyphUnknown {
		t.Errorf("%q: 気配察知 shows an item", glyph())
	}
	command(t, g, carry(g, "dougu", 0), commandEquip)
	if glyph() != it.Glyph {
		t.Errorf("%q with 道具感知", glyph())
	}
}
