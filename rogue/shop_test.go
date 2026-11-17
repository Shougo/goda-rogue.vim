package rogue

import (
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
	if !g.pickUp() {
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
	if it.ForSale || !strings.Contains(strings.Join(g.Log, ""), "泥棒だ！") {
		t.Errorf("for sale %v, log %q", it.ForSale, g.Log)
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
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if !k.Angry || !strings.Contains(g.Message, "店主を怒らせてしまった！") {
		t.Errorf("angry %v, message %q", k.Angry, g.Message)
	}
	// Without shopkeepers, the goods are free.
	for _, m := range g.Shop.Keepers {
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
