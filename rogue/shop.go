package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// ShopDef is a kind of shop.
type ShopDef struct {
	ID   string
	Name string
	// Goods are the categories of the items it sells (none: any).
	Goods []Category
	// Keeper is the shopkeeper, Keepers how many: from, to.
	Keeper  *MonsterDef
	Keepers [2]int
	// Guard is the guard that comes after a thief (nil for none), Guards
	// how many: from, to.
	Guard  *MonsterDef
	Guards [2]int
}

// Shop is the shop of a floor.
type Shop struct {
	Def  *ShopDef
	Room int
	// Keepers are its shopkeepers.
	Keepers []*Monster
}

// The prompts of a shop: paying, selling.
const (
	promptPay  = "pay"
	promptSell = "sell"
)

// The shopkeepers say.
const (
	sayWelcome = "店主「いらっしゃいませ。」"
	sayThanks  = "店主「まいどありがとうございます。」"
	sayThief   = "店主「泥棒だ！」"
)

// price is the price of it in a shop (arrows: all of them).
func price(it *Item) int {
	if it.Def.Category == CategoryArrow {
		return it.Def.Price * max(it.Count, 1)
	}
	return it.Def.Price
}

// sellPrice is what a shop pays for it.
func sellPrice(it *Item) int { return price(it) / 2 }

// hostile says whether m fights the player: an enemy, or a shopkeeper
// made angry.
func (m *Monster) hostile() bool {
	return !m.Ally && (!m.Keeper || m.Angry)
}

// openShop makes a room of the floor a shop, by its shop table: a room
// that is not where the player starts, nor the monster house, without
// the stairs, with a door. Its items are for sale; its shopkeepers stand
// inside, by its doors.
func (g *Game) openShop(rng *rand.Rand, rooms []int, start int) {
	var candidates []int
	for _, i := range rooms {
		r := g.Level.Rooms[i]
		if i != start && i != g.House && !r.Contains(g.Stairs) && r.W*r.H >= 6 && len(g.doors(i)) > 0 {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return
	}
	d := g.Dungeon
	room := candidates[rng.IntN(len(candidates))]
	def := d.Shop(pick(rng, d.ShopRows, g.Depth))
	rows := g.goodsRows(def)
	if len(rows) == 0 {
		// Nothing to sell here.
		return
	}
	r := g.Level.Rooms[room]
	// The shop has the room to itself.
	g.Monsters = slices.DeleteFunc(g.Monsters, func(m *Monster) bool { return r.Contains(m.Pos) })
	g.Items = slices.DeleteFunc(g.Items, func(it *Item) bool { return r.Contains(it.Pos) })
	g.Shop = &Shop{Def: def, Room: room}

	doors := g.doors(room)
	for i := range between(rng, def.Keepers[0], def.Keepers[1]) {
		door := doors[i%len(doors)]
		home := Point{min(max(door.X, r.X), r.X+r.W-1), min(max(door.Y, r.Y), r.Y+r.H-1)}
		if !g.free(home) {
			var ok bool
			if home, ok = g.freeNeighbor(home); !ok || !r.Contains(home) {
				continue
			}
		}
		m := newMonster(def.Keeper)
		m.Pos, m.Keeper, m.home, m.door = home, true, home, door
		g.Monsters = append(g.Monsters, m)
		g.Shop.Keepers = append(g.Shop.Keepers, m)
	}

	// The goods: on the tiles of the room, but next to the doors (the way
	// in).
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			p := Point{x, y}
			if !g.free(p) || slices.ContainsFunc(doors, func(d Point) bool { return chebyshev(d, p) <= 1 }) {
				continue
			}
			it := g.newItemOf(rng, d.Item(pick(rng, rows, g.Depth)), p)
			it.ForSale = true
			g.Items = append(g.Items, it)
		}
	}
}

// goodsRows are the rows of the shop item table of the floor for the shop
// def: its own, and those for any shop of the kinds it sells.
func (g *Game) goodsRows(def *ShopDef) []FloorRow {
	d := g.Dungeon
	var rows []FloorRow
	for _, r := range d.ShopItemRows {
		if r.First > g.Depth || g.Depth > r.Last {
			continue
		}
		c := d.Item(r.ID).Category
		if r.Shop == def.ID || r.Shop == "" && (len(def.Goods) == 0 || slices.Contains(def.Goods, c)) {
			rows = append(rows, r.FloorRow)
		}
	}
	return rows
}

// doors returns the doors of the room i: the walkable tiles of its wall.
func (g *Game) doors(i int) []Point {
	r := g.Level.Rooms[i]
	var doors []Point
	for y := r.Y - 1; y <= r.Y+r.H; y++ {
		for x := r.X - 1; x <= r.X+r.W; x++ {
			p := Point{x, y}
			if !r.Contains(p) && g.Level.At(p).Walkable() {
				doors = append(doors, p)
			}
		}
	}
	return doors
}

// inShop says whether p is in the shop of the floor.
func (g *Game) inShop(p Point) bool {
	return g.Shop != nil && g.Level.Rooms[g.Shop.Room].Contains(p)
}

// debt is what the player owes the shop: the items for sale it carries.
func (g *Game) debt() int {
	n := 0
	for _, it := range g.Inventory {
		if it.ForSale {
			n += price(it)
		}
	}
	return n
}

// actKeeper is a turn of a shopkeeper not angry: it stands at its place
// inside, or in its door while the player owes the shop.
func (g *Game) actKeeper(m *Monster) {
	to := m.home
	if g.debt() > 0 && g.inShop(g.Player) {
		to = m.door
	}
	if m.Pos != to {
		g.stepToward(m, to)
	}
}

// talkTo is the player walking into the shopkeeper m: it asks to pay what
// the player owes.
func (g *Game) talkTo(m *Monster) {
	debt := g.debt()
	if debt == 0 {
		g.say(sayWelcome)
		return
	}
	g.Prompt = &Prompt{
		Title:   fmt.Sprintf("店主「お代は %dG です。」", debt),
		Choices: []string{"払う", "やめる"},
		kind:    promptPay,
	}
}

// pay pays what the player owes, if it can.
func (g *Game) pay() {
	debt := g.debt()
	if g.Gold < debt {
		g.say("お金が足りない。")
		return
	}
	g.Gold -= debt
	for _, it := range g.Inventory {
		it.ForSale = false
	}
	g.say(fmt.Sprintf("%dG 払った。", debt))
	g.say(sayThanks)
}

// offer is the player putting it down in the shop: the shopkeeper offers
// to buy it.
func (g *Game) offer(it *Item) {
	if it.ForSale || sellPrice(it) == 0 || len(g.Shop.Keepers) == 0 || g.Shop.Keepers[0].Angry {
		return
	}
	g.selling = it
	g.Prompt = &Prompt{
		Title:   fmt.Sprintf("店主「%sを %dG で買い取りましょう。」", g.Name(it), sellPrice(it)),
		Choices: []string{"売る", "売らない"},
		kind:    promptSell,
	}
}

// sell sells the item the shopkeeper offered to buy.
func (g *Game) sell() {
	it := g.selling
	g.selling = nil
	if it == nil || !slices.Contains(g.Items, it) {
		return
	}
	it.ForSale = true
	g.Gold += sellPrice(it)
	g.say(fmt.Sprintf("%sを %dG で売った。", g.Name(it), sellPrice(it)))
}

// steal is the player leaving the shop with what it owes: the items are
// its, and the shopkeepers and their guards come after it.
func (g *Game) steal() {
	for _, it := range g.Inventory {
		it.ForSale = false
	}
	g.say(sayThief)
	for _, m := range g.Shop.Keepers {
		m.Angry, m.Hunter, m.Dozing = true, true, false
	}
	def := g.Shop.Def
	if def.Guard == nil {
		return
	}
	for range between(g.rng, def.Guards[0], def.Guards[1]) {
		if m := g.spawnNear(def.Guard); m != nil {
			m.Hunter = true
		}
	}
	g.say("番犬が放たれた！")
}

// spawnNear puts a monster of def on a free tile a few tiles from the
// player (guards after a thief), and returns it (nil if there is none).
func (g *Game) spawnNear(def *MonsterDef) *Monster {
	for range 200 {
		p := g.Player.Add(Point{g.rng.IntN(13) - 6, g.rng.IntN(13) - 6})
		if !g.Level.At(p).Walkable() || !g.free(p) || chebyshev(p, g.Player) < 3 {
			continue
		}
		m := newMonster(def)
		m.Pos, m.Spawned = p, true
		g.Monsters = append(g.Monsters, m)
		return m
	}
	return nil
}

// enterShop greets the player coming into the shop, and makes it a thief
// going out of it with what it owes.
func (g *Game) enterShop(wasIn bool) {
	in := g.inShop(g.Player)
	switch {
	case in && !wasIn && g.keepersCalm():
		g.say(g.Shop.Def.Name + "だ。")
		g.say(sayWelcome)
	case !in && wasIn && g.debt() > 0:
		g.steal()
	}
}

// keepersCalm says whether the shop has shopkeepers, and none is angry.
func (g *Game) keepersCalm() bool {
	return g.Shop != nil && len(g.Shop.Keepers) > 0 && !g.Shop.Keepers[0].Angry
}

// provoke makes the shopkeepers angry at an attack on one of them.
func (g *Game) provoke(m *Monster) {
	if !m.Keeper || m.Angry || g.Shop == nil {
		return
	}
	g.say("店主を怒らせてしまった！")
	for _, k := range g.Shop.Keepers {
		k.Angry, k.Hunter, k.Dozing = true, true, false
	}
}

// closeShop is the shop without shopkeepers: what it sold is anyone's.
func (g *Game) closeShop() {
	for _, it := range g.Items {
		it.ForSale = false
	}
	for _, it := range g.Inventory {
		it.ForSale = false
	}
}
