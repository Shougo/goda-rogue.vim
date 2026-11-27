package rogue

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// shopGame starts a run whose floors all have a shop, the shop id.
func shopGame(t *testing.T, id string) *Game {
	t.Helper()
	d := *cave()
	d.FloorRows = append([]FloorCounts(nil), d.FloorRows...)
	for i := range d.FloorRows {
		d.FloorRows[i].Shop = 100
		d.FloorRows[i].BigRoom, d.FloorRows[i].Maze, d.FloorRows[i].House = 0, 0, 0
	}
	d.ShopRows = []FloorRow{{First: 1, Last: d.Floors, ID: id, Weight: 1}}
	d.Rules.SpawnTurns = 1 << 30
	for seed := range 100 {
		g := NewGame(&d, "shop"+string(rune('a'+seed%26))+string(rune('0'+seed/26)))
		if g.Shop != nil && len(g.Shop.Keepers) > 0 {
			for _, it := range d.Items {
				g.identified[it.ID] = true
			}
			g.Monsters = append([]*Monster(nil), g.Shop.Keepers...)
			g.Traps = nil
			return g
		}
	}
	t.Fatal("no shop")
	return nil
}

// shopRoom is the room of the shop.
func (g *Game) shopRoom() Room { return g.Level.Rooms[g.Shop.Room] }

func forSale(g *Game) []*Item {
	var items []*Item
	for _, it := range g.Items {
		if it.ForSale {
			items = append(items, it)
		}
	}
	return items
}

func TestOpenShop(t *testing.T) {
	g := shopGame(t, "kusa")
	r := g.shopRoom()
	goods := forSale(g)
	if len(goods) == 0 {
		t.Fatal("nothing for sale")
	}
	for _, it := range goods {
		if !r.Contains(it.Pos) || it.Def.Category != CategoryGrass {
			t.Errorf("%s at %v", it.Def.ID, it.Pos)
		}
		if !strings.HasSuffix(g.Label(it), " "+itoa(price(it))+"G") {
			t.Errorf("label %q", g.Label(it))
		}
	}
	for _, k := range g.Shop.Keepers {
		if !k.Keeper || k.hostile() || !r.Contains(k.Pos) {
			t.Errorf("shopkeeper at %v, hostile %v", k.Pos, k.hostile())
		}
		if s, group := g.glyph(k.Pos); g.IsSeen(k.Pos) && (s != "商" || !strings.HasPrefix(group, hlKeeper)) {
			t.Errorf("shopkeeper shows as %q %q", s, group)
		}
	}
}

func itoa(n int) string {
	s := ""
	for {
		s = string(rune('0'+n%10)) + s
		n /= 10
		if n == 0 {
			return s
		}
	}
}

// pickUpGoods puts the player on an item for sale, and picks it up.
func pickUpGoods(t *testing.T, g *Game) *Item {
	t.Helper()
	it := forSale(g)[0]
	g.Player = it.Pos
	if !g.pickUp(true) {
		t.Fatal("cannot pick up")
	}
	return it
}

func TestBuy(t *testing.T) {
	g := shopGame(t, "yorozu")
	it := pickUpGoods(t, g)
	if !it.ForSale || g.debt() != price(it) || !strings.Contains(g.Message, "店主「") {
		t.Fatalf("for sale %v, debt %d, message %q", it.ForSale, g.debt(), g.Message)
	}
	// Using it before paying: no.
	if c := useCommands[it.Def.Category]; c != "" {
		command(t, g, it, c)
		if !strings.Contains(g.Message, "お代を払ってから") {
			t.Errorf("used before paying: %q", g.Message)
		}
	}
	// The shopkeeper goes to its door.
	k := g.Shop.Keepers[0]
	wait(t, g, 20)
	if k.Pos != k.door {
		t.Errorf("shopkeeper at %v, its door %v", k.Pos, k.door)
	}
	// Walking into it asks to pay.
	g.Gold = 0
	g.talkTo(k)
	if g.Prompt == nil || g.Prompt.kind != promptPay {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Message != "お金が足りない。" || !it.ForSale {
		t.Errorf("message %q", g.Message)
	}
	g.Gold = price(it) + 5
	g.talkTo(k)
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if it.ForSale || g.Gold != 5 || !strings.Contains(g.Message, "まいどありがとうございます") {
		t.Errorf("for sale %v, gold %d, message %q", it.ForSale, g.Gold, g.Message)
	}
	wait(t, g, 20)
	if k.Pos != k.home {
		t.Errorf("shopkeeper at %v, its place %v", k.Pos, k.home)
	}
}

func TestSell(t *testing.T) {
	g := shopGame(t, "yorozu")
	r := g.shopRoom()
	// A free tile of the shop.
	var spot Point
	found := false
	for y := r.Y; y < r.Y+r.H && !found; y++ {
		for x := r.X; x < r.X+r.W && !found; x++ {
			if p := (Point{x, y}); g.free(p) {
				spot, found = p, true
			}
		}
	}
	if !found {
		// A full shop: make room.
		spot = forSale(g)[0].Pos
		g.Items = g.Items[1:]
	}
	g.Player = spot
	grass := carry(g, "otogiri", 0)
	command(t, g, grass, commandDrop)
	if g.Prompt == nil || g.Prompt.kind != promptSell {
		t.Fatalf("prompt %+v, message %q", g.Prompt, g.Message)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Gold != price(grass)/2 || !grass.ForSale {
		t.Errorf("gold %d, for sale %v", g.Gold, grass.ForSale)
	}
}

// TestNoGoodsInPot: the goods not paid for cannot be put in a pot, out of
// the debt.
func TestNoGoodsInPot(t *testing.T) {
	g := shopGame(t, "yorozu")
	pot := g.newItemOf(g.rng, g.Dungeon.Item("hozon"), g.Player)
	g.Inventory = append(g.Inventory, pot)
	it := pickUpGoods(t, g)
	command(t, g, pot, commandPutIn)
	if g.Prompt != nil && slices.Contains(g.Prompt.Choices, g.Entry(it)) {
		t.Errorf("%q in %q", g.Entry(it), g.Prompt.Choices)
	}
	if g.debt() != price(it) {
		t.Errorf("debt %d", g.debt())
	}
}

// TestProvokeIsTheft: attacking a shopkeeper makes the player a thief,
// as in Shiren: the goods are free, and guards come.
func TestProvokeIsTheft(t *testing.T) {
	g := shopGame(t, "yorozu")
	pickUpGoods(t, g)
	g.provoke(g.Shop.Keepers[0])
	if g.debt() != 0 || len(forSale(g)) != 0 {
		t.Errorf("debt %d, %d for sale", g.debt(), len(forSale(g)))
	}
	checkThief(t, g)
}

func TestSteal(t *testing.T) {
	g := shopGame(t, "yorozu")
	it := pickUpGoods(t, g)
	// Out of the shop by a warp: a thief.
	trap(g, "warp", g.Player.Add(Point{0, 0}))
	g.Traps[len(g.Traps)-1].Pos = g.Player
	g.stepOn(g.Traps[len(g.Traps)-1])
	if g.inShop(g.Player) {
		t.Skip("warped into the shop")
	}
	if it.ForSale {
		t.Error("still for sale")
	}
	checkThief(t, g)
}

// checkThief checks that the player is a thief: told so, the shopkeepers
// angry, guards after it.
func checkThief(t *testing.T, g *Game) {
	t.Helper()
	if !strings.Contains(strings.Join(g.Log, ""), "泥棒だ！") {
		t.Errorf("log %q", g.Log)
	}
	guards := 0
	for _, m := range g.Monsters {
		if m.Def.Kind == KindGuard {
			guards++
			if !m.Hunter || !m.hostile() || m.EXP != 0 && !m.Spawned {
				t.Errorf("a guard: hunter %v", m.Hunter)
			}
		}
		if m.Keeper && (!m.Angry || !m.hostile()) {
			t.Error("a shopkeeper is not angry")
		}
	}
	if guards < 2 {
		t.Errorf("%d guards", guards)
	}
}

func TestAttackKeeper(t *testing.T) {
	g := shopGame(t, "yorozu")
	k := g.Shop.Keepers[0]
	g.HP, g.MaxHP = 99999, 99999
	// Next to it.
	p, ok := g.freeNeighbor(k.Pos)
	if !ok {
		t.Skip("no room")
	}
	g.Player = p
	for name, d := range Directions {
		if g.Player.Add(d) == k.Pos {
			g.Facing = name
		}
	}
	// The attack button talks to it, and takes no turn.
	turn := g.Turn
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if k.Angry || g.Turn != turn || !strings.Contains(g.Message, sayWelcome) {
		t.Fatalf("attacked: angry %v, turn %d to %d, %q", k.Angry, turn, g.Turn, g.Message)
	}
	// Owing: it asks to pay.
	goods := forSale(g)
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == goods[0] })
	g.Inventory = append(g.Inventory, goods[0])
	g.Do("attack", nil)
	if k.Angry || g.Prompt == nil || !strings.Contains(g.Prompt.Title, "お代は") {
		t.Fatalf("owing: angry %v, prompt %+v", k.Angry, g.Prompt)
	}
	g.Prompt = nil
	// A thrown item makes it angry.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	command(t, g, carry(g, "bokutou", 0), commandThrow)
	if !k.Angry || !strings.Contains(strings.Join(g.Log, ""), "店主を怒らせてしまった！") {
		t.Errorf("angry %v, message %q", k.Angry, g.Message)
	}
	// Without shopkeepers, the goods are free.
	// (A clone: removing one takes it out of the list.)
	for _, m := range slices.Clone(g.Shop.Keepers) {
		g.remove(m)
	}
	if len(forSale(g)) != 0 {
		t.Error("goods still for sale")
	}
}

// TestShopEntrance checks that the shopkeepers stand beside the ways in,
// never on them: the player can always walk in.
func TestShopEntrance(t *testing.T) {
	d := *cave()
	d.FloorRows = append([]FloorCounts(nil), d.FloorRows...)
	for i := range d.FloorRows {
		d.FloorRows[i].Shop = 100
	}
	shops := 0
	for seed := range 300 {
		g := NewGame(&d, "entrance"+itoa(seed))
		if g.Shop == nil {
			continue
		}
		shops++
		for _, k := range g.Shop.Keepers {
			if g.isEntry(g.Shop.Room, k.Pos) {
				t.Errorf("seed %d: shopkeeper on the way in at %v", seed, k.Pos)
			}
			if chebyshev(g.entry(g.Shop.Room, k.door), k.Pos) != 1 {
				t.Errorf("seed %d: shopkeeper at %v, not beside its door %v", seed, k.Pos, k.door)
			}
		}
	}
	if shops < 50 {
		t.Fatalf("only %d shops", shops)
	}
}

// TestNoSwapInShop: in a shop, putting down and picking up are selling
// and buying, so no swap.
func TestNoSwapInShop(t *testing.T) {
	g := shopGame(t, "yorozu")
	goods := g.Items[slices.IndexFunc(g.Items, func(it *Item) bool { return it.ForSale })]
	g.Player = goods.Pos
	carry(g, "yakusou", 0)
	if slices.Contains(g.footCommands(goods), commandSwap) || slices.Contains(g.itemCommands(g.Inventory[0]), commandSwap) {
		t.Errorf("swap offered in a shop: %q %q", g.footCommands(goods), g.itemCommands(g.Inventory[0]))
	}
}

// TestShopPrices: in a shop, the inventory shows what the shop pays, and
// the description of an item not identified the kinds it may be by its
// price.
func TestShopPrices(t *testing.T) {
	g := shopGame(t, "yorozu")
	r := g.shopRoom()
	g.Player = Point{r.X, r.Y}
	grass := carry(g, "otogiri", 0)
	// Two more grasses of its price: one identified, one not.
	for _, id := range []string{"yakusou", "chikarakusa"} {
		g.Dungeon.Item(id).Price = grass.Def.Price
	}
	g.identified["otogiri"], g.identified["chikarakusa"] = false, false
	want := fmt.Sprintf(" 売%dG", sellPrice(grass))
	if entries, _ := g.inventory(); !strings.HasSuffix(entries[len(g.Inventory)-1], want) {
		t.Errorf("%q", entries)
	}
	info := strings.Join(g.info(grass), "\n")
	for _, s := range []string{fmt.Sprintf("売値 %dG", sellPrice(grass)), "値段からの候補：上薬草、ちからの草"} {
		if !strings.Contains(info, s) {
			t.Errorf("no %q in\n%s", s, info)
		}
	}

	// Out of the shop: nothing of it.
	g.Player = Point{-1, -1}
	if entries, _ := g.inventory(); strings.Contains(entries[len(g.Inventory)-1], "売") {
		t.Errorf("%q", entries)
	}
	if info := strings.Join(g.info(grass), "\n"); strings.Contains(info, "候補") {
		t.Errorf("%s", info)
	}
}

// TestAllowedForSale: an item of the shop not paid for may be read about
// (説明), put down and equipped, not used; in a pot of the shop, its
// items may be taken out only.
func TestAllowedForSale(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	it := carry(g, "yakusou", 0)
	it.ForSale = true
	for c, want := range map[string]bool{commandInfo: true, commandDrop: true, commandDrink: false, commandThrow: false} {
		if ok, _ := g.allowed(it, nil, placeCarried, c); ok != want {
			t.Errorf("%s: allowed %v, want %v", c, ok, want)
		}
	}
	pot := carry(g, "hozon", 0)
	pot.ForSale = true
	in := &Item{Def: g.Dungeon.Item("yakusou")}
	if ok, _ := g.allowed(in, pot, placePot, commandTakeOut); !ok {
		t.Error("taking out of a pot of the shop")
	}
	if ok, _ := g.allowed(in, pot, placePot, commandDrink); ok {
		t.Error("drinking in a pot of the shop")
	}
}

// TestShopGoods: a shop sells as many goods as its kind says, in a block
// in the middle of its room; without a number, on every tile.
func TestShopGoods(t *testing.T) {
	count := func(g *Game) (n int, r Room) {
		r = g.Level.Rooms[g.Shop.Room]
		for _, it := range g.Items {
			if it.ForSale {
				n++
				if !r.Contains(it.Pos) {
					t.Errorf("goods at %v, out of the shop", it.Pos)
				}
			}
		}
		return n, r
	}
	g := shopGame(t, "yorozu")
	n, r := count(g)
	if def := g.Shop.Def; n == 0 || n > def.Goods[1] || (n < def.Goods[0] && r.W*r.H >= 30) {
		t.Fatalf("%d goods in a room of %dx%d, want %d-%d", n, r.W, r.H, def.Goods[0], def.Goods[1])
	}
	// In the middle: no farther from it than a tile left free.
	far := func(p Point) int { return max(abs(2*p.X-(2*r.X+r.W-1)), abs(2*p.Y-(2*r.Y+r.H-1))) }
	worst := 0
	for _, it := range g.Items {
		if it.ForSale {
			worst = max(worst, far(it.Pos))
		}
	}
	doors := g.doors(g.Shop.Room)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			p := Point{x, y}
			_, item := g.ItemAt(p)
			if !item && g.MonsterAt(p) == nil && far(p) < worst &&
				!slices.ContainsFunc(doors, func(d Point) bool { return chebyshev(d, p) <= 1 }) {
				t.Errorf("%v is free, nearer to the middle than some goods", p)
			}
		}
	}
}

// TestKeeperBlocks: owing the shop, the player finds each shopkeeper in
// its door at once, and each door has one.
func TestKeeperBlocks(t *testing.T) {
	g := shopGame(t, "yorozu")
	if n := len(g.doors(g.Shop.Room)); len(g.Shop.Keepers) < n {
		t.Fatalf("%d shopkeepers for %d doors", len(g.Shop.Keepers), n)
	}
	r := g.shopRoom()
	g.Player = Point{r.X + r.W/2, r.Y + r.H/2}
	goods := forSale(g)
	if len(goods) == 0 {
		t.Fatal("no goods")
	}
	it := goods[0]
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	g.Inventory = append(g.Inventory, it)
	if err := g.Do("wait", nil); err != nil {
		t.Fatal(err)
	}
	for _, k := range g.Shop.Keepers {
		if k.Pos != k.door {
			t.Errorf("a shopkeeper at %v, its door at %v", k.Pos, k.door)
		}
	}
	if !strings.Contains(strings.Join(g.Log, ""), "店主が入り口に立ちふさがった。") {
		t.Errorf("%q", g.Log)
	}
}
