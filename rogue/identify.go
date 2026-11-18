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
	if it.Def.Category == CategoryMoney {
		return fmt.Sprintf("%dG", it.Amount)
	}
	if g.identified[it.Def.ID] {
		return it.Def.Name
	}
	return g.fakeNames[it.Def.ID]
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
	if !g.identified[it.Def.ID] {
		return true
	}
	return !it.Known && (it.upgradable() || it.Def.Category == CategoryStaff)
}

// askIdentify asks which item to identify (a scroll), and says whether
// there is any.
func (g *Game) askIdentify() bool {
	p := &Prompt{Title: "何を識別する？", kind: promptIdentify}
	for i, it := range g.Inventory {
		if g.unknown(it) {
			p.Choices = append(p.Choices, g.Entry(it))
			p.items = append(p.items, i)
		}
	}
	if foot := g.foot(); foot != nil && g.unknown(foot) {
		p.Choices = append(p.Choices, g.footEntry(foot))
		p.items = append(p.items, footIndex)
	}
	if len(p.Choices) == 0 {
		return false
	}
	g.Prompt = p
	return true
}
