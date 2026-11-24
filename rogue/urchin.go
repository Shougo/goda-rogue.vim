package rogue

import (
	"fmt"
	"slices"
)

// The monsters of items (phase B): 童 throws the item it stands on, 鏡
// sends staffs and arrows back, 鴉 ruins what the player carries (Shougo,
// 2026-11-24: as ギャドン of Shiren).

// itemToThrow returns the tile of the nearest item a monster that throws
// (AbilityThrow), standing on none, sees: it goes there before it goes
// for the player (Shougo, 2026-11-25).
func (g *Game) itemToThrow(m *Monster) (Point, bool) {
	if _, ok := m.ability(AbilityThrow); !ok {
		return Point{}, false
	}
	if _, ok := g.ItemAt(m.Pos); ok {
		return Point{}, false
	}
	best, found := Point{}, false
	for _, it := range g.Items {
		if it.ForSale || !g.sees(m.Pos, it.Pos) || g.MonsterAt(it.Pos) != nil {
			continue
		}
		if !found || chebyshev(it.Pos, m.Pos) < chebyshev(best, m.Pos) {
			best, found = it.Pos, true
		}
	}
	return best, found
}

// throwAt is m throwing the item it stands on at the player, which it
// sees, by the chance of a (AbilityThrow): along its line to the player
// within its reach, or, with any (ranks 3 and 4), from anywhere within
// it, over the heads of the others. On a line the item flies to the
// first creature on the way: the player takes it (hitPlayerWith); a
// monster lets it fall by itself. It says whether m threw.
func (g *Game) throwAt(m *Monster, a Ability) bool {
	it, ok := g.ItemAt(m.Pos)
	if !ok || it.ForSale || !g.sees(m.Pos, g.Player) {
		return false
	}
	d, _, line := g.lineTo(m.Pos, g.Player, a.Values[0])
	anywhere := !line && a.Values[2] == 1 && chebyshev(m.Pos, g.Player) <= a.Values[0]
	if !line && !anywhere || g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	g.say(m.Name + "は" + g.Name(it) + "を投げつけた！")
	if anywhere {
		if !hits(g.rng, g.Dungeon.Rules.MonsterThrowRate) {
			g.say(g.Name(it) + "は当たらなかった。")
			g.landThrown(it, g.Player)
			return true
		}
		g.hitPlayerWith(it, m)
		return true
	}
	p := m.Pos
	for range throwRange {
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Walkable() {
			break
		}
		next := p.Add(d)
		if next == g.Player {
			if !hits(g.rng, g.Dungeon.Rules.MonsterThrowRate) {
				g.say(g.Name(it) + "は当たらなかった。")
				p = next
				continue
			}
			g.hitPlayerWith(it, m)
			return true
		}
		if g.MonsterAt(next) != nil {
			break
		}
		p = next
	}
	g.landThrown(it, p)
	return true
}

// landThrown is a thrown item it stopping on p: a pot breaks.
func (g *Game) landThrown(it *Item, p Point) {
	if it.Def.Category == CategoryPot {
		g.smash(it, p)
		return
	}
	g.land(it, p)
}

// hitPlayerWith is the item it, thrown by m, hitting the player: an arrow
// hurts as m's own; a grass has its effect on the player, as drunk (a
// cursed one does nothing), and is used up; money hurts by its amount;
// another item hurts a little and falls by the player (a pot breaks).
func (g *Game) hitPlayerWith(it *Item, m *Monster) {
	r := g.Dungeon.Rules
	g.say(g.Name(it) + "が当たった。")
	switch {
	case it.Def.Category == CategoryArrow:
		g.playerHurt(damage(g.rng, m.Attack/2+it.Def.Strength, g.Defense(), r.DefenseK), m)
		return
	case it.Def.Category == CategoryGrass && !it.Cursed:
		g.identify(it)
		g.effectOf(it, false)
		return
	case it.Def.Category == CategoryMoney:
		if !g.playerHurt(max(it.Amount/r.GoldThrow, 1), m) {
			g.land(it, g.Player)
		}
		return
	}
	if g.playerHurt(between(g.rng, 1, 2), m) {
		return
	}
	g.landThrown(it, g.Player)
}

// What a monster that reflects (AbilityReflect) sends back.
const (
	reflectStaffs = 0 // the bolts of staffs
	reflectArrows = 1 // those, and arrows
	reflectAll    = 2 // those, and any item thrown
)

// reflects says whether m sends back what the player sent at it: the
// bolt of a staff (staff), an arrow, or another item thrown. It says so.
func (g *Game) reflects(m *Monster, what int) bool {
	a, ok := m.ability(AbilityReflect)
	if !ok || what > a.Values[1] || g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	m.Dozing = false
	g.say(m.Name + "ははね返した！")
	return true
}

// What a monster that ruins (AbilityRuin) ruins.
const (
	ruinStaff = 0 // empties a staff
	ruinGrass = 1 // takes a grass or a scroll away
	ruinPot   = 2 // makes a pot a plain pot
	ruinAny   = 3 // one of those, at random
)

// ruin is m next to the player ruining an item it carries, by its
// ability a (AbilityRuin), instead of a blow. The seal 盗みよけ wards it
// off, as a theft. It says whether it did: with nothing to ruin (or warded
// off), it attacks.
func (g *Game) ruin(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	kinds := []int{a.Values[1]}
	if a.Values[1] == ruinAny {
		kinds = []int{ruinStaff, ruinGrass, ruinPot}
		g.rng.Shuffle(len(kinds), func(i, j int) { kinds[i], kinds[j] = kinds[j], kinds[i] })
	}
	for _, k := range kinds {
		can := g.ruinable(k)
		if len(can) == 0 {
			continue
		}
		if g.wears(EffectNoTheft) || g.sealChance(SealNoTheft) {
			g.say(m.Name + "は道具を狙ったが、盗みよけで防いだ。")
			return true
		}
		it := can[g.rng.IntN(len(can))]
		name := g.Name(it)
		g.event("ruined %s", m.Def.ID)
		switch k {
		case ruinStaff:
			it.Charges = 0
			g.say(fmt.Sprintf("%sは%sをつついた！　%sの力がなくなった。", m.Name, name, name))
		case ruinGrass:
			g.discard(it)
			g.say(fmt.Sprintf("%sは%sをついばんだ！　%sはなくなった。", m.Name, name, name))
		case ruinPot:
			g.plainPot(it)
			g.say(fmt.Sprintf("%sは%sをつついた！　%sは%sになった。", m.Name, name, name, g.Name(it)))
		}
		return true
	}
	return false
}

// ruinable returns the items carried that the ruin k reaches.
func (g *Game) ruinable(k int) []*Item {
	var can []*Item
	for _, it := range g.Inventory {
		if it.ForSale {
			continue
		}
		switch c := it.Def.Category; {
		case k == ruinStaff && c == CategoryStaff && it.Charges > 0,
			k == ruinGrass && (c == CategoryGrass || c == CategoryScroll),
			k == ruinPot && c == CategoryPot && it.Def.Effect != EffectPlain && g.plainPotDef() != nil:
			can = append(can, it)
		}
	}
	return can
}

// plainPotDef is the kind of the plain pot (普通の壺, Shougo 2026-11-25):
// no effect, what is in it neither used nor taken out, until it breaks.
func (g *Game) plainPotDef() *ItemDef {
	for _, def := range g.Dungeon.Items {
		if def.Category == CategoryPot && def.Effect == EffectPlain {
			return def
		}
	}
	return nil
}

// plainPot makes the pot it a plain pot: what is in it stays there (what
// a pot of synthesis merged stays merged), its room what it had (a pot of
// healing: the room left). Its kind is known at once.
func (g *Game) plainPot(it *Item) {
	it.Charges = max(it.Charges, 0)
	it.Def = g.plainPotDef()
	it.Glyph = it.Def.Glyph
	g.identify(it)
}

// back is the direction opposite d.
func back(d Point) Point { return Point{-d.X, -d.Y} }

// reflectThrown is an item the player threw (it) at m sent back, if m
// reflects it: an arrow hurts the player as its own would hurt m; another
// item falls at the player's feet (a pot breaks there). It says whether
// it was sent back.
func (g *Game) reflectThrown(m *Monster, it *Item) bool {
	what := reflectAll
	if it.Def.Category == CategoryArrow {
		what = reflectArrows
	}
	if !g.reflects(m, what) {
		return false
	}
	if it.Def.Category == CategoryArrow {
		r := g.Dungeon.Rules
		attack := playerAttack(g.PlayerLevel, g.CurrentStrength())/2 + it.Def.Strength
		g.say(g.Name(it) + "が当たった。")
		g.playerHurt(damage(g.rng, attack, g.Defense(), r.DefenseK), m)
		return true
	}
	g.landThrown(it, g.Player)
	return true
}

// rust rusts the weapon or the shield equipped of the categories cs by n
// each, unless サビよけ wards it off (then it says so) or a blessing takes
// it. It says whether something equipped was there to rust.
func (g *Game) rust(cs []Category, n int) bool {
	var its []*Item
	for _, c := range cs {
		if it := g.equipped(c); it != nil {
			its = append(its, it)
		}
	}
	if len(its) == 0 {
		return false
	}
	if g.wears(EffectNoRust) {
		g.say("サビよけの腕輪で守られた。")
		return true
	}
	if g.sealChance(SealNoRust) {
		g.say("サビよけの印で守られた。")
		return true
	}
	for _, it := range its {
		if g.shielded(it) {
			continue
		}
		it.Upgrade -= n
		g.say(g.Name(it) + "がさびてしまった。")
	}
	return true
}

// How a monster of rust (AbilityRust) rusts.
const (
	rustOne    = 0 // the weapon or the shield
	rustBoth   = 1 // both
	rustOrSeal = 2 // one, or half of the time it takes a seal away
)

// rustBy is m next to the player using its ability a (AbilityRust)
// (Shougo, 2026-11-25): it never attacks; by its chance it rusts what the
// player has equipped by its rust (the weapon or the shield, both, or one
// or, half of the time, a seal of one taken away), and otherwise watches.
// A seal of サビよけ goes first, always: it is taken away instead. One
// born with it (no seal) cannot be taken away: the ability fails. With
// nothing equipped, it fails. A blessing takes the rust (and is gone).
// It always acts.
func (g *Game) rustBy(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		g.say(m.Name + "は様子を見ている。")
		return true
	}
	var its []*Item
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		if it := g.equipped(c); it != nil {
			its = append(its, it)
		}
	}
	failed := m.Name + "はサビを吹きかけたが、何も起こらなかった。"
	if len(its) == 0 || g.wears(EffectNoRust) {
		g.say(failed)
		return true
	}
	g.event("rusted %s", m.Def.ID)
	for _, it := range its {
		if s := g.Dungeon.SealOn(it.Def.Seal, it.Def.Category); s != nil && s.Effect == SealNoRust {
			g.say(m.Name + "はサビを吹きかけたが、" + g.Name(it) + "はサビない。")
			return true
		}
	}
	for _, it := range its {
		for i, glyph := range it.Seals {
			if s := g.Dungeon.SealOn(glyph, it.Def.Category); s != nil && s.Effect == SealNoRust {
				g.say(m.Name + "はサビを吹きかけた！")
				g.eraseSeal(it, i)
				return true
			}
		}
	}
	g.say(m.Name + "はサビを吹きかけた！")
	switch a.Values[2] {
	case rustBoth:
	case rustOrSeal:
		if g.rng.IntN(2) == 0 {
			var sealed []*Item
			for _, it := range its {
				if len(it.Seals) > 0 {
					sealed = append(sealed, it)
				}
			}
			if len(sealed) == 0 {
				g.say("何も起こらなかった。")
				return true
			}
			it := sealed[g.rng.IntN(len(sealed))]
			g.eraseSeal(it, g.rng.IntN(len(it.Seals)))
			return true
		}
		fallthrough
	default:
		its = its[g.rng.IntN(len(its)):][:1]
	}
	for _, it := range its {
		if g.shielded(it) {
			continue
		}
		it.Upgrade -= a.Values[1]
		g.say(g.Name(it) + "がさびてしまった。")
	}
	return true
}

// sendsBack says whether m may send back what (reflectStaffs, ...), as a
// player who knows its kind would know.
func sendsBack(m *Monster, what int) bool {
	a, ok := m.ability(AbilityReflect)
	return ok && what <= a.Values[1]
}
