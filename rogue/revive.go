package rogue

// 復活の草 (Shougo, 2026-11-27, as in Shiren): carried (not in a pot), it
// brings the player back with full HP once, when it falls; unknown too,
// with a sealed mouth too. Drunk, it does nothing. It does not take the
// fullness or the states back, nor help against the wind.

// revived is the player falling, brought back by a grass of 復活 it
// carries (not cursed, not for sale), which is used up. It says whether it
// was.
func (g *Game) revived() bool {
	grass := carried(g, func(it *Item) bool {
		return it.Def.Category == CategoryGrass && it.Def.Effect == EffectRevive && !it.Cursed && !it.ForSale
	})
	if grass == nil {
		return false
	}
	g.discard(grass)
	g.identify(grass)
	g.HP = g.MaxHP
	g.event("revive %s", grass.Def.ID)
	g.say(g.Name(grass) + "の力で生き返った！")
	return true
}
