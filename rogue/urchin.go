package rogue

import (
	"fmt"
	"slices"
)

// The monsters of items (phase B): 童 throws the item it stands on, 鏡
// sends staffs and arrows back, 鴉 ruins what the player carries (Shougo,
// 2026-11-24: as ギャドン of Shiren).

// itemToThrow returns the tile of the nearest item a monster that throws
// (AbilityThrow), standing on none, sees: where it goes, the player out
// of its sight.
func (g *Game) itemToThrow(m *Monster) (Point, bool) {
	if _, ok := m.ability(AbilityThrow); !ok {
		return Point{}, false
	}
	if _, ok := g.ItemAt(m.Pos); ok {
		return Point{}, false
	}
	best, found := Point{}, false
	for _, it := range g.Items {
		if it.ForSale || !g.sees(m.Pos, it.Pos) {
			continue
		}
		if !found || chebyshev(it.Pos, m.Pos) < chebyshev(best, m.Pos) {
			best, found = it.Pos, true
		}
	}
	return best, found
}

// throwAt is m throwing the item it stands on along its line to the
// player, which it sees, by the chance of a (AbilityThrow). The item
// flies to the first creature on the way: the player takes it
// (hitPlayerWith); a monster lets it fall by itself. It says whether m
// threw.
func (g *Game) throwAt(m *Monster, a Ability) bool {
	it, ok := g.ItemAt(m.Pos)
	if !ok || it.ForSale {
		return false
	}
	d, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0])
	if !ok || !g.sees(m.Pos, g.Player) || g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	g.say(m.Name + "は" + g.Name(it) + "を投げつけた！")
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
	ruinPot   = 2 // makes a pot a pot of keeping
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
		if g.sealChance(SealNoTheft) {
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
			k == ruinPot && c == CategoryPot && it.Def.Effect != EffectKeep && g.keepingPot() != nil:
			can = append(can, it)
		}
	}
	return can
}

// keepingPot is the kind of pot that keeps what is put in: a plain pot.
func (g *Game) keepingPot() *ItemDef {
	for _, def := range g.Dungeon.Items {
		if def.Category == CategoryPot && def.Effect == EffectKeep {
			return def
		}
	}
	return nil
}

// plainPot makes the pot it a pot of keeping: what is in it can be taken
// out (what a pot of synthesis merged stays merged), its room what it
// had (a pot of healing: the room left).
func (g *Game) plainPot(it *Item) {
	if it.Def.Effect == EffectHealPot {
		it.Charges = max(it.Charges, 0)
	}
	it.Def = g.keepingPot()
	it.Glyph = it.Def.Glyph
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

// rustBy is m next to the player rusting what it has equipped, by its
// ability a (AbilityRust), instead of a blow: the weapon or the shield,
// at random (both with a both), by its rust. It says whether it did:
// with nothing equipped, it attacks.
func (g *Game) rustBy(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	var cs []Category
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		if g.equipped(c) != nil {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		return false
	}
	if a.Values[2] == 0 {
		cs = cs[g.rng.IntN(len(cs)):][:1]
	}
	g.say(m.Name + "はサビを吹きかけた！")
	g.event("rusted %s", m.Def.ID)
	return g.rust(cs, a.Values[1])
}

// sendsBack says whether m may send back what (reflectStaffs, ...), as a
// player who knows its kind would know.
func sendsBack(m *Monster, what int) bool {
	a, ok := m.ability(AbilityReflect)
	return ok && what <= a.Values[1]
}
