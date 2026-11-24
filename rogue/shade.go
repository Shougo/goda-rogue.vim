package rogue

// hidden says whether m is invisible to the player (AbilityInvisible):
// unless eye drops cleared its eyes on the floor.
func (g *Game) hidden(m *Monster) bool {
	_, ok := m.ability(AbilityInvisible)
	return ok && !g.Clear
}

// seenName is the name of m as the player sees it: "何者か" if hidden.
func (g *Game) seenName(m *Monster) string {
	if g.hidden(m) {
		return "何者か"
	}
	return m.Name
}

// throughWalls is m that goes through rock (AbilityWalls) stepping
// toward to, on any tile of the level but where a creature stands: the
// nearest to it, out of rock when it may (it fights only out of rock).
// It says whether it moved.
func (g *Game) throughWalls(m *Monster, to Point) bool {
	if _, ok := m.ability(AbilityWalls); !ok {
		return false
	}
	best, bestScore := m.Pos, chebyshev(m.Pos, to)*2
	if !g.Level.At(m.Pos).Walkable() {
		bestScore++
	}
	for _, d := range around {
		p := m.Pos.Add(d)
		if _, ok := g.index(p); !ok || g.occupied(p) {
			continue
		}
		score := chebyshev(p, to) * 2
		if !g.Level.At(p).Walkable() {
			score++
		}
		if score < bestScore {
			best, bestScore = p, score
		}
	}
	if best == m.Pos {
		return false
	}
	m.Pos = best
	return true
}
