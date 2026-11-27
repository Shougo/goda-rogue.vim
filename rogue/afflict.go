package rogue

import (
	"fmt"
	"math"
)

// The monsters of states (phase C, Shougo 2026-11-25): 梟 blinds, 霧 puts
// to sleep, 眼 confuses, 仙 weakens (its attack or its defense, at
// random). Ranks 1 and 2 next to the player, 3 and 4 from anywhere in
// sight. The bracelets of 睡眠よけ and 混乱よけ ward sleep and confusion
// off, from any source.

// What a monster of states (AbilityAfflict) brings.
const (
	afflictBlind   = 0
	afflictSleep   = 1
	afflictConfuse = 2
	afflictWeaken  = 3
)

// The weakening: its turns, and what it takes.
const (
	weakTurns = 20
	// weakShare is the share of the damage the player deals less (attack
	// down), or takes more (defense down).
	weakShare = 0.3
)

// afflict brings the state e to the player, unless a bracelet wards it
// off.
func (g *Game) afflict(e MagicEffect) {
	switch {
	case g.prevents(e):
		return
	case e == MagicSleep && g.wears(EffectNoSleep):
		g.say("睡眠よけの腕輪が眠気を払った。")
		return
	case e == MagicConfuse && g.wears(EffectNoConfuse):
		g.say("混乱よけの腕輪が守った。")
		return
	case e == MagicBlind && g.wears(EffectNoBlind):
		g.use(EffectNoBlind)
		g.say("目つぶしよけの腕輪が守った。")
		return
	case e == MagicSeal && g.wears(EffectNoSeal):
		g.use(EffectNoSeal)
		g.say("口封じよけの腕輪が守った。")
		return
	case e == MagicSleep && g.sealChance(SealNoSleep), e == MagicConfuse && g.sealChance(SealNoConfuse),
		e == MagicBlind && g.sealChance(SealNoBlind):
		g.say("盾が守った。")
		return
	}
	g.States.take(e)
	g.say(stateTexts[e].player)
}

// weakened is the damage n the player takes, more with its defense down;
// none while unhurt (無敵).
func (g *Game) weakened(n int) int {
	if g.Unhurt > 0 {
		return 0
	}
	if g.DefenseDown > 0 {
		n = int(math.Ceil(float64(n) * (1 + weakShare)))
	}
	if s := g.sealShare(SealFine); s > 0 && n > 0 && g.HP >= g.MaxHP {
		// 絶好調: full HP takes less (not under 1).
		n = max(int(math.Round(float64(n)*(1-s))), 1)
	}
	return n
}

// cushioned is the damage n of a blow of a monster on the player, less
// with a bracelet of 胃縮小 (Shougo: blows only; never under 1).
func (g *Game) cushioned(n int) int {
	if b := g.equipped(CategoryBracelet); b != nil && b.Def.Effect == EffectSmallStomach && n > 0 {
		g.use(EffectSmallStomach)
		n = max(int(math.Round(float64(n)*float64(100-b.Def.Strength)/100)), 1)
	}
	return n
}

// weakening is the damage n the player deals, less with its attack down.
func (g *Game) weakening(n int) int {
	if g.AttackDown > 0 && n > 0 {
		return max(int(math.Round(float64(n)*(1-weakShare))), 1)
	}
	return n
}

// afflictBy is m bringing its state to the player, by its ability a
// (AbilityAfflict): next to it instead of a blow, or (with sight) from
// anywhere it sees the player, by its chance. It says whether it did.
func (g *Game) afflictBy(m *Monster, a Ability) bool {
	near := chebyshev(m.Pos, g.Player) == 1
	if !near && (a.Values[1] == 0 || !g.sees(m.Pos, g.Player)) || g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	turns := a.Values[3]
	g.event("afflicted %s", m.Def.ID)
	switch a.Values[2] {
	case afflictBlind:
		g.say(m.Name + "は目をくらませてきた！")
		g.afflict(MagicBlind)
		if g.Blind > 0 {
			g.Blind = turns
		}
	case afflictSleep:
		g.say(m.Name + "は眠りの霧を吹きかけた！")
		g.afflict(MagicSleep)
		if g.Asleep > 0 {
			g.Asleep = turns
		}
	case afflictConfuse:
		g.say(m.Name + "はにらみつけた！")
		g.afflict(MagicConfuse)
		if g.Confused > 0 {
			g.Confused = turns
		}
	case afflictWeaken:
		g.say(m.Name + "は力を吸い取ろうとした！")
		if g.Prevent {
			g.say("予防の力で防いだ。")
			return true
		}
		if g.wears(EffectNoWeak) {
			g.use(EffectNoWeak)
			g.say("弱りよけの腕輪が守った。")
			return true
		}
		if g.sealChance(SealNoWeak) {
			g.say("盾が守った。")
			return true
		}
		w, name := &g.AttackDown, "攻撃力"
		if g.rng.IntN(2) == 0 {
			w, name = &g.DefenseDown, "防御力"
		}
		if *w > 0 {
			*w += turns
			g.say(fmt.Sprintf("%sがさらに下がり続ける……。", name))
		} else {
			*w = turns
			g.say(name + "が下がった！")
		}
	}
	return true
}

// actBlind is a blind monster's action: a step at random, no blow.
func (g *Game) actBlind(m *Monster) {
	m.Blind--
	d := around[g.rng.IntN(len(around))]
	p := m.Pos.Add(d)
	if g.clearFrom(m.Pos, d) && g.Level.At(p).Walkable() && !g.occupied(p) {
		m.Pos = p
	}
}
