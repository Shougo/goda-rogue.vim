package rogue

import (
	"math"
	"slices"
)

// The bots' healing. How much a healing grass is worth depends on how
// strong the player is: 50 HP heal a player of 40 max HP in full, a
// third of one of 150. A bot values a healing grass by the share of its
// max HP it heals, drinks the one that fits what is missing best, and
// heals sooner the weaker it is on the floor.

// healPrice is what a full heal is worth to the bot: the price of the
// healing grass that heals most (a pot of healing heals in full each
// time).
func (b *Bot) healPrice(g *Game) float64 {
	best := 0
	for _, def := range g.Dungeon.Items {
		if def.Category == CategoryGrass && def.Effect == EffectHeal {
			best = max(best, def.Price)
		}
	}
	return float64(best)
}

// kindPrice is what an item of the kind def is worth to the bot: a
// healing grass the share of the max HP it heals of a full heal
// (healPrice); another its price.
func (b *Bot) kindPrice(g *Game, def *ItemDef) float64 {
	if def.Category == CategoryGrass && def.Effect == EffectHeal {
		return b.healPrice(g) * float64(min(def.Strength, g.MaxHP)) / float64(max(g.MaxHP, 1))
	}
	return float64(def.Price)
}

// healTurns is how many turns of the worst monster of the floor
// (worstTurn) the bot keeps HP for before it heals: fewer when no
// monster of the floor would likely beat it one on one, more when many
// would (dangerousFloorShare).
func (b *Bot) healTurns(g *Game) float64 {
	switch d := b.floorDanger(g); {
	case d == 0:
		return 1.5
	case d >= dangerousFloorShare:
		return 3
	}
	return 2
}

// aHeal is a way to heal: a grass to drink, or a pot of healing with
// what to put in it.
type aHeal struct {
	item, fuel *Item
	amount     int
	cost       float64
}

// bestHeal returns the way to heal that fits what is missing best: the
// cheapest of those that heal it all (the smallest of equal cost), or,
// with none, the one that heals most.
func (b *Bot) bestHeal(g *Game) (aHeal, bool) {
	var hs []aHeal
	for _, it := range g.Inventory {
		if !it.ForSale && b.known(g, it, EffectHeal) {
			hs = append(hs, aHeal{it, nil, it.Def.Strength, b.kindPrice(g, it.Def)})
		}
	}
	if pot, fuel, ok := b.healingPot(g); ok {
		hs = append(hs, aHeal{pot, fuel, g.MaxHP, b.healPrice(g) + b.worth(g, fuel, false)})
	}
	if len(hs) == 0 {
		return aHeal{}, false
	}
	missing := g.MaxHP - g.HP
	return slices.MinFunc(hs, func(x, y aHeal) int {
		xa, ya := x.amount >= missing, y.amount >= missing
		switch {
		case xa != ya:
			if xa {
				return -1
			}
			return 1
		case !xa:
			return y.amount - x.amount
		case x.cost != y.cost:
			if x.cost < y.cost {
				return -1
			}
			return 1
		}
		return x.amount - y.amount
	}), true
}

// healUnknown is the least share of healing grasses among the kinds an
// unknown grass may be, by their weights on the floor (floorWeight), for
// the bot to drink one to heal: less, it keeps it for a safer time
// (drinking it at full HP to learn it). (By the count of the kinds, the
// share fell as grasses were added: 2 of 18.)
const healUnknown = 0.2

// unknownHeal returns an unknown grass to drink to heal, when no healing
// is known: one whose kinds are healing grasses often enough; nil if none.
func (b *Bot) unknownHeal(g *Game) *Item {
	return carried(g, func(it *Item) bool {
		if it.Def.Category != CategoryGrass || g.identified[it.Def.ID] || spoiled(it) {
			return false
		}
		heals, all := 0, 0
		for _, def := range b.mayBe(g, it) {
			w := floorWeight(g, def)
			all += w
			if def.Effect == EffectHeal {
				heals += w
			}
		}
		return float64(heals) >= healUnknown*float64(all)
	})
}

// useHeal heals by h.
func (b *Bot) useHeal(h aHeal) (string, []string) {
	if h.fuel != nil {
		return b.putIn(h.item, h.fuel)
	}
	return b.use(h.item, commandDrink)
}

// healingPot returns a pot of healing the bot knows, with room left (or
// not known yet), and the item worth least to put in it; false with none.
func (b *Bot) healingPot(g *Game) (*Item, *Item, bool) {
	pot := carried(g, func(it *Item) bool {
		return b.known(g, it, EffectHealPot) && (!it.Known || it.Charges > len(it.Contents))
	})
	if pot == nil {
		return nil, nil, false
	}
	var fuel *Item
	low := math.Inf(1)
	for _, it := range g.Inventory {
		if it.ForSale || it.Equipped || it.Def.Category == CategoryPot {
			continue
		}
		if w := b.worth(g, it, false); w < low {
			fuel, low = it, w
		}
	}
	return pot, fuel, fuel != nil
}

// putIn plans to put fuel in pot (carried): the inventory, the pot, the
// command, fuel.
func (b *Bot) putIn(pot, fuel *Item) (string, []string) {
	b.plan = []botStep{{item: pot}, {label: commandPutIn}, {item: fuel}}
	return "inventory", nil
}

// floorWeight is the weight of the kind def among those of its category
// in the item table of the floor (at least 1: one may come of elsewhere).
func floorWeight(g *Game, def *ItemDef) int {
	d := g.Dungeon
	w := 0
	if t := d.ItemTable(d.counts(g.Depth).ItemTable); t != nil {
		for _, r := range t.Items {
			if r.ID == def.ID && on(g.Depth, r.First, r.Last) {
				w += r.Weight
			}
		}
	}
	return max(w, 1)
}
