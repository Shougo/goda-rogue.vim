package rogue

import (
	"fmt"
	"slices"
)

// The commands of a pot.
const (
	commandPutIn   = "入れる"
	commandTakeOut = "出す"
)

// The prompts of a pot: the item to put in it, to take out of it.
const (
	promptPutIn   = "put in"
	promptTakeOut = "take out"
)

// potLabel shows how many more items it holds, once known: "[2]".
func (g *Game) potLabel(it *Item) string {
	if !g.identified[it.Def.ID] {
		return ""
	}
	if !it.Known {
		return "[?]"
	}
	return fmt.Sprintf("[%d]", it.Charges-len(it.Contents))
}

// askPutIn asks which item to put in the pot it (at index i).
func (g *Game) askPutIn(i int) bool {
	pot := g.Inventory[i]
	if len(pot.Contents) >= pot.Charges {
		g.learn(pot)
		g.say(g.Name(pot) + "はいっぱいだ。")
		return false
	}
	p := &Prompt{Title: g.Name(pot) + "に入れる", kind: promptPutIn, item: i}
	for j, it := range g.Inventory {
		if it.Def.Category != CategoryPot {
			p.Choices = append(p.Choices, g.Label(it))
			p.items = append(p.items, j)
		}
	}
	if len(p.Choices) == 0 {
		g.say("入れるものがない。")
		return false
	}
	g.Prompt = p
	return false
}

// putIn puts it in pot: it is kept, or merged with what is in a pot of
// synthesis.
func (g *Game) putIn(pot, it *Item) {
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	it.Equipped, it.used = false, false
	g.say(g.Name(it) + "を" + g.Name(pot) + "に入れた。")
	// Putting things in tells what the pot is, and how much it holds.
	g.learn(pot)
	if pot.Def.Effect == EffectSynthesis {
		for _, base := range pot.Contents {
			if g.merge(base, it) {
				return
			}
		}
	}
	pot.Contents = append(pot.Contents, it)
}

// merge merges it into base, in a pot of synthesis, and says whether it
// did: a weapon or a shield gets the seals (as many as its slots hold)
// and the upgrade value of another of its category; a bracelet is mended
// by the same bracelet.
func (g *Game) merge(base, it *Item) bool {
	c := base.Def.Category
	switch {
	case (c == CategoryWeapon || c == CategoryShield) && it.Def.Category == c:
		base.Upgrade += it.Upgrade
		for _, s := range it.Seals {
			if len(base.Seals) < base.Def.Slots && !slices.Contains(base.Seals, s) {
				base.Seals = append(base.Seals, s)
			}
		}
		base.Known = base.Known && it.Known
		g.say(g.Name(base) + "と" + g.Name(it) + "が合成された！")
		return true
	case c == CategoryBracelet && it.Def == base.Def:
		base.Cracks = 0
		g.say(g.Name(base) + "のひびが直った！")
		return true
	}
	return false
}

// askTakeOut asks which item to take out of the pot it (at index i).
func (g *Game) askTakeOut(i int) bool {
	pot := g.Inventory[i]
	if len(pot.Contents) == 0 {
		g.say(g.Name(pot) + "には何も入っていない。")
		return false
	}
	p := &Prompt{Title: g.Name(pot) + "から出す", kind: promptTakeOut, item: i}
	for _, it := range pot.Contents {
		p.Choices = append(p.Choices, g.Label(it))
	}
	g.Prompt = p
	return false
}

// takeOut takes the item at index n out of pot, into the inventory.
func (g *Game) takeOut(pot *Item, n int) bool {
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		g.say("持ち物がいっぱいで、出せない。")
		return false
	}
	it := pot.Contents[n]
	pot.Contents = slices.Delete(pot.Contents, n, n+1)
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	g.say(g.Name(it) + "を" + g.Name(pot) + "から出した。")
	return true
}

// smash breaks a thrown pot on p: what was in it falls around.
func (g *Game) smash(pot *Item, p Point) {
	g.say(g.Name(pot) + "は割れた。")
	for _, it := range pot.Contents {
		g.land(it, p)
	}
	pot.Contents = nil
}
