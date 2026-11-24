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

// askSlot asks which weapon or shield carried gets one more slot; false
// with none that can.
func (g *Game) askSlot() bool {
	p := &Prompt{Title: "どれの印の枠を増やす？", kind: promptSlot}
	for i, it := range g.Inventory {
		if it.upgradable() && !it.ForSale && it.slots() < maxSlots {
			p.Choices = append(p.Choices, g.Entry(it))
			p.items = append(p.items, i)
			p.targets = append(p.targets, it)
		}
	}
	if len(p.Choices) == 0 {
		return false
	}
	g.Prompt = p
	return true
}

// addSlot adds a slot to it.
func (g *Game) addSlot(it *Item) {
	it.MoreSlots++
	g.say(fmt.Sprintf("%sの印の枠が%dになった。", g.Name(it), it.slots()))
}

// askErase asks which seal of the weapons and shields carried to take
// away, one choice a seal; false with none.
func (g *Game) askErase() bool {
	p := &Prompt{Title: "どの印を消す？", kind: promptErase}
	for _, it := range g.Inventory {
		if !it.upgradable() || it.ForSale {
			continue
		}
		for i, s := range it.Seals {
			name := s
			if sd := g.Dungeon.SealOn(s, it.Def.Category); sd != nil {
				name = sd.Name
			}
			p.Choices = append(p.Choices, fmt.Sprintf("%s：【%s】%s", g.Name(it), s, name))
			p.items = append(p.items, i)
			p.targets = append(p.targets, it)
		}
	}
	if len(p.Choices) == 0 {
		return false
	}
	g.Prompt = p
	return true
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
