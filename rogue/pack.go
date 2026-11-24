package rogue

import "slices"

// pack puts the monsters that come with m (AbilityPack) next to it, as
// it is (dozing or not), and returns them: as many as there is room for.
func (g *Game) pack(m *Monster) []*Monster {
	a, ok := m.ability(AbilityPack)
	if !ok {
		return nil
	}
	var ms []*Monster
	for _, d := range around {
		if len(ms) == a.Values[0] {
			break
		}
		p := m.Pos.Add(d)
		if !g.Level.At(p).Walkable() || !g.free(p) || p == g.Player {
			continue
		}
		f := newMonster(m.Def)
		f.Pos, f.Dozing = p, m.Dozing
		g.Monsters = append(g.Monsters, f)
		ms = append(ms, f)
	}
	return ms
}

// setTrap is m hiding a trap of the floor's table where it stands, if a
// trap can be there; it says whether it did.
func (g *Game) setTrap(m *Monster) bool {
	d := g.Dungeon
	if !slices.ContainsFunc(d.TrapRows, func(r FloorRow) bool { return r.First <= g.Depth && g.Depth <= r.Last && r.Weight > 0 }) {
		return false
	}
	def := d.Trap(pick(g.rng, d.TrapRows, g.Depth))
	if def == nil || g.TrapAt(m.Pos) != nil || !g.trapSpot(m.Pos, def) || g.inShop(m.Pos) || g.stairsAt(m.Pos) {
		return false
	}
	g.Traps = append(g.Traps, &Trap{Def: def, Pos: m.Pos})
	if g.sees(g.Player, m.Pos) {
		g.say(m.Name + "は何かを仕掛けた。")
	}
	return true
}
