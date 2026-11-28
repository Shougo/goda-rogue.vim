package rogue

import "fmt"

// fullnessBack is how much of the max fullness a hungry ghost took comes
// back when the player eats more than it can hold.
const fullnessBack = 5

// fill is the player eating n of fullness: no more than it can hold
// (MaxFullness); what is too much brings back some of the max fullness a
// hungry ghost took, up to the rule.
func (g *Game) fill(n int) {
	g.hunger = 0
	g.Fullness += n
	if over := g.Fullness - g.MaxFullness; over > 0 && g.MaxFullness < g.Dungeon.Rules.Fullness {
		g.MaxFullness = min(g.MaxFullness+min(over, fullnessBack), g.Dungeon.Rules.Fullness)
		g.say("最大満腹度が少し戻った。")
	}
	g.Fullness = min(g.Fullness, g.fullnessCap())
}

// shrinkFullness is how much less fullness the player holds with a
// bracelet of 胃縮小 (Shougo: 25).
const shrinkFullness = 25

// fullnessCap is how full the player can be now: the max fullness, less
// with a bracelet of 胃縮小 (not under 1), more with one of 胃拡張.
func (g *Game) fullnessCap() int {
	if g.wears(EffectSmallStomach) {
		return max(g.MaxFullness-shrinkFullness, 1)
	}
	if b := g.bracelet(EffectBigStomach); b != nil {
		return g.MaxFullness + b.Def.Strength
	}
	return g.MaxFullness
}

// fullnessLabel is the fullness of the status: a percent of the rule,
// and the max when a hungry ghost or a bracelet lowered it ("80%（最大
// 90%）").
func (g *Game) fullnessLabel() string {
	rule := g.Dungeon.Rules.Fullness
	s := fmt.Sprintf("%d%%", g.Fullness*100/rule)
	if c := g.fullnessCap(); c < rule {
		s += fmt.Sprintf("（最大 %d%%）", c*100/rule)
	}
	return s
}

// minMaxFullness is the least max fullness a hungry ghost leaves, a
// percent of the rule.
const minMaxFullness = 20

// starve is the hungry ghost m next to the player making it hungry, by
// its ability a (AbilityHunger), instead of a blow: the fullness goes
// down, or the max fullness (not under minMaxFullness). It says whether
// it did something: with nothing to take, it attacks.
func (g *Game) starve(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	if g.wears(EffectNoStarve) {
		g.use(EffectNoStarve)
		g.say(m.Name + "は満腹度を吸おうとした。しかし空腹よけの腕輪が守った。")
		return true
	}
	if g.sealChance(SealNoStarve) {
		g.say(m.Name + "は満腹度を吸おうとした。しかし盾が守った。")
		return true
	}
	least := g.Dungeon.Rules.Fullness * minMaxFullness / 100
	switch n, top := a.Values[1], a.Values[2]; {
	case top > 0 && g.MaxFullness > least:
		g.MaxFullness = max(g.MaxFullness-top, least)
		g.Fullness = min(g.Fullness, g.MaxFullness)
		g.say(m.Name + "に最大満腹度を吸われた！")
	case n > 0 && g.Fullness > 0:
		g.Fullness = max(g.Fullness-n, 0)
		g.say(m.Name + "に満腹度を吸われた！")
	default:
		return false
	}
	g.event("starved %s", m.Def.ID)
	return true
}

// feed is the food it thrown at m: a hungry ghost eats it and rests in
// peace (the player defeated it). It says whether m was one.
func (g *Game) feed(m *Monster, it *Item) bool {
	if _, ok := m.ability(AbilityHunger); !ok || it.Def.Category != CategoryFood {
		return false
	}
	g.say(m.Name + "は" + g.Name(it) + "を食べて成仏した。")
	g.kill(m, nil, true, true)
	return true
}

// hungerText describes the ability.
func hungerText(v []int) string {
	what := fmt.Sprintf("満腹度を%d", v[1])
	if v[2] > 0 {
		what = fmt.Sprintf("最大満腹度を%d", v[2])
	}
	return fmt.Sprintf("隣にいると%d%%で%s吸う。食料を投げると成仏する", v[0], what)
}
