package rogue

import "math/rand/v2"

// Curses. A weapon, a shield or a bracelet found in the dungeon may be
// cursed (rules.txt 武具の呪い, 腕輪の呪い): once equipped it does not come off, so nothing
// else of its kind can be equipped, and it cannot be put down, swapped,
// thrown nor put in a pot. A cursed weapon or shield never has an upgrade
// value above 0. The player knows the curse by equipping the item or
// identifying it; a scroll (おはらい) takes the curses of everything
// carried away, the scrolls that upgrade a weapon or a shield those of
// what they upgrade. Synthesis carries a curse over to the base. Shops do
// not sell cursed goods.

// cursable says whether it can be cursed.
func (it *Item) cursable() bool {
	return it.upgradable() || it.Def.Category == CategoryBracelet
}

// curseKnown says whether the player knows that it is cursed.
func (it *Item) curseKnown() bool {
	return it.Cursed && (it.Known || it.Equipped)
}

// rollCurse curses the new item it, by the chance of the dungeon (no roll
// without one: the floors of a dungeon without curses stay as they were).
func (g *Game) rollCurse(rng *rand.Rand, it *Item) {
	if !it.cursable() {
		return
	}
	in := g.Dungeon.Rules.CurseGear
	if it.Def.Category == CategoryBracelet {
		in = g.Dungeon.Rules.CurseBracelet
	}
	if in == 0 || rng.IntN(in) != 0 {
		return
	}
	it.Cursed = true
	if it.Upgrade > 0 {
		it.Upgrade = -it.Upgrade
	}
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
