package rogue

// longHit is m that hits from two tiles away (AbilityLong) hitting the
// player there, on one of its lines with nothing in between; it says
// whether it did.
func (g *Game) longHit(m *Monster) bool {
	if _, ok := m.ability(AbilityLong); !ok {
		return false
	}
	d, n, ok := g.lineTo(m.Pos, g.Player, 2)
	if !ok || n != 2 || g.MonsterAt(m.Pos.Add(d)) != nil {
		return false
	}
	g.hitPlayer(m)
	return true
}

// blink is m that blinks (AbilityBlink), seeing the player not next to
// it, appearing on a free tile next to the player by its chance; it says
// whether it did.
func (g *Game) blink(m *Monster) bool {
	a, ok := m.ability(AbilityBlink)
	if !ok || !g.sees(m.Pos, g.Player) || chebyshev(m.Pos, g.Player) <= 1 || g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	var free []Point
	for _, d := range around {
		p := g.Player.Add(d)
		if g.Level.At(p).Walkable() && !g.occupied(p) && g.clearFrom(g.Player, d) {
			free = append(free, p)
		}
	}
	if len(free) == 0 {
		return false
	}
	m.Pos = free[g.rng.IntN(len(free))]
	g.say(m.Name + "が目の前に現れた！")
	return true
}
