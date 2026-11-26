package rogue

import "slices"

// What an item does to a fight, as the bots reckon: the chance that the
// player dies fighting the enemies ms after a turn spent on the item.

// meanOf returns the mean of f over the kinds an item may be.
func meanOf(kinds []*ItemDef, f func(*ItemDef) float64) float64 {
	t := 0.0
	for _, def := range kinds {
		t += f(def)
	}
	return t / float64(max(len(kinds), 1))
}

// best returns the monster of ms with the least chance, and the chance
// (nil with none).
func (b *Bot) best(ms []*Monster, chance func(*Monster) float64) (*Monster, float64) {
	var pick *Monster
	least := 0.0
	for _, m := range ms {
		if c := chance(m); pick == nil || c < least {
			pick, least = m, c
		}
	}
	return pick, least
}

// affected returns the enemies ms with m as a bolt of e leaves it: out of
// the fight (warped), or changed (asleep, confused, slowed, sealed, a
// rank down, its HP cut). An effect that does not help leaves them as
// they are.
func affected(g *Game, ms []*Monster, m *Monster, e MagicEffect) []*Monster {
	c := *m
	switch e {
	case MagicWarp:
		return slices.DeleteFunc(slices.Clone(ms), func(o *Monster) bool { return o == m })
	case MagicConfuse, MagicSleep, MagicSeal:
		c.States.take(e)
	case MagicSlow:
		if m.Slowed > 0 {
			// Slowed already: another wave does nothing more (the bots
			// waved at a slowed tree turn after turn).
			return ms
		}
		// Every other turn: about a speed down.
		switch c.Speed {
		case SpeedNormal:
			c.Speed = SpeedSlow
		case SpeedDouble, SpeedMoveDouble:
			c.Speed = SpeedNormal
		}
	case MagicLevel1, MagicLevel3:
		for range levels(e) {
			if prev := g.Dungeon.prevRank(c.Def); prev != nil {
				c.Def, c.MaxHP, c.Attack, c.Defense, c.Speed = prev, prev.HP, prev.Attack, prev.Defense, prev.Speed
			}
		}
	case MagicHalf, MagicQuarter:
		c.MaxHP = max(c.MaxHP/fraction(e), 1)
	default:
		return ms
	}
	out := slices.Clone(ms)
	out[slices.Index(out, m)] = &c
	return out
}

// drinkDeath is the chance after drinking a grass of the kind def:
// healed, the max HP raised, warped away (the enemies next to it attack
// once more), asleep (they attack all the while); another does nothing
// for the fight.
func drinkDeath(g *Game, ms []*Monster, def *ItemDef) float64 {
	switch def.Effect {
	case EffectHeal:
		return fightDeath(g, ms, min(g.HP+def.Strength, g.MaxHP), 1)
	case EffectMaxHP:
		return fightDeath(g, ms, g.HP+def.Strength, 1)
	}
	switch MagicEffect(def.Effect) {
	case MagicWarp:
		var foes []foe
		for _, m := range ms {
			if arrival(g, m) == 0 {
				foes = append(foes, foe{hitDamages(g, m), 1, 1, m})
			}
		}
		return deathDP(g, g.HP, foes)
	case MagicSleep:
		return fightDeath(g, ms, g.HP, 1+sleepTurns)
	}
	return fightDeath(g, ms, g.HP, 1)
}

// throwDeath is the chance after throwing a grass of the kind def at m,
// next to the player: its effect on m if it hits (and it helps), else
// nothing.
func throwDeath(g *Game, ms []*Monster, m *Monster, def *ItemDef) float64 {
	miss := fightDeath(g, ms, g.HP, 1)
	e := MagicEffect(def.Effect)
	if !castable(CategoryGrass, e) || e == MagicHaste || m.room() > 0 {
		// (One of synthesis swallows it.)
		return miss
	}
	hit := float64(g.Dungeon.Rules.PlayerThrowRate) / 100
	return hit*fightDeath(g, affected(g, ms, m, e), g.HP, 1) + (1-hit)*miss
}

// feedDeath is the chance after throwing food at m next to the player:
// a hungry ghost hit is gone (feed); another takes a blow of 1 or 2.
func feedDeath(g *Game, ms []*Monster, m *Monster) float64 {
	miss := fightDeath(g, ms, g.HP, 1)
	if _, ok := m.ability(AbilityHunger); !ok || m.room() > 0 {
		return miss
	}
	hit := float64(g.Dungeon.Rules.PlayerThrowRate) / 100
	without := fightDeath(g, slices.DeleteFunc(slices.Clone(ms), func(o *Monster) bool { return o == m }), g.HP, 1)
	return hit*without + (1-hit)*miss
}

// readDeath is the chance after reading a scroll of the kind def: its
// effect on the enemies of the room, if it has one of a bolt.
func readDeath(g *Game, ms []*Monster, def *ItemDef) float64 {
	if def.Effect == EffectUncurse {
		// The monsters that curse around are gone (exorcise).
		around := g.enemiesAround()
		return fightDeath(g, slices.DeleteFunc(slices.Clone(ms), func(m *Monster) bool {
			_, ok := m.ability(AbilityCurse)
			return ok && slices.Contains(around, m)
		}), g.HP, 1)
	}
	switch def.Effect {
	case EffectWipe, EffectSlash:
		// Those around die (not a shopkeeper), or lose its strength
		// in HP.
		around := g.enemiesAround()
		var out []*Monster
		for _, m := range ms {
			switch {
			case !slices.Contains(around, m):
				out = append(out, m)
			case def.Effect == EffectWipe && m.Keeper:
				out = append(out, m)
			case def.Effect == EffectSlash && m.HP > def.Strength:
				c := *m
				c.MaxHP = m.HP - def.Strength
				out = append(out, &c)
			}
		}
		return fightDeath(g, out, g.HP, 1)
	}
	e := MagicEffect(def.Effect)
	if !castable(CategoryScroll, e) || e == MagicHaste {
		return fightDeath(g, ms, g.HP, 1)
	}
	around := g.enemiesAround()
	for _, m := range ms {
		if slices.Contains(around, m) {
			ms = affected(g, ms, m, e)
		}
	}
	return fightDeath(g, ms, g.HP, 1)
}

// waveDeath is the chance after waving a staff of the kind def at m,
// next to the player: blown away (if it flies far enough), an ally (if
// it can be one), or the effect of a bolt on it.
func waveDeath(g *Game, ms []*Monster, m *Monster, def *ItemDef) float64 {
	without := func() float64 {
		return fightDeath(g, slices.DeleteFunc(slices.Clone(ms), func(o *Monster) bool { return o == m }), g.HP, 1)
	}
	switch def.Effect {
	case EffectBlowAway:
		// One that hits a wall or a monster at once stays next to
		// the player, or comes back at once: it only takes the bump.
		if blowAwayFlight(g, m) >= minFlight {
			return without()
		}
	case EffectRecruit:
		if !m.Keeper && m.Def.Kind == "" && len(g.allies()) < g.Dungeon.Rules.MaxAllies {
			return without()
		}
	default:
		if e := MagicEffect(def.Effect); castable(CategoryStaff, e) && e != MagicHaste && e != MagicBlow {
			return fightDeath(g, affected(g, ms, m, e), g.HP, 1)
		}
	}
	return fightDeath(g, ms, g.HP, 1)
}
