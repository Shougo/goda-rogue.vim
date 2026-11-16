package rogue

import (
	"fmt"
	"slices"
)

// throwRange is how far a thrown item flies.
const throwRange = 10

// shoot shoots an arrow of those equipped to the front; it says whether
// it did (and the turn passes).
func (g *Game) shoot() bool {
	a := g.equipped(CategoryArrow)
	if a == nil {
		g.say("矢を装備していない。")
		return false
	}
	g.throwItem(a)
	return true
}

// throwItem throws it (one arrow of arrows) to the front of the player.
// It flies up to throwRange tiles, until it hits a monster or a wall; it
// lands where it stops, unless it was used up on the monster it hit.
func (g *Game) throwItem(it *Item) {
	thrown := it
	if it.Def.Category == CategoryArrow && it.Count > 1 {
		it.Count--
		thrown = &Item{Def: it.Def, Glyph: it.Glyph, Count: 1}
		g.say(g.Name(it) + "を撃った。")
	} else {
		g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
		it.Equipped, it.used = false, false
		if it.Def.Category == CategoryArrow {
			g.say(g.Name(it) + "を撃った。")
		} else {
			g.say(g.Name(it) + "を投げた。")
		}
	}

	d := Directions[g.Facing]
	r := g.Dungeon.Rules
	p := g.Player
	for range throwRange {
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Walkable() {
			break
		}
		p = p.Add(d)
		m := g.MonsterAt(p)
		if m == nil {
			continue
		}
		m.Dozing = false
		if !hits(g.rng, r.PlayerHitRate) {
			g.say(m.Name + "に当たらなかった。")
			continue
		}
		if g.hitBy(thrown, m) {
			return
		}
		break
	}
	if thrown.Def.Category == CategoryPot {
		g.smash(thrown, p)
		return
	}
	g.land(thrown, p)
}

// hitBy is the thrown item it hitting m; it says whether it was used up.
func (g *Game) hitBy(it *Item, m *Monster) bool {
	r := g.Dungeon.Rules
	switch it.Def.Category {
	case CategoryArrow:
		attack := playerAttack(g.PlayerLevel, g.CurrentStrength())/2 + it.Def.Strength
		g.hurt(m, damage(g.rng, attack, m.Defense, r.DefenseK))
		return true
	case CategoryGrass:
		g.say(g.Name(it) + "が" + m.Name + "に当たった。")
		switch it.Def.Effect {
		case EffectHeal:
			healed := min(it.Def.Strength, m.MaxHP-m.HP)
			m.HP += healed
			g.say(fmt.Sprintf("%sのHPが%d回復した。", m.Name, healed))
			g.identify(it)
		default:
			g.say("何も起こらなかった。")
		}
		return true
	}
	g.hurt(m, between(g.rng, 1, 2))
	if it.Def.Category == CategoryPot {
		g.smash(it, m.Pos)
		return true
	}
	return false
}

// hurt gives m n damage from the player.
func (g *Game) hurt(m *Monster, n int) {
	g.provoke(m)
	m.HP -= n
	g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
	if m.HP <= 0 {
		g.remove(m)
		g.say(m.Name + "を倒した。")
		if !m.Ally {
			g.defeated(m)
		}
	}
}

// land puts it on p, or the nearest free tile around (no item, no
// stairs); with none, it is lost.
func (g *Game) land(it *Item, p Point) {
	for r := 0; r <= 2; r++ {
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				q := p.Add(Point{x, y})
				if chebyshev(p, q) != r || !g.Level.At(q).Walkable() || q == g.Stairs {
					continue
				}
				if _, ok := g.ItemAt(q); ok {
					continue
				}
				it.Pos = q
				g.Items = append(g.Items, it)
				g.say(g.Name(it) + "が落ちた。")
				return
			}
		}
	}
	g.say(g.Name(it) + "は消えてしまった。")
}
