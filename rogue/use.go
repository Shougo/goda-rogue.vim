package rogue

import (
	"fmt"
)

// useItem uses it: drinks a grass, reads a scroll, waves a staff, eats
// food. All but a staff are used up.
func (g *Game) useItem(it *Item) {
	if it.Def.Category != CategoryStaff {
		g.discard(it)
	}
	name := g.Name(it)
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
		it.Charges--
	case CategoryFood:
		g.say(name + "を食べた。")
	}

	// Using it tells what it is.
	g.identify(it)

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
	case EffectIdentify:
		if !g.askIdentify() {
			g.say("識別するものはなかった。")
		}
	case EffectBlowAway:
		g.blowAway()
	case EffectFood:
		g.Fullness = min(g.Fullness+n, g.Dungeon.Rules.Fullness)
		g.hunger = 0
		g.say("お腹がふくれた。")
	default:
		g.say("何も起こらなかった。")
	}
}

// bolt sends a magic bolt from the player to its front, and returns the
// monster it hits (nil if none: it stops at a wall).
func (g *Game) bolt() *Monster {
	d := Directions[g.Facing]
	for p := g.Player; g.clearFrom(p, d); {
		p = p.Add(d)
		if !g.Level.At(p).Walkable() {
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
	m.Dozing = false
	g.provoke(m)
	g.say(m.Name + "を吹き飛ばした。")
	g.fling(m, Directions[g.Facing], true)
}

// fling sends m flying by d, up to blowAwayTiles; hitting a wall or another
// creature on the way hurts it. byPlayer says whether the player (a staff)
// sent it, who then defeats it.
func (g *Game) fling(m *Monster, d Point, byPlayer bool) {
	for range blowAwayTiles {
		next := m.Pos.Add(d)
		if g.clearFrom(m.Pos, d) && g.Level.At(next).Walkable() && !g.occupied(next) {
			m.Pos = next
			continue
		}
		m.HP -= blowAwayDamage
		g.say(fmt.Sprintf("%sはぶつかって%dのダメージ。", m.Name, blowAwayDamage))
		if m.HP > 0 {
			g.wounded(m)
			return
		}
		g.remove(m)
		if !byPlayer {
			g.say(m.Name + "は倒れた。")
			return
		}
		g.say(m.Name + "を倒した。")
		if !m.Ally {
			g.defeated(m)
		}
		return
	}
}

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
// 回復 bracelet), and hurts it once it is starving: no HP comes back, and
// a point goes every turn.
func (g *Game) getHungry() {
	if g.Fullness == 0 {
		g.HP--
		if g.HP <= 0 {
			g.HP = 0
			g.end("飢え死にした", false)
		}
		return
	}
	// The seal ハラヘラズ: every other turn only.
	if !g.sealed(SealNoHunger) || g.Turn%2 == 0 {
		g.hunger++
	}
	if g.wears(EffectRegen) {
		g.hunger++
	}
	if g.hunger < g.Dungeon.Rules.HungerTurns {
		return
	}
	g.hunger -= g.Dungeon.Rules.HungerTurns
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
