package rogue

import (
	"fmt"
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
		g.discard(it)
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
	// With a bracelet of 遠投 it flies through the monsters it hits, on to
	// the next (what it does, it does to each); used up, it is gone at
	// the end.
	pierce, usedUp := g.wears(EffectPierce) && thrown.Def.Category != CategoryPot, false
	for range throwRange {
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Open() {
			break
		}
		p = p.Add(d)
		m := g.MonsterAt(p)
		if m == nil {
			continue
		}
		m.Dozing = false
		if g.wears(EffectClumsy) {
			g.clumsy(thrown, m)
			return
		}
		if !hits(g.rng, r.PlayerThrowRate) {
			g.say(m.Name + "に当たらなかった。")
			continue
		}
		if g.reflectThrown(m, thrown) || g.burn(m, thrown) || g.swallow(m, thrown) || g.feed(m, thrown) {
			return
		}
		used := g.hitBy(thrown, m)
		if pierce {
			g.use(EffectPierce)
			usedUp = usedUp || used
			continue
		}
		if used {
			return
		}
		break
	}
	if usedUp {
		return
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
		n := damage(g.rng, attack, m.Defense, r.DefenseK)
		if g.PowerUp > 0 {
			// (Arrows too, not other things thrown: Shougo, 2026-11-27.)
			n = n * powerUpRate / 100
		}
		g.hurt(m, n)
		return true
	case CategoryGrass:
		if it.Cursed {
			// No effect: it hurts a little, as any other item.
			break
		}
		g.say(g.Name(it) + "が" + m.Name + "に当たった。")
		switch it.Def.Effect {
		case EffectHeal:
			healed := min(it.Def.Strength, m.MaxHP-m.HP)
			m.HP += healed
			g.say(fmt.Sprintf("%sのHPが%d回復した。", m.Name, healed))
			g.identify(it)
		case EffectMaxHP:
			m.MaxHP += it.Def.Strength
			m.HP += it.Def.Strength
			g.say(m.Name + "の最大HPが上がった。")
			g.identify(it)
		default:
			e := MagicEffect(it.Def.Effect)
			if !castable(CategoryGrass, e) {
				g.say("何も起こらなかった。")
				break
			}
			g.provoke(m)
			g.magicOn(m, e, Directions[g.Facing])
			g.identify(it)
		}
		// A blessed grass is not used up: it falls by the monster.
		if g.kept(it) {
			g.say("祝福の力で" + g.Name(it) + "はなくならなかった。")
			return false
		}
		return true
	}
	if it.Def.Category == CategoryMoney {
		// Its amount makes the damage; defense does not lower it.
		g.hurt(m, max(it.Amount/r.GoldThrow, 1))
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
	n = g.weakening(n)
	g.provoke(m)
	m.HP -= n
	// (A blow frees it of かなしばり.)
	m.Bound = 0
	g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
	if m.HP > 0 {
		g.wounded(m)
	}
	if m.HP <= 0 {
		g.kill(m, nil, true, true)
	}
}

// land puts it on p, or the nearest free tile around (no item, no
// stairs); with none, it is lost; in water, it sinks.
func (g *Game) land(it *Item, p Point) {
	if g.Level.At(p) == TileWater {
		g.say(g.Name(it) + "は水に沈んでしまった。")
		return
	}
	for r := 0; r <= 2; r++ {
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				q := p.Add(Point{x, y})
				if chebyshev(p, q) != r || !g.Level.At(q).Walkable() || g.stairsAt(q) {
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
