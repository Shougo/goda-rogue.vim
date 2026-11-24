package rogue

import (
	"math/rand/v2"
	"slices"
)

// A mimic (AbilityMimic) hides on a new floor: as an item (an Item with
// its Mimic), or as stairs (a Decoy). How deep its disguise is decides
// what shows it (the first number of the ability):
//
//	1  trying to pick the item up; a sensing bracelet in reach, or eye
//	   drops
//	2  the item is picked up and carried: using it in any way, or the
//	   next floor; identifying it tells what it is (it is then listed
//	   as the monster, and still shows itself when used)
//
// Shown, it appears next to the player and strikes first. Defeated, it
// may drop the item it looked like.
const (
	mimicShallow = 1
	mimicDeep    = 2
)

// Decoy is a mimic that looks like the stairs.
type Decoy struct {
	Pos Point
	Def *MonsterDef
}

// mimicOf returns the mimicry of the kind def, if it has it.
func mimicOf(def *MonsterDef) (Ability, bool) {
	for _, a := range def.Abilities {
		if a.Name == AbilityMimic {
			return a, true
		}
	}
	return Ability{}, false
}

// shallow says whether it is a mimic that shows itself when picked up.
func (it *Item) shallow() bool {
	if it.Mimic == nil {
		return false
	}
	a, _ := mimicOf(it.Mimic)
	return a.Values[0] == mimicShallow
}

// decoyAt returns the index of the decoy at p, or -1.
func (g *Game) decoyAt(p Point) int {
	return slices.IndexFunc(g.Decoys, func(d Decoy) bool { return d.Pos == p })
}

// stairsAt says whether p looks like the stairs: them, or a decoy.
func (g *Game) stairsAt(p Point) bool {
	return p == g.Stairs || g.decoyAt(p) >= 0
}

// hideMimics hides the mimics among the monsters of a new floor.
func (g *Game) hideMimics(rng *rand.Rand, rooms []int) {
	for _, m := range slices.Clone(g.Monsters) {
		a, ok := mimicOf(m.Def)
		if !ok || m.Ally {
			continue
		}
		g.Monsters = slices.DeleteFunc(g.Monsters, func(o *Monster) bool { return o == m })
		if !g.hide(rng, rooms, m.Def, m.Pos, a) {
			g.Monsters = append(g.Monsters, m)
		}
	}
}

// hide hides a mimic of the kind def, by its mimicry a: as stairs or as
// goods of the shop by their chances, else as an item at pos (or on
// another tile, if pos is taken). It says whether it did.
func (g *Game) hide(rng *rand.Rand, rooms []int, def *MonsterDef, pos Point, a Ability) bool {
	roll := rng.IntN(100)
	spot := func() Point { return g.spot(rng, rooms) }
	switch {
	case roll < a.Values[1]:
		// Stairs, not in the room the player arrives in.
		p, ok := g.find(spot, needHidden, func(p Point) bool {
			return len(rooms) == 1 || g.Level.RoomAt(p) != g.Level.RoomAt(g.Player)
		})
		if ok {
			g.Decoys = append(g.Decoys, Decoy{Pos: p, Def: def})
		}
		return ok
	case roll < a.Values[1]+a.Values[2] && g.Shop != nil:
		// One of the goods of the shop.
		var goods []*Item
		for _, it := range g.Items {
			if it.ForSale && it.Mimic == nil {
				goods = append(goods, it)
			}
		}
		if len(goods) > 0 {
			goods[rng.IntN(len(goods))].Mimic = def
			return true
		}
	}
	if !g.fits(pos, needHidden) {
		var ok bool
		if pos, ok = g.find(spot, needHidden); !ok {
			return false
		}
	}
	return g.hideAsItem(rng, def, pos, a)
}

// hideAsItem hides a mimic of the kind def as an item of the floor's
// table at pos. A carried one is no money nor arrows, which do not stay
// items of their own when picked up.
func (g *Game) hideAsItem(rng *rand.Rand, def *MonsterDef, pos Point, a Ability) bool {
	for range 20 {
		it := g.newItem(rng, pos)
		if c := it.Def.Category; a.Values[0] == mimicDeep && (c == CategoryMoney || c == CategoryArrow) {
			continue
		}
		it.Mimic = def
		g.Items = append(g.Items, it)
		return true
	}
	return false
}

// spawnHidden hides a mimic of the kind def that comes after the floor
// began as an item out of the player's sight; it gives no rewards, as
// the monsters spawned.
func (g *Game) spawnHidden(def *MonsterDef, a Ability) {
	if p, ok := g.find(g.wanderTarget, needHidden|needUnseen); ok && g.hideAsItem(g.rng, def, p, a) {
		g.Items[len(g.Items)-1].spawned = true
	}
}

// appear puts a monster of the kind def on at, or on the free tile
// nearest to the player if at is taken, awake; nil if there is no room.
func (g *Game) appear(def *MonsterDef, at Point) *Monster {
	if !g.Level.At(at).Walkable() || g.occupied(at) {
		at = g.Player
		found := false
	search:
		for r := 1; r <= 5; r++ {
			// (The tiles it can strike from first.)
			for _, clear := range []bool{true, false} {
				for dy := -r; dy <= r; dy++ {
					for dx := -r; dx <= r; dx++ {
						d := Point{dx, dy}
						p := g.Player.Add(d)
						if chebyshev(p, g.Player) != r || !g.Level.At(p).Walkable() || g.occupied(p) ||
							clear && r == 1 && !g.clearFrom(g.Player, d) {
							continue
						}
						at, found = p, true
						break search
					}
				}
			}
		}
		if !found {
			return nil
		}
	}
	m := newMonster(def)
	m.Pos = at
	g.Monsters = append(g.Monsters, m)
	return m
}

// strikeFirst is the first strike of the mimic m as it shows itself, if
// it is next to the player. With turn, the turn passes after it: it
// waits its action of that turn out.
func (g *Game) strikeFirst(m *Monster, turn bool) {
	d := g.Player.Sub(m.Pos)
	if chebyshev(m.Pos, g.Player) == 1 && g.clearFrom(m.Pos, d) {
		g.hitPlayer(m)
		m.waits = turn
	}
}

// unmask is the mimic that looks like the item it (carried or on the
// floor) showing itself: the item is gone, the monster appears where it
// lay or next to the player, and strikes first. With turn, the turn
// passes after it.
func (g *Game) unmask(it *Item, turn bool) {
	name := g.Name(it)
	def := it.Mimic
	at := g.Player
	if slices.Contains(g.Items, it) {
		at = it.Pos
	}
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	m := g.appear(def, at)
	if m == nil {
		g.say(name + "は消えてしまった。")
		return
	}
	if !it.Unmasked {
		g.say(name + "は" + def.Name + "だった！")
	} else {
		g.say(def.Name + "が正体を現した！")
	}
	g.event("mimic %s", def.ID)
	// What it looked like is what it may drop.
	it.Mimic, it.Unmasked, it.Equipped, it.used, it.Glyph = nil, false, false, false, it.Def.Glyph
	m.loot, m.Spawned = it, it.spawned
	g.strikeFirst(m, turn)
}

// unmaskDecoy is the mimic that looks like the stairs under the player
// showing itself.
func (g *Game) unmaskDecoy(i int, turn bool) {
	def := g.Decoys[i].Def
	g.Decoys = slices.Delete(g.Decoys, i, i+1)
	m := g.appear(def, g.Player)
	if m == nil {
		g.say("階段は消えてしまった。")
		return
	}
	g.say("階段は" + def.Name + "だった！")
	g.event("mimic %s", def.ID)
	g.strikeFirst(m, turn)
}

// label marks the carried mimic it as known for what it is (identified):
// it is listed as the monster.
func (g *Game) label(it *Item) {
	name := g.Name(it)
	it.Unmasked, it.Glyph = true, it.Mimic.Glyph
	g.say(name + "は" + it.Mimic.Name + "が化けたものだった！")
}

// showSensed shows the shallow mimics a sensing bracelet reaches: they
// stand where they lay, without a strike.
func (g *Game) showSensed() {
	for _, it := range slices.Clone(g.Items) {
		if it.shallow() && g.sensesMonster(it.Pos) {
			g.use(EffectSenseEnemy)
			g.showUp(it)
		}
	}
}

// showUp is the mimic that looks like the item it on the floor found
// out: it stands where it lay (next to the player, if the player stands
// there), without a strike.
func (g *Game) showUp(it *Item) {
	name, def := g.Name(it), it.Mimic
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	m := g.appear(def, it.Pos)
	if m == nil {
		return
	}
	if g.sees(g.Player, m.Pos) {
		g.say(name + "は" + def.Name + "だった！")
	}
	it.Mimic, it.Unmasked, it.Glyph = nil, false, it.Def.Glyph
	m.loot, m.Spawned = it, it.spawned
}

// eyeDrops shows the shallow mimics and the traps of the floor.
func (g *Game) eyeDrops() {
	g.say("目がよく見えるようになった。")
	g.Clear = true
	for _, t := range g.Traps {
		t.Found = true
	}
	for _, it := range slices.Clone(g.Items) {
		if it.shallow() {
			g.showUp(it)
		}
	}
}

// arriveWithMimics is the mimics the player carried to a new floor
// showing themselves.
func (g *Game) arriveWithMimics() {
	for _, it := range slices.Clone(g.Inventory) {
		if it.Mimic != nil && g.Over == "" {
			g.unmask(it, false)
		}
	}
}
