package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
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
	TrapPit  TrapEffect = "落とし穴"
	TrapMine TrapEffect = "地雷"
	// TrapBigMine: as a mine, down to a quarter of the HP.
	TrapBigMine TrapEffect = "大型地雷"
	TrapSnare   TrapEffect = "トラばさみ"
	TrapPoison  TrapEffect = "毒矢"
	TrapSleep   TrapEffect = "睡眠"
	TrapWarp    TrapEffect = "ワープ"
	TrapHunger  TrapEffect = "腹減り"
	TrapRust    TrapEffect = "サビ"
	TrapRot     TrapEffect = "腐り"
	TrapCurse   TrapEffect = "呪い"
	// TrapOnigiri: an item carried turns into an onigiri, a monster too;
	// the trap breaks then with a chance of its strength (Shougo,
	// 2026-11-27: no endless food).
	TrapOnigiri TrapEffect = "おにぎり"
)

var trapEffects = []TrapEffect{TrapPit, TrapMine, TrapBigMine, TrapSnare, TrapPoison, TrapSleep, TrapWarp, TrapHunger, TrapRust, TrapRot, TrapCurse, TrapOnigiri}

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

// placeTraps hides the traps of a new floor in its rooms (trapSpot), not
// in a shop.
func (g *Game) placeTraps(rng *rand.Rand, rooms []int) {
	d := g.Dungeon
	c := d.counts(g.Depth)
	for range between(rng, c.MinTrap, c.MaxTrap) {
		p := g.spot(rng, rooms)
		def := d.Trap(pick(rng, d.TrapRows, g.Depth))
		if !g.trapSpot(p, def) || g.inShop(p) {
			continue
		}
		g.Traps = append(g.Traps, &Trap{Def: def, Pos: p})
	}
}

// trapSpot says whether a trap of the kind def can be hidden on p: a tile
// with no trap, item, player or stairs (a monster may stand on it), not
// the entrance of a room (a tile of it next to a corridor: as in Shiren,
// the way in and out is never trapped), and no pit on the last floor.
func (g *Game) trapSpot(p Point, def *TrapDef) bool {
	if p == g.Player || g.blocked(p, needNoStairs|needNoTrap|needNoItem) || def.Effect == TrapPit && g.Depth >= g.Dungeon.Floors {
		return false
	}
	// Under a monster is fine: monsters do not set traps off.
	return !g.entrance(p)
}

// entrance says whether p is a tile of a room next to a corridor.
func (g *Game) entrance(p Point) bool {
	if g.Level.At(p) != TileRoom {
		return false
	}
	for _, d := range Directions {
		if g.Level.At(p.Add(d)) == TileCorridor {
			return true
		}
	}
	return false
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

// trapTexts say what each kind of trap does, for look.
var trapTexts = map[TrapEffect]string{
	TrapPit:     "落ちて次の階へ。少しダメージを受ける。",
	TrapMine:    "爆発して HP が半分になる。",
	TrapBigMine: "大爆発して HP が 1/4 になる。",
	TrapSnare:   "しばらく動けなくなる。",
	TrapPoison:  "毒矢でダメージを受け、ちからが下がる。",
	TrapSleep:   "しばらく眠ってしまう。",
	TrapWarp:    "フロアのどこかへ飛ばされる。",
	TrapHunger:  "お腹が減る。",
	TrapRust:    "装備している武器と盾がさびる。",
	TrapRot:     "持っているおにぎりがすべて腐る（壺の中は無事）。",
	TrapCurse:   "装備以外の持ち物がどれか呪われる。",
	TrapOnigiri: "持ち物が 1 つおにぎりになる（装備中の物、壺、食料はならない。敵が踏むと、その敵がおにぎりになる）。踏むと壊れることがある。",
}

// commandStep steps on the trap under the player on purpose.
const commandStep = "踏む"

// stepOn is the player stepping on the trap t; it says whether the trap
// went off.
func (g *Game) stepOn(t *Trap) bool { return g.spring(t, false) }

// spring sets off the trap t under the player; a found trap does not go
// off by itself with the rule 見つけた罠 安全, but stepping on it on
// purpose (force) does. It says whether the trap went off.
func (g *Game) spring(t *Trap, force bool) bool {
	if !force && g.wears(EffectFloat) {
		// Floating over it: nothing.
		g.use(EffectFloat)
		return false
	}
	if !force && g.wears(EffectTrapper) {
		// A bracelet of the trapper: the player never sets a trap off
		// by walking.
		t.Found = true
		g.use(EffectTrapper)
		g.say(t.Def.Name + "がある。")
		return false
	}
	if !force && g.sealChance(SealNoTrap) {
		// A ward of the shield: the trap is found, and does not go off.
		t.Found = true
		g.say(t.Def.Name + "を踏んだが、盾が守った。")
		return false
	}
	if t.Found && g.Dungeon.Rules.FoundTrapsSafe && !force {
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
	case TrapMine, TrapBigMine:
		g.say("爆発に巻き込まれた！")
		lost := g.blastDamage(g.HP - mineLeft(t.Def.Effect, g.HP))
		g.HP -= lost
		g.say(fmt.Sprintf("%dのダメージを受けた。", lost))
		g.grillCarried()
	case TrapSnare:
		g.Stuck, g.heldBy = n, nil
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
		if g.prevents(MagicSleep) {
			break
		}
		g.say("眠ってしまった……。")
		for range n {
			g.endTurn(false)
			if g.Over != "" {
				return true
			}
		}
		g.say("目が覚めた。")
	case TrapWarp:
		g.warpAway()
	case TrapHunger:
		if g.wears(EffectNoStarve) {
			g.use(EffectNoStarve)
			g.say("ハラヘリよけの腕輪が守った。")
			break
		}
		if g.sealChance(SealNoStarve) {
			g.say("盾が守った。")
			break
		}
		g.Fullness = max(g.Fullness-n, 0)
		g.say("お腹が減った。")
	case TrapRust:
		if !g.rust([]Category{CategoryWeapon, CategoryShield}, 1) {
			g.say("何も起こらなかった。")
		}
	case TrapRot:
		g.rotTrap()
	case TrapCurse:
		g.curseTrap(n)
	case TrapOnigiri:
		g.onigiriTrap()
		g.mayBreak(t)
	}
	return true
}

// onigiriTrap is the trap of おにぎり on the player: an item carried, at
// random, turns into an onigiri; not one equipped, a pot (Shougo,
// 2026-11-27: too unfair), food or goods for sale.
func (g *Game) onigiriTrap() {
	var can []*Item
	for _, it := range g.Inventory {
		switch it.Def.Category {
		case CategoryPot, CategoryFood:
			continue
		}
		if !it.Equipped && !it.ForSale {
			can = append(can, it)
		}
	}
	if len(can) == 0 || g.Dungeon.plainOnigiri() == nil {
		g.say("何も起こらなかった。")
		return
	}
	g.toOnigiri(can[g.rng.IntN(len(can))])
}

// mayBreak breaks the trap t, with the chance of its strength.
func (g *Game) mayBreak(t *Trap) {
	if g.rng.IntN(100) >= t.Def.Strength {
		return
	}
	g.Traps = slices.DeleteFunc(g.Traps, func(x *Trap) bool { return x == t })
	if g.sees(g.Player, t.Pos) {
		g.say(t.Def.Name + "は壊れた。")
	}
}

// trapperSees shows every trap of the floor to the player wearing a
// bracelet of the trapper (罠師).
func (g *Game) trapperSees() {
	if !g.wears(EffectTrapper) {
		return
	}
	for _, t := range g.Traps {
		if !t.Found {
			t.Found = true
			g.use(EffectTrapper)
		}
	}
}

// monsterSprings is the monster m stepping on the trap t while the player
// wears a bracelet of the trapper (罠師): monsters set traps off then
// (never an ally, nor a calm shopkeeper). What it does to m: a pit takes
// it off the floor, a mine halves its HP, the poisoned arrow hurts it, a
// snare holds it and the trap of sleep puts it to sleep (both: turns of
// sleep), the warp sends it away; the others do nothing to a monster. A
// monster it kills is defeated by the player.
func (g *Game) monsterSprings(m *Monster, t *Trap) {
	if m.Ally || m.Keeper && !m.Angry {
		return
	}
	shown := g.sees(g.Player, m.Pos)
	say := func(s string) {
		if shown {
			g.say(s)
		}
	}
	t.Found = true
	say(m.Name + "は" + t.Def.Name + "を踏んだ！")
	n := t.Def.Strength
	switch t.Def.Effect {
	case TrapPit:
		say(m.Name + "は落とし穴に落ちていった。")
		g.remove(m)
	case TrapMine, TrapBigMine:
		if m.Unhurt == 0 {
			m.HP = mineLeft(t.Def.Effect, m.HP)
		}
		say(m.Name + "は爆発に巻き込まれた。")
	case TrapPoison:
		n = m.harm(n)
		m.HP -= n
		// (A blow frees it of かなしばり.)
		m.Bound = 0
		say(fmt.Sprintf("毒矢が刺さった。%sに%dのダメージ。", m.Name, n))
		if m.HP <= 0 {
			g.kill(m, nil, true, shown)
		}
	case TrapSnare, TrapSleep:
		m.Asleep = max(m.Asleep, n)
		say(m.Name + "は動けなくなった。")
	case TrapWarp:
		if p, ok := g.freeTile(); ok {
			m.Pos = p
			say(m.Name + "はどこかへ飛ばされた。")
		}
	case TrapOnigiri:
		// (No EXP: it is not defeated.)
		if o := g.Dungeon.plainOnigiri(); o != nil {
			say(m.Name + "はおにぎりになった！")
			p := m.Pos
			g.remove(m)
			g.land(&Item{Def: o, Glyph: o.Glyph}, p)
		}
		g.mayBreak(t)
	default:
		say("しかし何も起こらなかった。")
	}
}

// mineLeft is what is left of hp after a mine (e): a half, a quarter for
// a big one, at least 1.
func mineLeft(e TrapEffect, hp int) int {
	if e == TrapBigMine {
		return max((hp+3)/4, 1)
	}
	return max((hp+1)/2, 1)
}

// trapDamage hurts the player by n, by the trap why; it says whether the
// player fell.
func (g *Game) trapDamage(n int, why string) bool {
	n = g.weakened(n)
	g.HP -= n
	g.say(fmt.Sprintf("%dのダメージ。", n))
	g.event("hurt %d %s", n, why)
	if g.HP <= 0 {
		g.HP = 0
		g.end(why+"で倒れた", false)
		// (Brought back by 復活の草, it did not.)
		return g.Over != ""
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
	p, ok := g.find(func() Point { return g.randomTile(g.rng, rooms[g.rng.IntN(len(rooms))]) }, needFree|needNoTrap)
	if !ok {
		return
	}
	from := g.Level.RoomAt(g.Player)
	g.Player = p
	if room := g.Level.RoomAt(p); room != from {
		g.wake(from)
		g.wake(room)
	}
	g.look()
	g.markSafe(p)
}
