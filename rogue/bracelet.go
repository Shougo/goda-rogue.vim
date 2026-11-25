package rogue

import (
	"fmt"
	"slices"
)

// call wakes the monsters of the floor for the player wearing a bracelet
// of 魔物呼び: they come for it (act), and new ones come twice as often
// (linger). Not the allies nor a calm shopkeeper.
func (g *Game) call() {
	if !g.wears(EffectCall) {
		return
	}
	for _, m := range g.Monsters {
		if m.Dozing && !m.Ally && !(m.Keeper && !m.Angry) {
			m.Dozing = false
			g.use(EffectCall)
		}
	}
}

// leap is the player wearing a bracelet of 高飛び sent somewhere on the
// floor, a turn in a hundred its strength; leaving a shop with what is not
// paid for is theft. It says whether it leapt.
func (g *Game) leap() bool {
	b := g.equipped(CategoryBracelet)
	if b == nil || b.Def.Effect != EffectLeap || g.rng.IntN(100) >= b.Def.Strength {
		return false
	}
	g.use(EffectLeap)
	g.say("高く飛び上がった！")
	g.warpAway()
	return true
}

// warpAway sends the player to a tile of a room of the floor; leaving a
// shop with what is not paid for is theft.
func (g *Game) warpAway() {
	wasInShop := g.inShop(g.Player)
	g.warp()
	g.say("どこかへ飛ばされた。")
	if wasInShop && !g.inShop(g.Player) && g.debt() > 0 {
		g.steal()
	}
}

// blast is the player wearing a bracelet of 爆発 blowing up, a turn in a
// hundred its strength (Shougo): its HP halves; the monsters around are
// gone (no EXP), an ally's HP halves, a shopkeeper's too and it is an
// attack on it; the items on the floor around (and under it) are gone;
// the walls around crumble (not the edge of the floor). It says whether
// it blew up.
func (g *Game) blast() bool {
	b := g.equipped(CategoryBracelet)
	if b == nil || b.Def.Effect != EffectBlast || g.rng.IntN(100) >= b.Def.Strength {
		return false
	}
	g.use(EffectBlast)
	g.say("腕輪が爆発した！")
	g.HP = max((g.HP+1)/2, 1)
	g.say("HPが半分になった。")
	g.event("blast")
	for _, m := range slices.Clone(g.Monsters) {
		if chebyshev(m.Pos, g.Player) != 1 {
			continue
		}
		switch {
		case m.Ally, m.Keeper:
			m.HP = max((m.HP+1)/2, 1)
			g.say(fmt.Sprintf("%sのHPが半分になった。", m.Name))
			g.provoke(m)
		default:
			g.say(m.Name + "は消し飛んだ。")
			g.remove(m)
			g.dropStolen(m)
		}
	}
	g.Items = slices.DeleteFunc(g.Items, func(it *Item) bool { return chebyshev(it.Pos, g.Player) <= 1 })
	for _, d := range around {
		p := g.Player.Add(d)
		if g.Level.At(p) == TileRock && p.X > 0 && p.Y > 0 && p.X < g.Level.W-1 && p.Y < g.Level.H-1 {
			g.Level.set(p, TileCorridor)
		}
	}
	return true
}

// blastDamage is the damage n of an explosion on the player, halved with a
// bracelet of 耐爆 (not under 1).
func (g *Game) blastDamage(n int) int {
	if n > 0 && g.wears(EffectBlastGuard) {
		g.use(EffectBlastGuard)
		g.say("耐爆の腕輪が爆風を和らげた。")
		return max(n/2, 1)
	}
	return n
}

// clumsy is what the player threw or shot missing m, with a bracelet of
// ヘタ投げ: it falls at m's feet (a pot breaks there), and a trap under m
// goes off on it, as with a bracelet of the trapper.
func (g *Game) clumsy(it *Item, m *Monster) {
	g.use(EffectClumsy)
	g.say(m.Name + "に当たらなかった。")
	if it.Def.Category == CategoryPot {
		g.smash(it, m.Pos)
	} else {
		g.land(it, m.Pos)
	}
	if t := g.TrapAt(m.Pos); t != nil && slices.Contains(g.Monsters, m) {
		g.monsterSprings(m, t)
	}
}

// standsOn says whether the player can stand on the tile t at p: a
// floor; water with a bracelet of 水蜘蛛 or 浮遊; rock with a bracelet of
// 壁抜け, but the edge of the floor.
func (g *Game) standsOn(t Tile, p Point) bool {
	switch t {
	case TileWater:
		return g.wears(EffectWaterWalk) || g.wears(EffectFloat)
	case TileRock:
		return g.wears(EffectWalls) && p.X > 0 && p.Y > 0 && p.X < g.Level.W-1 && p.Y < g.Level.H-1
	}
	return t.Walkable()
}

// overboard is the player where it cannot stand any more, its bracelet
// taken off (or another put on, or blown there): in water it falls in and
// is washed away, in the rock it is pushed out, to a tile somewhere on the
// floor (Shougo).
func (g *Game) overboard() {
	t := g.Level.At(g.Player)
	if g.standsOn(t, g.Player) {
		return
	}
	if t == TileWater {
		g.say("水路に落ちてしまった！")
	} else {
		g.say("壁の中から押し出された！")
	}
	g.warpAway()
}

// inWalls is a turn of the player in the rock (a bracelet of 壁抜け): 1 HP
// lost, never the last.
func (g *Game) inWalls() {
	if g.Level.At(g.Player) != TileRock || !g.wears(EffectWalls) {
		return
	}
	g.use(EffectWalls)
	if g.HP > 1 {
		g.HP--
	}
}

// cursePower is what a bracelet of 呪い力 adds to the strength (Shougo):
// its strength for each cursed item carried (the bracelet too), not those
// in pots.
func (g *Game) cursePower() int {
	b := g.equipped(CategoryBracelet)
	if b == nil || b.Def.Effect != EffectCursePower {
		return 0
	}
	n := 0
	for _, it := range g.Inventory {
		if it.Cursed {
			n++
		}
	}
	return n * b.Def.Strength
}

// drain is the scroll of 水がれ read: the waterways of the floor dry up
// into floor; the islands are reached on foot then. It says whether there
// was water.
func (g *Game) drain() bool {
	dried := false
	for i, t := range g.Level.Tiles {
		if t == TileWater {
			g.Level.set(Point{i % g.Level.W, i / g.Level.W}, TileRoom)
			dried = true
		}
	}
	return dried
}
