package rogue

import (
	"fmt"
	"slices"
)

// still says whether m never moves by itself (AbilityStill); a staff, a
// blow or a charge may still move it.
func (m *Monster) still() bool {
	_, ok := m.ability(AbilityStill)
	return ok
}

// holdBy is m, after a blow that hit the player, holding it by its
// ability (AbilityHold): the player cannot step away (Stuck) for so many
// turns.
func (g *Game) holdBy(m *Monster) {
	a, ok := m.ability(AbilityHold)
	if !ok || g.rng.IntN(100) >= a.Values[0] || g.Stuck >= a.Values[1] {
		return
	}
	g.Stuck, g.heldBy = a.Values[1], m
	g.say(m.Name + "に捕まった！")
}

// stillHeld says whether the player is still held, Stuck: by a snare, or
// by a monster there and next to it. A monster gone lets it go.
func (g *Game) stillHeld() bool {
	if m := g.heldBy; m != nil && (!slices.Contains(g.Monsters, m) || chebyshev(m.Pos, g.Player) > 1) {
		g.Stuck, g.heldBy = 0, nil
		return false
	}
	return true
}

// burn is the item it, thrown or shot by the player, reaching m: a
// monster that burns things (AbilityBurn) burns it, with no effect. It
// says whether it burnt.
func (g *Game) burn(m *Monster, it *Item) bool {
	if _, ok := m.ability(AbilityBurn); !ok {
		return false
	}
	g.provoke(m)
	if grillable(it) && g.Dungeon.grilledDef() != nil {
		// (An onigiri is grilled, and falls by it.)
		g.grill(it)
		g.say(m.Name + "に焼かれて、焼きおにぎりになった。")
		g.land(it, m.Pos)
		return true
	}
	g.say(g.Name(it) + "は" + m.Name + "に燃やされた！")
	return true
}

// holdText describes AbilityHold.
func holdText(v []int) string {
	return fmt.Sprintf("攻撃が当たると%d%%で%dターン捕まえる。離れられない", v[0], v[1])
}
