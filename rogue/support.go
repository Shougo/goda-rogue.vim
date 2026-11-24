package rogue

import "fmt"

// support is m helping another monster it sees, by its abilities, in
// the place of its action: healing one that is hurt (AbilityHeal), or
// ranking one up (AbilityCheer). It says whether it did.
func (g *Game) support(m *Monster) bool {
	if a, ok := m.ability(AbilityHeal); ok && g.rng.IntN(100) < a.Values[0] {
		if f := g.helped(m, func(f *Monster) bool { return f.HP < f.MaxHP }); f != nil {
			n := min(a.Values[1], f.MaxHP-f.HP)
			f.HP += n
			if g.sees(g.Player, m.Pos) {
				g.say(fmt.Sprintf("%sは%sのHPを%d回復した。", m.Name, f.Name, n))
			}
			return true
		}
	}
	if a, ok := m.ability(AbilityCheer); ok && g.rng.IntN(100) < a.Values[0] {
		if f := g.helped(m, func(f *Monster) bool {
			_, cheers := f.ability(AbilityCheer)
			return f.Def.Kind == "" && !cheers && g.Dungeon.nextRank(f.Def) != nil
		}); f != nil {
			name := f.Name
			g.rankUp(f)
			if g.sees(g.Player, m.Pos) || g.sees(g.Player, f.Pos) {
				g.say(fmt.Sprintf("%sの祈りで%sは%sになった！", m.Name, name, f.Name))
			}
			return true
		}
	}
	return false
}

// helped returns the first other enemy that m sees and that ok accepts;
// nil if none.
func (g *Game) helped(m *Monster, ok func(*Monster) bool) *Monster {
	for _, f := range g.Monsters {
		if f != m && f.hostile() && g.sees(m.Pos, f.Pos) && ok(f) {
			return f
		}
	}
	return nil
}
