package rogue

import "slices"

// Luck. Some scrolls do more now and then (rules.txt ラッキー, one in so
// many), and always when blessed, as in Shiren 6:
//
//	識別       identifies everything carried, not one item
//	おはらい   takes the curses of everything carried away, not one
//	武器強化, 盾強化  raise the upgrade value by luckyUpgrade more
//
// Luck does not show when it would change nothing (nothing to identify).

// sayLucky is said when a scroll does more.
const sayLucky = "ラッキー！"

// luckyUpgrade is how much more a lucky scroll of upgrade gives.
const luckyUpgrade = 2

// luckyEffects are the effects that luck makes better.
var luckyEffects = []Effect{EffectIdentify, EffectUncurse, EffectUpWeapon, EffectUpShield}

// lucky says whether the scroll it, read now, does more: always when it
// was blessed, else by the chance of the dungeon.
func (g *Game) lucky(it *Item, blessed bool) bool {
	if it.Def.Category != CategoryScroll || !slices.Contains(luckyEffects, it.Def.Effect) {
		return false
	}
	in := g.Dungeon.Rules.Lucky
	// (Rolled for a blessed scroll too: the same run with or without
	// its blessing.)
	return in > 0 && g.rng.IntN(in) == 0 || blessed
}

// identifyAll identifies everything carried that has something unknown
// (but a mimic, which stays what it looks like), and says whether there
// was any.
func (g *Game) identifyAll() bool {
	found := false
	for _, it := range g.Inventory {
		if it.Mimic != nil || !g.unknown(it) {
			continue
		}
		if !found {
			g.say(sayLucky)
		}
		found = true
		g.learn(it)
	}
	if found {
		g.say("持ち物がすべて識別された。")
	}
	return found
}

// uncurseOne takes the curse of it away, and of what is in it (a pot);
// the player knows from then on that it is not cursed.
func (g *Game) uncurseOne(it *Item) {
	found := it.Cursed
	it.Cursed, it.CurseTold = false, true
	for _, in := range it.Contents {
		found = found || in.Cursed
		in.Cursed = false
	}
	if found {
		g.say(g.Name(it) + "の呪いが解けた。")
	} else {
		g.say(g.Name(it) + "は呪われていなかった。")
	}
}
