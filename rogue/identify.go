package rogue

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// nameItems gives the item kinds of the categories with fake names one
// each, for this run (from its seed, with a random source of its own),
// and identifies the others.
func (g *Game) nameItems() {
	d := g.Dungeon
	rng := rand.New(rand.NewPCG(seedHash(d, g.Seed), math.MaxUint64-1))
	g.fakeNames = map[string]string{}
	for _, c := range categories {
		names := append([]string(nil), d.FakeNames[c]...)
		rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
		for _, it := range d.Items {
			if it.Category != c {
				continue
			}
			if len(names) == 0 {
				g.identified[it.ID] = true
				continue
			}
			g.fakeNames[it.ID], names = names[0], names[1:]
		}
	}
}

// Name is the name of the item as the player knows it: its fake name until
// its kind is identified.
func (g *Game) Name(it *Item) string {
	if it.Unmasked {
		return it.Mimic.Name
	}
	if it.Def.Category == CategoryMoney {
		return fmt.Sprintf("%dG", it.Amount)
	}
	if g.identified[it.Def.ID] {
		return it.Def.Name
	}
	return g.fakeLabel(it.Def.ID)
}

// identify makes the kind of it known, and says what it was.
func (g *Game) identify(it *Item) {
	if g.identified[it.Def.ID] {
		return
	}
	fake := g.Name(it)
	g.identified[it.Def.ID] = true
	g.event("identify %s", it.Def.ID)
	g.say(fake + "は" + it.Def.Name + "だった。")
}

// learn makes it known: its kind, and its upgrade value or charges.
func (g *Game) learn(it *Item) {
	g.identify(it)
	it.Known = true
}

// unknown says whether there is something to identify about it.
func (g *Game) unknown(it *Item) bool {
	if it.Unmasked {
		return false
	}
	if !g.identified[it.Def.ID] {
		return true
	}
	if it.Known {
		return false
	}
	// Whether it is cursed, where such items are, unless the player knows.
	return it.upgradable() || it.Def.Category == CategoryStaff || g.curses(it) && !it.curseTold()
}

// curses says whether an item like it may be cursed or blessed in the
// dungeon.
func (g *Game) curses(it *Item) bool {
	r := g.Dungeon.Rules
	switch {
	case !it.cursable():
		return false
	case r.Bless > 0:
		return true
	case it.upgradable():
		return r.CurseGear > 0
	case it.Def.Category == CategoryBracelet:
		return r.CurseBracelet > 0
	case it.Def.Category == CategoryFood:
		return r.CurseFood > 0
	}
	return r.CurseItem > 0
}

// askIdentify asks which item to identify (a scroll): any carried, and
// the one under the player (one known already: nothing happens). It says
// whether there is any.
func (g *Game) askIdentify() bool {
	p := &Prompt{Title: "何を識別する？", kind: promptIdentify}
	for i, it := range g.Inventory {
		p.Choices = append(p.Choices, g.Entry(it))
		p.items = append(p.items, i)
		p.targets = append(p.targets, it)
	}
	if foot := g.foot(); foot != nil {
		p.Choices = append(p.Choices, g.footEntry(foot))
		p.items = append(p.items, footIndex)
		p.targets = append(p.targets, foot)
	}
	if len(p.Choices) == 0 {
		return false
	}
	g.Prompt = p
	return true
}
