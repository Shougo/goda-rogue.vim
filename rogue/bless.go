package rogue

import "math/rand/v2"

// Blessings. An item of the dungeon that can be cursed may be blessed
// instead (rules.txt 祝福), never both: a curse on a blessed item, or a
// blessing on a cursed one, leaves it as any other.
//
// A blessing takes a bad effect on its item in its place, once: rust,
// rot, a curse. It does not stop what takes the item away (a thief, a
// monster of synthesis).
//
// A blessed grass, scroll or food is not used up: used, or thrown at a
// monster, it has its effect and loses its blessing. A blessed staff
// keeps its charges when waved, and loses its blessing half of the time
// (blessedStaff). A weapon, a shield, a bracelet or a pot only has the
// protection.
//
// The player knows a blessing as it knows a curse (curseTold). A shop
// sells and buys a blessed item at twice its price (blessRate).

// blessRate is the price of a blessed item, a percent of its price (as
// Shiren 6).
const blessRate = 200

// blessedStaff is the chance, a percent, that a blessed staff loses its
// blessing when waved.
const blessedStaff = 50

// blessKnown says whether the player knows that it is blessed.
func (it *Item) blessKnown() bool {
	return it.Blessed && it.curseTold()
}

// rollBless blesses the new item it, by the chance of the dungeon (no
// roll without one).
func (g *Game) rollBless(rng *rand.Rand, it *Item) {
	in := g.Dungeon.Rules.Bless
	if !it.cursable() || in == 0 {
		return
	}
	// (Rolled for a cursed item too: the same floors with or without
	// its curse.)
	if rng.IntN(in) == 0 && !it.Cursed {
		it.Blessed = true
	}
}

// shielded is a bad effect coming on it, and says whether its blessing
// took it: the blessing is gone, and the player knows.
func (g *Game) shielded(it *Item) bool {
	if !it.Blessed {
		return false
	}
	it.Blessed, it.CurseTold = false, true
	g.say(g.Name(it) + "の祝福が身代わりになった。")
	return true
}

// kept is the player using the grass, scroll or food it up, and says
// whether a blessing kept it: the blessing is gone instead.
func (g *Game) kept(it *Item) bool {
	if !it.Blessed {
		return false
	}
	it.Blessed, it.CurseTold = false, true
	return true
}
