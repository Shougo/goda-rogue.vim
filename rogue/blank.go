package rogue

import (
	"slices"
	"strings"
)

// The scroll of 白紙 (Shougo, 2026-11-27): reading it, the player writes
// the name of a scroll it knows, typed with completion (there are too
// many to choose from a list), and reads it as that scroll. Cancelled,
// or with nothing to write, it stays blank.

// promptBlank asks for the name to write on a scroll of 白紙.
const promptBlank = "blank"

// blank says whether it is a scroll of 白紙.
func (g *Game) blank(it *Item) bool {
	return it.Def.Effect == EffectBlank
}

// writable are the names of the scrolls the player may write: the kinds
// identified, but 白紙.
func (g *Game) writable() []string {
	var names []string
	for _, d := range g.Dungeon.Items {
		if d.Category == CategoryScroll && d.Effect != EffectBlank && g.identified[d.ID] {
			names = append(names, d.Name)
		}
	}
	return names
}

// askBlank asks what to write on the scroll of 白紙 it; it says whether
// the turn passes: with nothing to write, the scroll is known for what
// it is, and the turn spent.
func (g *Game) askBlank(it *Item) bool {
	names := g.writable()
	if len(names) == 0 {
		g.identify(it)
		g.say("書ける巻物を知らない。")
		return true
	}
	g.blanking = it
	g.Prompt = &Prompt{Title: "何の巻物を書く？", Input: true, Complete: names, kind: promptBlank}
	return false
}

// writeBlank writes name on the scroll of 白紙 being written, and reads
// it: it says whether it did (the turn passes). A name it does not know
// asks again.
func (g *Game) writeBlank(name string) bool {
	it := g.blanking
	g.blanking = nil
	if it == nil || !slices.Contains(g.Inventory, it) && it != g.foot() {
		return false
	}
	name = strings.TrimSpace(name)
	var def *ItemDef
	for _, d := range g.Dungeon.Items {
		if d.Category == CategoryScroll && d.Effect != EffectBlank && g.identified[d.ID] &&
			(d.Name == name || strings.TrimSuffix(d.Name, "の巻物") == name) {
			def = d
			break
		}
	}
	if def == nil {
		g.say("「" + name + "」という巻物は知らない。")
		g.askBlank(it)
		return false
	}
	g.identify(it)
	g.say(g.Name(it) + "に「" + def.Name + "」と書いた。")
	g.discard(it)
	// (The written scroll keeps the blessing or the curse of the blank.)
	g.useItem(&Item{Def: def, Glyph: def.Glyph, Blessed: it.Blessed, Cursed: it.Cursed, CurseTold: true})
	return true
}
