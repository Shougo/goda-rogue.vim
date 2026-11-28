package rogue

import "slices"

// The bots and the states (Shougo, 2026-11-25): keep off the monsters
// that bring a state (the tiles next to those that do it next to the
// player are kept off as the lines of a charge are, reachLines); once
// near one and low on HP, deal with it by an item; confused, use items,
// not a step (it goes anywhere); confused with no enemy near, wait for it
// to pass. Blind (Shougo): the map seen before is remembered; with an
// enemy next to it, no step (it fights or uses items); with none, it
// waits, and hit from out of sight, it flees (a warp, the stairs).

// neutralizers are the effects of a bolt (a staff, a grass thrown, a
// scroll) that stop a monster that brings a state.
var neutralizers = []MagicEffect{MagicSleep, MagicSeal, MagicWarp, MagicBlow, MagicConfuse, MagicBlind, MagicLevel1, MagicLevel3}

// afflicters are the enemies that may bring a state to the player now:
// awake, next to it, or seeing it with the reach to.
func (b *Bot) afflicters(g *Game) []*Monster {
	var ms []*Monster
	for _, m := range g.visibleEnemies() {
		a, ok := m.ability(AbilityAfflict)
		if !ok || m.Dozing || m.Asleep > 0 || m.Blind > 0 || m.Confused > 0 {
			continue
		}
		if chebyshev(m.Pos, g.Player) == 1 || a.Values[1] == 1 && g.sees(m.Pos, g.Player) {
			ms = append(ms, m)
		}
	}
	return ms
}

// afflicted is the rule of the states: an item at an enemy that brings
// one, and waiting out a state with no enemy next to the player.
func (b *Bot) afflicted(g *Game) (string, []string) {
	hurt := g.HP < b.lastHP
	b.lastHP = g.HP
	if g.Blind > 0 {
		// A grass of eye drops cures it.
		if it := carried(g, func(it *Item) bool {
			return it.Def.Category == CategoryGrass && it.Def.Effect == EffectEyeDrops && !b.unsure(g, it) && !it.curseKnown()
		}); it != nil && !g.Sealed {
			return b.use(it, commandDrink)
		}
	}
	if g.Blind > 0 && len(b.adjacent(g)) == 0 {
		if hurt {
			// Hit from out of sight: away, by a warp, or to the stairs
			// it remembers (Shougo: hard to answer, flee first).
			if it := carried(g, func(it *Item) bool {
				return it.Def.Category == CategoryGrass && it.Def.Effect == Effect(MagicWarp) && !b.unsure(g, it)
			}); it != nil {
				return b.use(it, commandDrink)
			}
			if g.IsSeen(g.Stairs) {
				if g.Player == g.Stairs {
					return "stairs", nil
				}
				if action, args := b.walk(g, func(p Point) bool { return p == g.Stairs }); action != "" {
					return action, args
				}
			}
		}
		// It sees nothing: a step may meet what it does not see.
		return "wait", nil
	}
	if g.Confused > 0 && len(b.adjacent(g)) == 0 && len(b.afflicters(g)) == 0 {
		return "wait", nil
	}
	// Only low on HP: alone, one is fought as any enemy (the items
	// spent stopping every one were missed later; in the deep cave the
	// mean floor reached fell from 25.0 to 21.4).
	if g.HP*2 >= g.MaxHP {
		return "", nil
	}
	for _, m := range b.afflicters(g) {
		if a, ok := b.stopper(g, m); ok {
			return b.take(g, &a)
		}
	}
	return "", nil
}

// stopper is the cheapest item the bot knows to stop m: a staff waved at
// it (next to the player or on a line), a grass thrown at it, a scroll
// read (next to it), of an effect that stops it (neutralizers).
func (b *Bot) stopper(g *Game, m *Monster) (botAnswer, bool) {
	var best botAnswer
	found := false
	near := chebyshev(m.Pos, g.Player) == 1
	_, _, line := g.lineTo(g.Player, m.Pos, throwRange)
	if near {
		line = true
	}
	for _, it := range g.Inventory {
		if it.ForSale || spoiled(it) || b.unsure(g, it) || it.Cursed && it.curseKnown() {
			continue
		}
		e := MagicEffect(it.Def.Effect)
		var command string
		switch it.Def.Category {
		case CategoryStaff:
			if it.Charges == 0 && it.Known || !line || sendsBack(m, reflectStaffs) {
				continue
			}
			command = commandWave
		case CategoryGrass:
			if !line {
				continue
			}
			command = commandThrow
		case CategoryScroll:
			if !near {
				continue
			}
			command = commandRead
		default:
			continue
		}
		if it.Def.Effect == EffectBlowAway {
			e = MagicBlow
		}
		if !slices.Contains(neutralizers, e) {
			continue
		}
		cost := b.worth(g, it, false)
		if !found || cost < best.cost {
			best = botAnswer{cost, 0, it, command, m, nil, false, ""}
			found = true
		}
	}
	return best, found
}
