package rogue

import "fmt"

// The scrolls of 印増大 and 印消し (Shougo, 2026-11-24): the first adds a
// slot for seals to a weapon or a shield chosen, up to maxSlots; the
// second takes one seal chosen away (not the ability an item is born
// with: it is no seal).

// maxSlots is the most slots for seals a weapon or a shield can have.
const maxSlots = 10

// slots is how many seals it can have.
func (it *Item) slots() int {
	return it.Def.Slots + it.MoreSlots
}

// addSlot adds a slot to it.
func (g *Game) addSlot(it *Item) {
	it.MoreSlots++
	g.say(fmt.Sprintf("%sの印の枠が%dになった。", g.Name(it), it.slots()))
}

// askSeal asks which seal of it (a weapon or shield, chosen for the
// scroll of 印消し) to take away; the prompt keeps the scroll, to be
// identified once a seal is gone.
func (g *Game) askSeal(it *Item, scroll *Item) {
	p := &Prompt{Title: "どの印を消す？", kind: promptErase, scroll: scroll}
	for i, s := range it.Seals {
		name := s
		if sd := g.Dungeon.SealOn(s, it.Def.Category); sd != nil {
			name = sd.Name
		}
		p.Choices = append(p.Choices, fmt.Sprintf("【%s】%s", s, name))
		p.items = append(p.items, i)
		p.targets = append(p.targets, it)
	}
	g.Prompt = p
}

// eraseSeal takes the seal at i away from it.
func (g *Game) eraseSeal(it *Item, i int) {
	if i < 0 || i >= len(it.Seals) {
		return
	}
	s := it.Seals[i]
	it.Seals = append(it.Seals[:i:i], it.Seals[i+1:]...)
	g.say(fmt.Sprintf("%sの【%s】が消えた。", g.Name(it), s))
}
