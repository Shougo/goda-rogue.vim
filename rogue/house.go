package rogue

import "math/rand/v2"

// newItem makes an item of the floor on p, from its item table.
func (g *Game) newItem(rng *rand.Rand, pos Point) *Item {
	d := g.Dungeon
	return g.newItemOf(rng, d.ItemTable(d.counts(g.Depth).ItemTable).pick(rng, d, g.Depth), pos)
}

// goldPiles are the least and the most gold of a pile on the floor, on
// some floors (Shougo, 2026-11-24): a pile is worth some of the goods of a
// shop, as in Shiren. Between them, they go in a straight line.
var goldPiles = []struct{ depth, lo, hi int }{
	{1, 100, 400},
	{30, 300, 1000},
	{50, 400, 1200},
	{99, 500, 1500},
}

// goldPile returns the least and the most gold of a pile on the floor
// depth.
func goldPile(depth int) (int, int) {
	first, last := goldPiles[0], goldPiles[len(goldPiles)-1]
	if depth <= first.depth {
		return first.lo, first.hi
	}
	for i := 1; i < len(goldPiles); i++ {
		a, b := goldPiles[i-1], goldPiles[i]
		if depth <= b.depth {
			f := func(x, y int) int { return x + (y-x)*(depth-a.depth)/(b.depth-a.depth) }
			return f(a.lo, b.lo), f(a.hi, b.hi)
		}
	}
	return last.lo, last.hi
}

// newItemOf makes an item of the kind def on p.
func (g *Game) newItemOf(rng *rand.Rand, def *ItemDef, pos Point) *Item {
	it := &Item{Def: def, Pos: pos, Glyph: def.Glyph}
	switch {
	case def.Category == CategoryMoney:
		lo, hi := goldPile(g.Depth)
		it.Amount = between(rng, lo, hi)
	case it.upgradable():
		it.Upgrade = rollUpgrade(rng)
		if def.Seal != "" {
			it.Seals = []string{def.Seal}
		}
	case def.Category == CategoryPot:
		it.Charges = between(rng, def.Charges[0], def.Charges[1])
	case def.Category == CategoryStaff:
		it.Charges = between(rng, def.Charges[0], def.Charges[1])
	case def.Category == CategoryArrow:
		it.Count = between(rng, def.Charges[0], def.Charges[1])
	}
	g.rollCurse(rng, it)
	g.rollBless(rng, it)
	return it
}

// fillHouse makes a room of the floor a monster house: many monsters,
// asleep, items, and more traps (as in Shiren: a danger, and a help to a
// player who knows it). It is not the room where the player starts,
// unless it is the only one (a big room). The bigger the room, the more
// of each, up to a most: a big room holds some 20 monsters, 35 items and
// 20 traps with those of the floor.
func (g *Game) fillHouse(rng *rand.Rand, rooms []int, start int) {
	var candidates []int
	for _, i := range rooms {
		if i != start {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		candidates = rooms
	}
	g.House = candidates[rng.IntN(len(candidates))]
	r := g.Level.Rooms[g.House]
	area := r.W * r.H
	d := g.Dungeon
	for range min(max(area/5, 5), houseMonsters) {
		p := g.randomTile(rng, g.House)
		if !g.free(p) || chebyshev(p, g.Player) <= 2 {
			continue
		}
		m := newMonster(d.Monster(pick(rng, d.MonsterRows, g.Depth)))
		m.Pos, m.Dozing, m.House = p, true, true
		g.Monsters = append(g.Monsters, m)
	}
	for range min(max(area/8, 4), houseItems) {
		p := g.randomTile(rng, g.House)
		if g.free(p) {
			g.Items = append(g.Items, g.newItem(rng, p))
		}
	}
	c := d.counts(g.Depth)
	// (The traps of floors.txt are those of a room; a big room has
	// several times as many.)
	n := between(rng, c.MinHouseTrap, c.MaxHouseTrap) * min(max(area/houseTrapArea, 1), houseTrapTimes)
	// A few tries a trap: its traps are the point of a monster house.
	for tries := 4 * n; n > 0 && tries > 0; tries-- {
		p := g.randomTile(rng, g.House)
		def := d.Trap(pick(rng, d.TrapRows, g.Depth))
		if !g.trapSpot(p, def) {
			continue
		}
		g.Traps = append(g.Traps, &Trap{Def: def, Pos: p})
		n--
	}
}

// The most a monster house adds to its room: monsters and items; and its
// traps, those of a room for every houseTrapArea tiles, houseTrapTimes
// times at most.
const (
	houseMonsters  = 14
	houseItems     = 30
	houseTrapArea  = 100
	houseTrapTimes = 3
)

// enterHouse tells the player it is in the monster house, the first time.
func (g *Game) enterHouse() {
	if g.House >= 0 && !g.houseFound && g.Level.RoomAt(g.Player) == g.House {
		g.houseFound = true
		g.say("モンスターハウスだ！")
	}
}

// houseWakes is the chance (1 in) that a monster asleep in a monster house
// wakes up on its turn, with the player in the house; one next to the
// player (2 tiles) always does.
const houseWakes = 16

// stir wakes, perhaps, a monster asleep in a monster house where the player
// is (a big room, where entering it does not wake them all).
func (g *Game) stir(m *Monster) {
	if !m.House || g.Level.RoomAt(g.Player) != g.House {
		return
	}
	if chebyshev(m.Pos, g.Player) <= 2 || g.rng.IntN(houseWakes) == 0 {
		m.Dozing = false
	}
}

// spot returns a free tile for a monster, an item or a trap of a new
// floor: in one of the rooms, or anywhere in a maze.
func (g *Game) spot(rng *rand.Rand, rooms []int) Point {
	if g.Level.Maze {
		if p, ok := g.find(func() Point { return Point{rng.IntN(g.Level.W), rng.IntN(g.Level.H)} }, needFree); ok {
			return p
		}
	}
	return g.randomTile(rng, rooms[rng.IntN(len(rooms))])
}
