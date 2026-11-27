package rogue

import "fmt"

// breathFloor is the reach of a breath from anywhere on the floor.
const breathFloor = 99

// breathe is m breathing fire on the player (AbilityBreath), if it can
// and its chance says so; it says whether it did. The player is never
// next to it (it bites then).
func (g *Game) breathe(m *Monster, a Ability) bool {
	if m.breathWait > 0 {
		m.breathWait--
		return false
	}
	dist := chebyshev(m.Pos, g.Player)
	if dist < 2 {
		return false
	}
	room := g.Level.RoomAt(m.Pos)
	sameRoom := room >= 0 && room == g.Level.RoomAt(g.Player)
	_, _, onLine := g.lineTo(m.Pos, g.Player, maxReach)
	switch reach := a.Values[0]; {
	case reach == 0:
		if !onLine || !g.sees(m.Pos, g.Player) {
			return false
		}
	case reach == 1:
		if !sameRoom {
			return false
		}
	case reach < breathFloor:
		if dist > reach {
			return false
		}
	}
	if g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	m.breathWait = dist / 2
	if sameRoom && onLine {
		m.breathWait = 0
	}
	g.say(fmt.Sprintf("%sは炎を吐いた！", g.seenName(m)))
	g.burnPlayer(a.Values[2], m)
	return true
}
