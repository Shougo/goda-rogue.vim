package rogue

import "math/rand/v2"

// newItem makes an item of the floor on p, from its item table.
func (g *Game) newItem(rng *rand.Rand, pos Point) *Item {
	def := g.Dungeon.Item(pick(rng, g.Dungeon.ItemRows, g.Depth))
	it := &Item{Def: def, Pos: pos, Glyph: def.Glyph}
	switch {
	case def.Category == CategoryMoney:
		it.Amount = between(rng, 10, 30+g.Depth*10)
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
	return it
}

// fillHouse makes a room of the floor a monster house: many monsters,
// asleep, and items. It is not the room where the player starts, unless
// it is the only one (a big room).
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
	for range min(max(area/5, 5), 24) {
		p := g.randomTile(rng, g.House)
		if !g.free(p) || chebyshev(p, g.Player) <= 2 {
			continue
		}
		m := newMonster(d.Monster(pick(rng, d.MonsterRows, g.Depth)))
		m.Pos, m.Dozing, m.House = p, true, true
		g.Monsters = append(g.Monsters, m)
	}
	for range min(max(area/10, 3), 12) {
		p := g.randomTile(rng, g.House)
		if g.free(p) {
			g.Items = append(g.Items, g.newItem(rng, p))
		}
	}
}

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
		for range 100 {
			p := Point{rng.IntN(g.Level.W), rng.IntN(g.Level.H)}
			if g.Level.At(p).Walkable() && g.free(p) {
				return p
			}
		}
	}
	return g.randomTile(rng, rooms[rng.IntN(len(rooms))])
}
