package rogue

import (
	"fmt"
	"slices"
)

// revive is m defeated: the first time, a monster that gets up
// (AbilityRevive) does, with a share of its max HP, and gives nothing
// yet. It says whether it got up.
func (g *Game) revive(m *Monster) bool {
	a, ok := m.ability(AbilityRevive)
	if !ok || m.Revived {
		return false
	}
	m.Revived, m.Fused = true, false
	m.HP = max(m.MaxHP*a.Values[0]/100, 1)
	if g.sees(g.Player, m.Pos) {
		g.say(m.Name + "は倒れたが、また起き上がった！")
	}
	return true
}

// fuse is m hurt: one that explodes (AbilityExplode), down to its share
// of HP, will at its next action.
func (g *Game) fuse(m *Monster) {
	a, ok := m.ability(AbilityExplode)
	if !ok || m.Fused || m.HP*100 > m.MaxHP*a.Values[0] {
		return
	}
	m.Fused = true
	if g.sees(g.Player, m.Pos) {
		g.say(m.Name + "は今にも爆発しそうだ！")
	}
}

// explode is m exploding: every creature next to it loses its share of
// HP (at least 1, and a monster may die of it), the items next to it and
// under it burn, and it is gone, giving nothing.
func (g *Game) explode(m *Monster) {
	a, _ := m.ability(AbilityExplode)
	shown := g.sees(g.Player, m.Pos)
	g.remove(m)
	g.say(m.Name + "が爆発した！")
	share := 100
	if len(a.Values) > 1 {
		share = a.Values[1]
	}
	g.Items = slices.DeleteFunc(g.Items, func(it *Item) bool { return chebyshev(it.Pos, m.Pos) <= 1 })
	for _, f := range slices.Clone(g.Monsters) {
		if chebyshev(f.Pos, m.Pos) > 1 {
			continue
		}
		n := max(f.HP*share/100, 1)
		f.HP -= n
		// (A blow frees it of かなしばり.)
		f.Bound = 0
		f.Dozing = false
		if shown {
			g.say(fmt.Sprintf("%sに%dのダメージ。", f.Name, n))
		}
		if f.HP <= 0 {
			g.kill(f, nil, false, shown)
		} else {
			g.wounded(f)
		}
	}
	if chebyshev(g.Player, m.Pos) <= 1 {
		g.playerHurt(g.blastDamage(max(g.HP*share/100, 1)), m)
	}
}
