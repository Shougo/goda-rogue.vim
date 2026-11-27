package rogue

import "fmt"

// What a monster that curses (AbilityCurse) may curse.
const (
	curseArms     = 0 // the weapon or the shield equipped
	curseEquipped = 1 // those, or the bracelet equipped
	curseAny      = 2 // any item carried, equipped or not
)

// curser is the monster m next to the player cursing what it carries, by
// its ability a (AbilityCurse), instead of a blow: so many items, at
// random among those the ability reaches that are not cursed yet. A
// blessing takes the curse in the place of its item. It says whether it
// did something: with nothing to curse, it attacks.
func (g *Game) curser(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	done := false
	for range a.Values[2] {
		var can []*Item
		for _, it := range g.Inventory {
			if it.Cursed || !it.cursable() || it.ForSale {
				continue
			}
			switch {
			case a.Values[1] == curseAny,
				it.Equipped && it.upgradable(),
				it.Equipped && a.Values[1] == curseEquipped && it.Def.Category == CategoryBracelet:
				can = append(can, it)
			}
		}
		if len(can) == 0 {
			break
		}
		if !done {
			g.say(m.Name + "は呪いをかけた！")
			g.event("cursed %s", m.Def.ID)
		}
		done = true
		g.curseShown(can[g.rng.IntN(len(can))])
	}
	return done
}

// curseShown is a curse the player sees come on it: it knows from then
// on whether the item is cursed.
func (g *Game) curseShown(it *Item) {
	name := g.Name(it)
	if g.curse(it) {
		g.say(name + "が呪われてしまった！")
	}
	it.CurseTold = true
}

// curseTrap is the trap of curses: n of the items carried that are no
// equipment, at random, are cursed.
func (g *Game) curseTrap(n int) {
	done := false
	for range max(n, 1) {
		var can []*Item
		for _, it := range g.Inventory {
			if !it.Cursed && it.cursable() && !it.worn() && !it.ForSale {
				can = append(can, it)
			}
		}
		if len(can) == 0 {
			break
		}
		done = true
		g.curseShown(can[g.rng.IntN(len(can))])
	}
	if !done {
		g.say("何も起こらなかった。")
	}
}

// exorcise is a scroll of おはらい read with monsters that curse around
// (enemiesAround): they are no more.
func (g *Game) exorcise() bool {
	gone := false
	for _, m := range g.enemiesAround() {
		if _, ok := m.ability(AbilityCurse); ok {
			g.say(m.Name + "は消え去った。")
			g.kill(m, nil, true, true)
			gone = true
		}
	}
	return gone
}

// curseText describes the ability.
func curseText(v []int) string {
	what := "装備中の武器か盾"
	switch v[1] {
	case curseEquipped:
		what = "装備中の武器か盾か腕輪"
	case curseAny:
		what = "持ち物（装備中のものも）"
	}
	return fmt.Sprintf("隣にいると%d%%で%sを%d個呪う。おはらいの巻物を読むと消え去る", v[0], what, v[2])
}
