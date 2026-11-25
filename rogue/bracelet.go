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
// attack on it; the walls around crumble (not the edge of the floor). It
// says whether it blew up.
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
	for _, d := range around {
		p := g.Player.Add(d)
		if g.Level.At(p) == TileRock && p.X > 0 && p.Y > 0 && p.X < g.Level.W-1 && p.Y < g.Level.H-1 {
			g.Level.set(p, TileCorridor)
		}
	}
	return true
}
