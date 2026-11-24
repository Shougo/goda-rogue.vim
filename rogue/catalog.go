package rogue

import (
	"fmt"
	"slices"
	"strings"
)

// The catalog of the kinds of items (識別帳): for each category with
// fake names, the kinds identified with the fake name they had, the
// fake names carried and not identified (with their price and the kinds
// of that price once seen in a shop), and the kinds not identified yet.
// The bots reason on the same: the kinds an unknown item may be.

// meet is the player carrying what it carries now: it has met their
// fake names, and in a shop that would buy, it sees their prices.
func (g *Game) meet() {
	shop := g.shopOpen()
	for _, it := range g.Inventory {
		if g.fakeNames[it.Def.ID] == "" {
			continue
		}
		g.met[it.Def.ID] = true
		if shop && !it.ForSale {
			g.priced[it.Def.ID] = true
		}
	}
}

// showCatalog shows the catalog (any answer closes it). It costs no turn.
func (g *Game) showCatalog() {
	var lines []string
	for _, c := range categories {
		var known, met, unknown []string
		for _, def := range g.Dungeon.Items {
			fake := g.fakeNames[def.ID]
			if def.Category != c || fake == "" {
				continue
			}
			switch {
			case g.identified[def.ID]:
				if g.met[def.ID] {
					known = append(known, def.Name+" ← "+fake)
				} else {
					known = append(known, def.Name)
				}
			default:
				unknown = append(unknown, fmt.Sprintf("%s %dG", def.Name, def.Price))
				if g.met[def.ID] || g.names[def.ID] != "" {
					met = append(met, g.metLine(def, g.fakeLabel(def.ID)))
				}
			}
		}
		if len(known)+len(unknown) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("■ %s（識別 %d / %d）", c, len(known), len(known)+len(unknown)))
		for _, s := range known {
			lines = append(lines, "  "+s)
		}
		for _, s := range met {
			lines = append(lines, "  "+s)
		}
		if len(unknown) > 0 {
			lines = append(lines, "  未識別: "+strings.Join(unknown, "、"))
		}
	}
	if len(lines) == 0 {
		g.say("識別する道具はない。")
		return
	}
	g.Prompt = &Prompt{Title: "識別帳", Choices: lines, kind: promptInfo}
}

// metLine is the line of a fake name met and not identified: with its
// price and the kinds of that price, once seen in a shop.
func (g *Game) metLine(def *ItemDef, fake string) string {
	if !g.priced[def.ID] {
		return fake + "?"
	}
	var kinds []string
	for _, k := range g.Dungeon.Items {
		if k.Category == def.Category && !g.identified[k.ID] && k.Price == def.Price {
			kinds = append(kinds, k.Name)
		}
	}
	slices.Sort(kinds)
	return fmt.Sprintf("%s? %dG: %s", fake, def.Price, strings.Join(kinds, "、"))
}
