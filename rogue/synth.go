package rogue

import "slices"

// The monster of synthesis (AbilitySynthesis). What the player throws at
// it, it swallows, as many items as its ability says; in its belly the
// items merge as in a pot of synthesis (a weapon or a shield with another
// of its kind: seals and upgrade values; a bracelet mended by the same
// bracelet), and a material gives its seal to a weapon or a shield
// swallowed before it (異種合成), which no pot does. What does not merge
// stays as it is. Defeated, it drops all of it. Full, or with its
// abilities sealed, it is hit by a thrown item as any monster. The higher
// ranks also suck a weapon or a shield of the player in, while they have
// room.

// room says how many more items m can swallow: 0 without the ability, or
// with its abilities sealed.
func (m *Monster) room() int {
	a, ok := m.ability(AbilitySynthesis)
	if !ok || m.Sealed {
		return 0
	}
	return max(a.Values[0]-m.swallowed, 0)
}

// swallow is m swallowing the thrown item it, if it has room; it says
// whether it did.
func (g *Game) swallow(m *Monster, it *Item) bool {
	if m.room() == 0 {
		return false
	}
	g.say(m.Name + "は" + g.Name(it) + "を飲み込んだ。")
	g.digest(m, it)
	return true
}

// digest puts it in the belly of m, merged into what is there if it can
// be.
func (g *Game) digest(m *Monster, it *Item) {
	m.swallowed++
	g.event("swallowed %s", it.Def.ID)
	for _, base := range m.belly {
		if g.merge(base, it, true) {
			return
		}
	}
	m.belly = append(m.belly, it)
}

// suck is m, next to the player, sucking a weapon or a shield the player
// carries in by the chance of its ability, an equipped one too, a cursed
// one too (not one in a pot); it says whether it did, instead of a blow.
func (g *Game) suck(m *Monster) bool {
	a, ok := m.ability(AbilitySynthesis)
	if !ok || a.Values[1] == 0 || m.room() == 0 {
		return false
	}
	var can []*Item
	for _, it := range g.Inventory {
		if it.upgradable() && !it.ForSale {
			can = append(can, it)
		}
	}
	if len(can) == 0 || g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	it := can[g.rng.IntN(len(can))]
	name := g.Name(it)
	it.Equipped, it.used = false, false
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	g.say(m.Name + "に" + name + "を吸い込まれた！")
	g.digest(m, it)
	return true
}

// dropBelly is m, defeated, dropping what it holds.
func (g *Game) dropBelly(m *Monster) {
	for _, it := range m.belly {
		g.land(it, m.Pos)
	}
	m.belly = nil
}
