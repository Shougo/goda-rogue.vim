package rogue

import "math/rand/v2"

// Curses. An item of the dungeon, but arrows and money, may be cursed
// (rules.txt 武具の呪い, 腕輪の呪い, 道具の呪い, 食料の呪い).
//
// A cursed weapon, shield or bracelet does not come off once equipped, so
// nothing else of its kind can be equipped, and it cannot be put down,
// swapped, thrown nor put in a pot. A cursed weapon or shield never has
// an upgrade value above 0.
//
// Another cursed item cannot be used: drinking, reading, waving or eating
// it, or putting something in a cursed pot, takes the turn and does
// nothing (a staff keeps its charges). Thrown, it does not have its
// effect. It can be put down, thrown, sold and put in a pot as any other.
//
// The player knows the curse by equipping the item, trying to use it,
// identifying it, or by its price in a shop (curseRate); a scroll
// (おはらい) takes the curse of an item carried away (of everything
// carried when lucky, lucky.go), the scrolls that upgrade a weapon or a
// shield those of what they upgrade.
// Synthesis carries a curse over to the base.

// curseRate is the price of a cursed item, a percent of its price (as
// Shiren 6).
const curseRate = 87

// worn says whether it is a weapon, a shield or a bracelet: what a curse
// keeps on the player; a curse stops the use of another item.
func (it *Item) worn() bool {
	return it.upgradable() || it.Def.Category == CategoryBracelet
}

// cursable says whether it can be cursed.
func (it *Item) cursable() bool {
	return it.Def.Category != CategoryArrow && it.Def.Category != CategoryMoney
}

// curseTold says whether the player knows if it is cursed: the goods of
// a shop show it with their price.
func (it *Item) curseTold() bool {
	return it.Known || it.Equipped || it.CurseTold || it.ForSale
}

// curseKnown says whether the player knows that it is cursed.
func (it *Item) curseKnown() bool {
	return it.Cursed && it.curseTold()
}

// curseStops is the player trying to use the cursed item it (not
// equipment), and says whether the curse stopped it: nothing happens, and
// the player knows the curse from then on.
func (g *Game) curseStops(it *Item) bool {
	if !it.Cursed || it.worn() {
		return false
	}
	it.CurseTold = true
	g.say(g.Name(it) + "は呪われていて使えない！")
	return true
}

// rollCurse curses the new item it, by the chance of the dungeon (no roll
// without one: the floors of a dungeon without curses stay as they were).
func (g *Game) rollCurse(rng *rand.Rand, it *Item) {
	if !it.cursable() {
		return
	}
	r := g.Dungeon.Rules
	in := r.CurseItem
	switch {
	case it.upgradable():
		in = r.CurseGear
	case it.Def.Category == CategoryBracelet:
		in = r.CurseBracelet
	case it.Def.Category == CategoryFood:
		in = r.CurseFood
	}
	if in == 0 || rng.IntN(in) != 0 {
		return
	}
	it.Cursed = true
	if it.Upgrade > 0 {
		it.Upgrade = -it.Upgrade
	}
}

// curse is a curse coming on it (not at its birth): it takes the
// blessing of a blessed item away instead; it says whether it is cursed
// now.
func (g *Game) curse(it *Item) bool {
	if !it.cursable() || it.Cursed || g.shielded(it) {
		return false
	}
	it.Cursed = true
	return true
}

// curseBlocks says whether a curse stops the command c on the carried
// item it, and says so: it is cursed and equipped, or it would take the
// place of a cursed one.
func (g *Game) curseBlocks(it *Item, c string) bool {
	switch c {
	case commandUnequip, commandDrop, commandSwap, commandPutIn, commandThrow:
		if it.Equipped && it.Cursed {
			g.say(g.Name(it) + "は呪われていて外せない！")
			return true
		}
	case commandEquip:
		if old := g.equipped(it.Def.Category); old != nil && old.Cursed {
			g.say(g.Name(old) + "は呪われていて外せない！")
			return true
		}
	}
	return false
}

// uncurse takes the curses of everything carried away (in pots too).
func (g *Game) uncurse() {
	found := false
	var clear func(its []*Item)
	clear = func(its []*Item) {
		for _, it := range its {
			found = found || it.Cursed
			it.Cursed = false
			clear(it.Contents)
		}
	}
	clear(g.Inventory)
	if found {
		g.say("持ち物の呪いが解けた。")
	} else {
		g.say("呪われたものはなかった。")
	}
}
