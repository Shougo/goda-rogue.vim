package rogue

import (
	"strings"
	"unicode/utf8"
)

// Naming. The player may name a kind it has not identified (名前をつける, as
// in Shiren): its items show the name after their fake name, 橙の草「混乱?」,
// in the inventory, on the floor and in the catalog, until the kind is
// identified. It costs no turn.

// maxName is the most characters of a name.
const maxName = 16

// nameable says whether the player may name the kind of it: one with a
// fake name, not identified yet.
func (g *Game) nameable(it *Item) bool {
	return it.Def.Category != CategoryMoney && g.fakeNames[it.Def.ID] != "" && !g.identified[it.Def.ID]
}

// askName asks the name of the kind of it (the text of the action
// "name"; an empty one takes the name away).
func (g *Game) askName(it *Item) {
	g.naming = it.Def.ID
	g.Prompt = &Prompt{Title: g.fakeNames[it.Def.ID] + "の名前", Input: true, Default: g.names[it.Def.ID], kind: promptName}
}

// nameKind names the kind the player is naming (askName) name: at most
// maxName characters, none to take its name away.
func (g *Game) nameKind(name string) {
	id := g.naming
	g.naming, g.Prompt = "", nil
	if id == "" || g.identified[id] {
		return
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxName {
		name = string([]rune(name)[:maxName])
	}
	if name == "" {
		delete(g.names, id)
		g.say(g.fakeNames[id] + "の名前を消した。")
		return
	}
	g.names[id] = name
	g.say(g.fakeNames[id] + "を「" + name + "」と名付けた。")
}

// fakeLabel is the fake name of the kind id, with its name if the player
// named it.
func (g *Game) fakeLabel(id string) string {
	if name := g.names[id]; name != "" {
		return g.fakeNames[id] + "「" + name + "」"
	}
	return g.fakeNames[id]
}
