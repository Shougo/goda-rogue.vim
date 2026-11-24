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

// knightMoves are the moves of a knight, in a fixed order.
var knightMoves = []Point{{1, -2}, {2, -1}, {2, 1}, {1, 2}, {-1, 2}, {-2, 1}, {-2, -1}, {-1, -2}}

// swoop is m that moves as a knight (AbilityKnight) swooping on the
// player a knight's move away; it says whether it did.
func (g *Game) swoop(m *Monster) bool {
	a, ok := m.ability(AbilityKnight)
	if !ok {
		return false
	}
	v := g.Player.Sub(m.Pos)
	for _, k := range knightMoves {
		if k == v {
			g.say(m.Name + "が舞い降りてきた！")
			g.playerHurt(a.Values[0], m)
			return true
		}
	}
	return false
}

// knightStep is m that moves as a knight jumping to the free tile that
// brings it nearest to to, if nearer than a step would; it says whether
// it moved.
func (g *Game) knightStep(m *Monster, to Point) bool {
	if _, ok := m.ability(AbilityKnight); !ok {
		return false
	}
	dist := g.distances(to)
	here, ok := g.index(m.Pos)
	if !ok || dist[here] < 0 {
		return false
	}
	best, bestDist := Point{}, dist[here]-1
	for _, k := range knightMoves {
		p := m.Pos.Add(k)
		i, ok := g.index(p)
		if !ok || dist[i] < 0 || dist[i] >= bestDist || g.occupied(p) {
			continue
		}
		best, bestDist = p, dist[i]
	}
	if bestDist >= dist[here]-1 {
		return false
	}
	m.Pos = best
	return true
}
