package rogue

import (
	"fmt"
	"slices"
)

// The things a thief (AbilitySteal) takes.
const (
	stealGold  = 0
	stealItems = 1
)

// thieve is the thief m next to the player taking from it, by its ability
// a, if it may: money (half the purse), or items (not equipped ones,
// unless it takes those too). Then it warps away and flees. It says
// whether it stole: with nothing to take, it attacks instead.
func (g *Game) thieve(m *Monster, a Ability) bool {
	if m.Fled || g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	switch a.Values[1] {
	case stealGold:
		if g.Gold == 0 {
			return false
		}
		n := (g.Gold + 1) / 2
		g.Gold -= n
		m.stolen = append(m.stolen, g.money(n))
		g.say(fmt.Sprintf("%sに%dG を盗まれた！", m.Name, n))
	default:
		for range a.Values[2] {
			var can []*Item
			for _, it := range g.Inventory {
				if !it.Equipped || a.Values[3] == 1 && !it.Cursed {
					can = append(can, it)
				}
			}
			if len(can) == 0 {
				break
			}
			it := can[g.rng.IntN(len(can))]
			if it.Equipped {
				g.unequip(it)
			}
			g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
			it.used = false
			m.stolen = append(m.stolen, it)
			g.say(g.Name(it) + "を" + m.Name + "に盗まれた！")
		}
		if len(m.stolen) == 0 {
			return false
		}
	}
	g.event("stolen %s", m.Def.ID)
	m.Fled, m.hasTarget = true, false
	if p, ok := g.freeTile(); ok {
		m.Pos = p
		g.say(m.Name + "は消えた。")
	}
	return true
}

// money returns money of the amount n, as an item.
func (g *Game) money(n int) *Item {
	for _, def := range g.Dungeon.Items {
		if def.Category == CategoryMoney {
			return &Item{Def: def, Glyph: def.Glyph, Amount: n}
		}
	}
	return nil
}

// flee is a thief that stole keeping away from the player: a step that
// takes it farther, if there is one. It says whether it stepped.
func (g *Game) flee(m *Monster) bool {
	best, far := m.Pos, chebyshev(m.Pos, g.Player)
	for _, d := range around {
		p := m.Pos.Add(d)
		if !g.Level.At(p).Walkable() || g.occupied(p) || p == g.Player || !g.clearFrom(m.Pos, d) {
			continue
		}
		if n := chebyshev(p, g.Player); n > far {
			best, far = p, n
		}
	}
	if best == m.Pos {
		return false
	}
	m.Pos = best
	return true
}

// dropStolen is the thief m, defeated, dropping what it stole.
func (g *Game) dropStolen(m *Monster) {
	for _, it := range m.stolen {
		if it != nil {
			g.land(it, m.Pos)
		}
	}
	m.stolen = nil
}
