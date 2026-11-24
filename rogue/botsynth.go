package rogue

// The bots and the pot of synthesis: nothing comes out of it, so a merge
// takes four steps, one a turn, with no enemy in sight: the weapon (or
// shield) equipped goes in first, as the base; then the others of its
// category worth merging (an ability born with, seals, an upgrade value
// known); then the pot is thrown at a wall next to the player, and
// breaks; the rules to pick up and to equip do the rest, the base worth
// more than before (gearWorth).

// synthesize is the step of a merge the bot is at, if any.
func (b *Bot) synthesize(g *Game) (string, []string) {
	if len(g.visibleEnemies()) > 0 || g.HP*2 < g.MaxHP {
		return "", nil
	}
	pot := carried(g, func(it *Item) bool {
		return it.Def.Category == CategoryPot && b.known(g, it, EffectSynthesis) && !it.curseKnown()
	})
	if pot == nil {
		return "", nil
	}
	room := pot.Charges - len(pot.Contents)
	if len(pot.Contents) == 0 {
		for _, c := range []Category{CategoryWeapon, CategoryShield} {
			base := g.equipped(c)
			if room >= 2 && base != nil && !base.Cursed && b.material(g, base) != nil {
				return b.putIn(pot, base)
			}
		}
		return "", nil
	}
	base := pot.Contents[0]
	if c := base.Def.Category; c != CategoryWeapon && c != CategoryShield {
		return "", nil
	}
	if m := b.material(g, base); m != nil && room > 0 {
		return b.putIn(pot, m)
	}
	return b.breakPot(g, pot)
}

// material is an item carried worth merging into base: of its category,
// not equipped (unless nothing else is: the bots equip what they find
// once the base is in the pot), not known to be cursed (a curse comes
// along), with an ability born with, seals, or an upgrade value known
// above 0, and room on base for what it brings.
func (b *Bot) material(g *Game, base *Item) *Item {
	return carried(g, func(it *Item) bool {
		if it == base || it.Def.Category != base.Def.Category || it.curseKnown() || it.Equipped && it.Cursed {
			return false
		}
		seals := len(it.Seals)
		if it.Def.Seal != "" {
			seals++
		}
		return seals > 0 && len(base.Seals) < base.slots() || it.Known && it.Upgrade > 0
	})
}

// breakPot throws pot at a wall next to the player, facing one first
// (facing costs no turn); with no wall next to it, ahead.
func (b *Bot) breakPot(g *Game, pot *Item) (string, []string) {
	wall := func(name string) bool { return !g.Level.At(g.Player.Add(Directions[name])).Walkable() }
	if wall(g.Facing) {
		return b.use(pot, commandThrow)
	}
	for _, name := range []string{"up", "right", "down", "left"} {
		if wall(name) {
			return "turn", []string{name}
		}
	}
	return b.use(pot, commandThrow)
}
