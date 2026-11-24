package rogue

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
