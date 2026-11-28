package rogue

import "slices"

// The answers to a danger of the staffs that send no bolt at a monster
// (staff2.go): a wall in a corridor between the player and one coming,
// and a leap away, closer to the stairs.

// staffAnswers returns the answers of the staff it, known to be of the
// kind def, to the enemies ms.
func (b *Bot) staffAnswers(g *Game, ms []*Monster, it *Item, def *ItemDef, cost float64) []botAnswer {
	switch def.Effect {
	case EffectWall:
		if m, ok := b.wallOff(g, ms); ok {
			rest := slices.DeleteFunc(slices.Clone(ms), func(o *Monster) bool { return o == m })
			return []botAnswer{{cost, fightDeath(g, rest, g.HP, 1), it, commandWave, m, nil, false, ""}}
		}
	case EffectLeapTo:
		if dir, chance, ok := b.leapAway(g); ok {
			return []botAnswer{{cost, chance, it, commandWave, nil, nil, false, dir}}
		}
	}
	return nil
}

// wallOff returns an enemy of ms coming two tiles away along a corridor,
// on a line, that a wall on the corridor tile between would keep out: the
// tile empty (no one, no item, no stairs, no trap the player knows of),
// the enemy not one that walks through walls nor strikes from afar.
func (b *Bot) wallOff(g *Game, ms []*Monster) (*Monster, bool) {
	for _, m := range ms {
		if m.still() || strikesFromAfar(m) || chebyshev(m.Pos, g.Player) != 2 {
			continue
		}
		if _, ok := m.ability(AbilityWalls); ok {
			continue
		}
		v := m.Pos.Sub(g.Player)
		if v.X != 0 && v.Y != 0 && abs(v.X) != abs(v.Y) {
			continue
		}
		d := Point{sign(v.X), sign(v.Y)}
		p := g.Player.Add(d)
		if g.Level.At(p) != TileCorridor || !g.clearFrom(g.Player, d) || g.occupied(p) ||
			g.stairsAt(p) || g.foundTrapAt(p) != nil || g.inShop(p) {
			continue
		}
		if _, ok := g.ItemAt(p); ok {
			continue
		}
		return m, true
	}
	return nil, false
}

// leapAway returns the way to leap (飛びつき) that leaves the player the
// least chance of dying walking on to the stairs, with that chance: the
// enemies left behind follow as fleeDeath reckons. Not without the
// stairs in sight, nor a way that lands next to the player.
func (b *Bot) leapAway(g *Game) (string, float64, bool) {
	if !b.seesStairs(g) || g.Stuck > 0 {
		return "", 0, false
	}
	// The ways worked out for this action are from where the player
	// stands: not to be kept for the tiles it would land on (the bots
	// then walked a step of nowhere).
	deciding := b.deciding
	b.deciding = false
	defer func() { b.deciding = deciding }()
	best, least := "", 0.0
	for _, name := range clockwise {
		d := Directions[name]
		to := g.Player
		for p := g.Player; g.clearFrom(p, d); {
			next := p.Add(d)
			if !g.Level.At(next).Walkable() || g.MonsterAt(next) != nil {
				break
			}
			p = next
			to = p
		}
		if chebyshev(to, g.Player) < 2 {
			continue
		}
		h := g.Clone()
		h.Player = to
		chance := 0.0
		if to != h.Stairs {
			step, dist, ok := b.route(h, func(p Point) bool { return b.stairs(h, p) })
			if !ok {
				continue
			}
			chance = fleeDeath(h, b.threats(h), h.HP, dist, step)
		}
		if best == "" || chance < least {
			best, least = name, chance
		}
	}
	return best, least, best != ""
}
