package rogue

import (
	"fmt"
	"math/rand/v2"
)

// TrapDef is a kind of trap.
type TrapDef struct {
	ID     string
	Name   string
	Effect TrapEffect
	// Strength is how much its effect does.
	Strength int
}

// TrapEffect is what a trap does when the player steps on it.
type TrapEffect string

// The effects of traps.
const (
	TrapPit    TrapEffect = "落とし穴"
	TrapMine   TrapEffect = "地雷"
	TrapSnare  TrapEffect = "トラばさみ"
	TrapPoison TrapEffect = "毒矢"
	TrapSleep  TrapEffect = "睡眠"
	TrapWarp   TrapEffect = "ワープ"
	TrapHunger TrapEffect = "腹減り"
	TrapRust   TrapEffect = "サビ"
)

var trapEffects = []TrapEffect{TrapPit, TrapMine, TrapSnare, TrapPoison, TrapSleep, TrapWarp, TrapHunger, TrapRust}

// Trap is a trap on the floor; Found says whether the player knows it.
type Trap struct {
	Def   *TrapDef
	Pos   Point
	Found bool
}

const (
	glyphTrap = "罠"
	hlTrap    = "GodaRogueTrap"
	miniTrap  = '^'
)

// TrapAt returns the trap on p, or nil.
func (g *Game) TrapAt(p Point) *Trap {
	for _, t := range g.Traps {
		if t.Pos == p {
			return t
		}
	}
	return nil
}

// foundTrapAt returns the trap the player knows on p, or nil.
func (g *Game) foundTrapAt(p Point) *Trap {
	if t := g.TrapAt(p); t != nil && t.Found {
		return t
	}
	return nil
}

// placeTraps hides the traps of a new floor in its rooms: on free tiles,
// not under items. No pit on the last floor.
func (g *Game) placeTraps(rng *rand.Rand, rooms []int) {
	d := g.Dungeon
	c := d.counts(g.Depth)
	for range between(rng, c.MinTrap, c.MaxTrap) {
		p := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		def := d.Trap(pick(rng, d.TrapRows, g.Depth))
		if !g.free(p) || g.TrapAt(p) != nil || def.Effect == TrapPit && g.Depth >= d.Floors {
			continue
		}
		g.Traps = append(g.Traps, &Trap{Def: def, Pos: p})
	}
}

// check is a swing at nothing: it checks the tile in front of the player,
// finding a trap there, or marking the tile safe.
func (g *Game) check(p Point) {
	if !g.Level.At(p).Walkable() {
		return
	}
	t := g.TrapAt(p)
	switch {
	case t == nil:
		g.markSafe(p)
	case !t.Found:
		t.Found = true
		g.say(t.Def.Name + "を見つけた！")
	}
}

// stepOn is the player stepping on the trap t; it says whether the trap
// went off.
func (g *Game) stepOn(t *Trap) bool {
	if t.Found && g.Dungeon.Rules.FoundTrapsSafe {
		g.say(t.Def.Name + "がある。")
		return false
	}
	t.Found = true
	g.sprung = true
	g.say(t.Def.Name + "だ！")
	n := t.Def.Strength
	switch t.Def.Effect {
	case TrapPit:
		g.say("落とし穴に落ちた。")
		if g.trapDamage(n, "落とし穴") {
			return true
		}
		g.wearAndTear()
		g.descend()
	case TrapMine:
		g.HP = max((g.HP+1)/2, 1)
		g.say("爆発に巻き込まれて、HPが半分になった。")
	case TrapSnare:
		g.Stuck = n
		g.say("トラばさみにかかった。")
	case TrapPoison:
		g.say("毒矢が刺さった。")
		if g.trapDamage(n, "毒矢") {
			return true
		}
		if g.Strength > 1 {
			g.Strength--
			g.say("ちからが1下がった。")
		}
	case TrapSleep:
		g.say("眠ってしまった……。")
		for range n {
			g.endTurn(false)
			if g.Over != "" {
				return true
			}
		}
		g.say("目が覚めた。")
	case TrapWarp:
		g.warp()
		g.say("どこかへ飛ばされた。")
	case TrapHunger:
		g.Fullness = max(g.Fullness-n, 0)
		g.say("お腹が減った。")
	case TrapRust:
		rusted := false
		for _, c := range []Category{CategoryWeapon, CategoryShield} {
			if it := g.equipped(c); it != nil {
				it.Upgrade--
				rusted = true
				g.say(g.Name(it) + "がさびてしまった。")
			}
		}
		if !rusted {
			g.say("何も起こらなかった。")
		}
	}
	return true
}

// trapDamage hurts the player by n, by the trap why; it says whether the
// player fell.
func (g *Game) trapDamage(n int, why string) bool {
	g.HP -= n
	g.say(fmt.Sprintf("%dのダメージ。", n))
	if g.HP <= 0 {
		g.HP = 0
		g.end(why+"で倒れた", false)
		return true
	}
	return false
}

// warp sends the player to a free tile of a room of the floor.
func (g *Game) warp() {
	var rooms []int
	for i, r := range g.Level.Rooms {
		if !r.Dummy {
			rooms = append(rooms, i)
		}
	}
	for range 100 {
		p := g.randomTile(g.rng, rooms[g.rng.IntN(len(rooms))])
		if !g.free(p) || g.TrapAt(p) != nil {
			continue
		}
		from := g.Level.RoomAt(g.Player)
		g.Player = p
		if room := g.Level.RoomAt(p); room != from {
			g.wake(from)
			g.wake(room)
		}
		g.look()
		g.markSafe(p)
		return
	}
}
