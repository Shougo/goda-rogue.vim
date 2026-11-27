package rogue

import (
	"fmt"
	"math"
	"slices"
)

// useItem uses it: drinks a grass, reads a scroll, waves a staff, eats
// food. All but a staff are used up.
func (g *Game) useItem(it *Item) {
	if g.curseStops(it) {
		return
	}
	name := g.Name(it)
	// (The name before the blessing shows.)
	blessed := it.Blessed
	if it.Def.Category != CategoryStaff && !g.kept(it) {
		g.discard(it)
	}
	g.event("use %s", it.Def.ID)
	switch it.Def.Category {
	case CategoryGrass:
		g.say(name + "を飲んだ。")
	case CategoryScroll:
		g.say(name + "を読んだ。")
	case CategoryStaff:
		g.say(name + "を振った。")
		if it.Charges == 0 {
			// It does not tell what it is, but it is empty.
			it.Known = true
			g.say("何も起こらなかった。")
			return
		}
		it.CurseTold = true
		if !blessed {
			it.Charges--
			it.Waved++
			break
		}
		g.say("祝福の力で回数は減らなかった。")
		if g.rng.IntN(100) < blessedStaff {
			it.Blessed = false
			g.say(name + "の祝福が解けた。")
		}
	case CategoryFood:
		g.say(name + "を食べた。")
	}

	if blessed && it.Def.Category != CategoryStaff {
		g.say("祝福の力で" + name + "はなくならなかった。")
	}

	// Using it tells what it is; a scroll that chooses, once its effect
	// is had (asks).
	if !slices.Contains(choosingEffects, it.Def.Effect) {
		g.identify(it)
	}

	g.effectOf(it, g.lucky(it, blessed))
}

// choosingEffects are those of the scrolls that ask for an item: the
// scroll is identified once its effect is had, not when it is read
// (Shougo, 2026-11-27); with nothing to choose, or the choice cancelled,
// it stays unknown.
var choosingEffects = []Effect{EffectIdentify, EffectUncurse, EffectMoreSlots, EffectEraseSeal,
	EffectOnigiri, EffectPotUp, EffectSuck, EffectBless, EffectPlate}

// asks is the scroll it that asks for an item: whether it opened its
// prompt (ok), which identifies it once chosen. Unknown, the prompt does
// not tell what it is for.
func (g *Game) asks(it *Item, ok bool) bool {
	if !ok {
		return false
	}
	g.Prompt.scroll = it
	if !g.identified[it.Def.ID] {
		g.Prompt.Title = "どれに使う？"
	}
	return true
}

// used is the effect of the scroll of the prompt p had: it is identified.
func (g *Game) used(p *Prompt) {
	if p.scroll != nil {
		g.identify(p.scroll)
	}
}

// effectOf is the effect of it, used by the player (or a grass thrown at
// it, which then has its effect on the player), lucky or not.
func (g *Game) effectOf(it *Item, lucky bool) {
	n := it.Def.Strength
	switch it.Def.Effect {
	case EffectHeal:
		if g.HP >= g.MaxHP {
			up := max(n/50, 1)
			g.MaxHP += up
			g.HP = g.MaxHP
			g.say(fmt.Sprintf("最大HPが%d上がった。", up))
			return
		}
		healed := min(n, g.MaxHP-g.HP)
		g.HP += healed
		g.say(fmt.Sprintf("HPが%d回復した。", healed))
	case EffectStrength:
		// Under its max, the strength comes back (not above it); at its
		// max, both go up (as Shiren).
		if g.Strength < g.MaxStrength {
			up := min(n, g.MaxStrength-g.Strength)
			g.Strength += up
			g.say(fmt.Sprintf("ちからが%d回復した。", up))
			return
		}
		g.MaxStrength += n
		g.Strength += n
		g.say(fmt.Sprintf("ちからが%d上がった。", n))
	case EffectLight:
		g.light()
		g.say("フロアが明るくなった。")
	case EffectDrain:
		if g.drain() {
			g.say("フロアの水路の水がかれた。")
		} else {
			g.say("しかし何も起こらなかった。")
		}
	case EffectIdentify:
		switch {
		case lucky && g.identifyAll():
			g.identify(it)
		case !g.asks(it, g.askIdentify()):
			g.say("識別するものはなかった。")
		}
	case EffectBlowAway:
		g.blowAway()
	case EffectRecruit:
		g.recruit()
	case EffectEyeDrops:
		g.eyeDrops()
	case EffectFood:
		g.fill(n)
		g.say("お腹がふくれた。")
	case EffectRotten:
		g.eatRotten(n)
	case EffectMaxHP:
		g.MaxHP += n
		g.HP += n
		g.say(fmt.Sprintf("最大HPが%d上がった。", n))
	case EffectUpWeapon:
		g.upgrade(CategoryWeapon, n+luckyUpgrade*boolInt(lucky), lucky)
	case EffectUpShield:
		g.upgrade(CategoryShield, n+luckyUpgrade*boolInt(lucky), lucky)
	case EffectSwap:
		g.swapPlaces()
	case EffectMoreSlots:
		if !g.asks(it, g.ask(promptSlot)) {
			g.say("何も起こらなかった。")
		}
	case EffectEraseSeal:
		if !g.asks(it, g.ask(promptEraseItem)) {
			g.say("何も起こらなかった。")
		}
	case EffectUncurse:
		if g.exorcise() {
			g.identify(it)
		}
		switch {
		case lucky:
			g.say(sayLucky)
			g.uncurse()
			g.identify(it)
		case !g.asks(it, g.ask(promptUncurse)):
			g.say("何も起こらなかった。")
		}
	case EffectSlash:
		g.slash(n)
	case EffectWipe:
		g.wipe()
	case EffectNoTraps:
		g.clearTraps()
	case EffectSeer:
		g.seer()
	case EffectGather:
		g.gather()
	case EffectLost:
		g.lost()
	case EffectGatherFoes:
		g.gatherFoes()
	case EffectTrouble:
		g.trouble()
	case EffectOnigiri, EffectPotUp, EffectSuck, EffectBless, EffectPlate:
		kinds := map[Effect]string{EffectOnigiri: promptOnigiri, EffectPotUp: promptPotUp, EffectSuck: promptSuck, EffectBless: promptBless, EffectPlate: promptPlate}
		if !g.asks(it, g.ask(kinds[it.Def.Effect])) {
			g.say("何も起こらなかった。")
		}
	default:
		if e := MagicEffect(it.Def.Effect); castable(it.Def.Category, e) {
			g.cast(it.Def.Category, e)
			return
		}
		g.say("何も起こらなかった。")
	}
}

// castable says whether an item of the category c can have the effect e
// of a bolt (monsters.txt 魔法): a grass has it on who drinks it or on
// the monster it is thrown at, a scroll on the enemies of the room, a
// staff on the monster its bolt hits. Only a staff blows away.
func castable(c Category, e MagicEffect) bool {
	switch c {
	case CategoryGrass, CategoryScroll:
		return e != MagicBlow && slices.Contains(magicEffects, e)
	case CategoryStaff:
		return slices.Contains(magicEffects, e)
	}
	return false
}

// cast is the effect e of a bolt, of an item of the category c the
// player uses.
func (g *Game) cast(c Category, e MagicEffect) {
	d := Directions[g.Facing]
	switch c {
	case CategoryGrass:
		g.magicOnPlayer(e, d, nil)
	case CategoryStaff:
		if m := g.bolt(); m != nil {
			if g.reflects(m, reflectStaffs) {
				g.magicOnPlayer(e, back(d), m)
				return
			}
			g.provoke(m)
			g.magicOn(m, e, d)
		}
	case CategoryScroll:
		ms := g.enemiesAround()
		if len(ms) == 0 {
			g.say("何も起こらなかった。")
		}
		for _, m := range ms {
			g.magicOn(m, e, d)
		}
	}
}

// enemiesAround returns the enemies a scroll reaches: those of the
// player's room, or those next to it in a corridor.
func (g *Game) enemiesAround() []*Monster {
	room := g.Level.RoomAt(g.Player)
	var ms []*Monster
	for _, m := range g.Monsters {
		if !m.hostile() {
			continue
		}
		if room >= 0 && g.Level.RoomAt(m.Pos) == room || chebyshev(m.Pos, g.Player) <= 1 {
			ms = append(ms, m)
		}
	}
	return ms
}

// upgrade raises the upgrade value of the equipped weapon or shield by
// n; lucky says that n is more than the scroll gives (lucky.go).
func (g *Game) upgrade(c Category, n int, lucky bool) {
	it := g.equipped(c)
	if it == nil {
		g.say("何も起こらなかった。")
		return
	}
	if lucky {
		g.say(sayLucky)
	}
	it.Upgrade += n
	it.Known = true
	// The blessing takes the curse away too.
	cursed := it.Cursed
	it.Cursed = false
	g.say(fmt.Sprintf("%sが強くなった。", g.describe(it)))
	if cursed {
		g.say(g.Name(it) + "の呪いが解けた。")
	}
}

// swapPlaces sends a bolt that makes the player and the monster it hits
// change places.
func (g *Game) swapPlaces() {
	m := g.bolt()
	if m == nil || g.reflects(m, reflectStaffs) {
		// (Sent back, it changes the player with itself.)
		return
	}
	m.Dozing = false
	g.provoke(m)
	to := m.Pos
	m.Pos = g.Player
	g.say(m.Name + "と場所が入れ替わった。")
	g.shove(to)
}

// bolt sends a magic bolt from the player to its front, and returns the
// monster it hits (nil if none: it stops at a wall).
func (g *Game) bolt() *Monster {
	d := Directions[g.Facing]
	for p := g.Player; g.clearFrom(p, d); {
		p = p.Add(d)
		if !g.Level.At(p).Open() {
			break
		}
		if m := g.MonsterAt(p); m != nil {
			return m
		}
	}
	g.say("魔法弾は何にも当たらなかった。")
	return nil
}

// blowAwayTiles is how far a blow-away bolt sends a monster.
const blowAwayTiles = 10

// blowAwayDamage is the damage a monster blown away takes when it hits
// something.
const blowAwayDamage = 5

// blowAway sends a bolt that blows the monster it hits away from the
// player; hitting a wall or another monster on the way hurts it.
func (g *Game) blowAway() {
	m := g.bolt()
	if m == nil {
		return
	}
	if g.reflects(m, reflectStaffs) {
		g.say("吹き飛ばされた。")
		g.flingPlayer(back(Directions[g.Facing]), m)
		return
	}
	m.Dozing = false
	g.provoke(m)
	g.say(m.Name + "を吹き飛ばした。")
	g.fling(m, Directions[g.Facing], true)
}

// fling sends m flying by d, up to blowAwayTiles; hitting a wall or another
// creature on the way hurts it. byPlayer says whether the player (a staff)
// sent it, who then defeats it. It flies over water; one that does not
// swim, landing in it, is washed away somewhere on the floor.
func (g *Game) fling(m *Monster, d Point, byPlayer bool) {
	defer func() {
		if g.Level.At(m.Pos) == TileWater && !m.swims() && slices.Contains(g.Monsters, m) {
			if p, ok := g.freeTile(); ok {
				m.Pos = p
				g.say(m.Name + "は水路に落ちて、どこかへ流された。")
			}
		}
	}()
	for range blowAwayTiles {
		next := m.Pos.Add(d)
		if g.clearFrom(m.Pos, d) && g.Level.At(next).Open() && !g.occupied(next) {
			m.Pos = next
			continue
		}
		m.HP -= blowAwayDamage
		// (A blow frees it of かなしばり.)
		m.Bound = 0
		g.say(fmt.Sprintf("%sはぶつかって%dのダメージ。", m.Name, blowAwayDamage))
		if m.HP > 0 {
			g.wounded(m)
			return
		}
		g.kill(m, nil, byPlayer, true)
		return
	}
}

// starveWarning is every how many turns the starving player is told.
const starveWarning = 10

// hungerMessages are said as the fullness falls to them.
var hungerMessages = []struct {
	fullness int
	msg      string
}{
	{30, "お腹が減ってきた。"},
	{10, "早く何か食べないと……。"},
	{0, "お腹が減って倒れそうだ！"},
}

// getHungry makes the player hungrier as turns pass (twice as fast with a
// 回復 or ハラヘリ bracelet), and hurts it once it is starving: no HP
// comes back, and a point goes every turn.
func (g *Game) getHungry() {
	// A bracelet of 胃縮小 put on: what is over its cap is gone.
	g.Fullness = min(g.Fullness, g.fullnessCap())
	if g.Fullness == 0 {
		// Said again now and then: it is easy to miss once.
		if g.Turn%starveWarning == 0 {
			g.say("空腹でHPが減っている！　何か食べないと……。")
		}
		g.HP--
		if g.HP <= 0 {
			g.HP = 0
			g.end("飢え死にした", false)
		}
		return
	}
	// In hundredths of a turn's hunger: ハラヘラズ takes its share off.
	g.hunger += int(math.Round(100 * (1 - g.sealShare(SealNoHunger))))
	if g.wears(EffectRegen) || g.wears(EffectHungry) {
		g.hunger += 100
	}
	if g.hunger < 100*g.Dungeon.Rules.HungerTurns {
		return
	}
	g.hunger -= 100 * g.Dungeon.Rules.HungerTurns
	g.Fullness--
	for _, h := range hungerMessages {
		if g.Fullness == h.fullness {
			g.say(h.msg)
		}
	}
}

// light lights the floor: the player sees all of it.
func (g *Game) light() {
	g.Lit, g.Dark = true, false
	for i := range g.Seen {
		g.Seen[i] = true
	}
}
