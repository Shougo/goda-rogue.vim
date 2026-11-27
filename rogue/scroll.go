package rogue

import (
	"fmt"
	"slices"
)

// The scrolls of the second batch (Shougo, 2026-11-25: more scrolls than
// Shiren 6). Those that act on the enemies reach those of the room, or
// those next to the player in a corridor (enemiesAround), as the scrolls
// of a bolt do; those that ask for an item ask after the scroll is read,
// as 識別 does (the scroll is gone either way).

// The prompts of the scrolls that ask for an item.
const (
	promptOnigiri = "onigiri"
	promptPotUp   = "pot up"
	promptSuck    = "suck"
	promptBless   = "bless"
	promptPlate   = "plate"
)

// slash is the scroll of 真空斬り: n damage to each enemy around, from the
// player (its EXP, when one dies). The weakness of the player (仙) does
// not lessen it: it is the scroll's.
func (g *Game) slash(n int) {
	ms := g.enemiesAround()
	if len(ms) == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.say("真空の刃が敵を切り裂いた！")
	for _, m := range ms {
		if !slices.Contains(g.Monsters, m) {
			// (One that split or died of another already.)
			continue
		}
		m.Dozing = false
		g.provoke(m)
		m.HP -= n
		g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
		if m.HP > 0 {
			g.wounded(m)
			continue
		}
		g.kill(m, nil, true, true)
	}
}

// wipe is the scroll of 全滅: the enemies around die, their EXP to the
// player; a shopkeeper (or a guard) is not touched, angry or not.
func (g *Game) wipe() {
	var ms []*Monster
	for _, m := range g.enemiesAround() {
		if !m.Keeper {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		g.say("何も起こらなかった。")
		return
	}
	for _, m := range ms {
		if slices.Contains(g.Monsters, m) {
			g.kill(m, nil, true, true)
		}
	}
}

// clearTraps is the scroll of 罠消し: every trap of the floor is gone,
// found or not.
func (g *Game) clearTraps() {
	if len(g.Traps) == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.Traps = nil
	g.say("フロアの罠がすべて消えた。")
}

// seer is the scroll of 千里眼: the player senses the monsters of the
// floor until it leaves it (as a bracelet of sensing of the whole floor).
func (g *Game) seer() {
	g.Seer = true
	g.say("フロアの敵の気配が分かるようになった。")
}

// gather is the scroll of 道具寄せ: the items of the floor (not the goods
// of a shop) come to the tiles around the player, nearest first; those
// with no room left stay where they are.
func (g *Game) gather() {
	var its []*Item
	for _, it := range g.Items {
		if !it.ForSale && chebyshev(it.Pos, g.Player) > 0 {
			its = append(its, it)
		}
	}
	slices.SortStableFunc(its, func(a, b *Item) int { return chebyshev(a.Pos, g.Player) - chebyshev(b.Pos, g.Player) })
	moved := 0
	for _, it := range its {
		// (Its own tile is free while it moves.)
		from := it.Pos
		it.Pos = Point{-1, -1}
		p, ok := g.freeFloor(g.Player)
		if !ok || chebyshev(p, g.Player) >= chebyshev(from, g.Player) {
			it.Pos = from
			continue
		}
		it.Pos = p
		moved++
	}
	if moved == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.say("フロアの道具が引き寄せられた。")
}

// freeFloor returns the nearest tile to p on which an item may lie: the
// floor (no water, no rock), no stairs, no item.
func (g *Game) freeFloor(p Point) (Point, bool) {
	for r := 0; r < max(g.Level.W, g.Level.H); r++ {
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				q := p.Add(Point{x, y})
				if chebyshev(p, q) != r || !g.Level.At(q).Walkable() || g.stairsAt(q) {
					continue
				}
				if _, ok := g.ItemAt(q); !ok {
					return q, true
				}
			}
		}
	}
	return Point{}, false
}

// lost is the scroll of 迷子: the player is sent somewhere on the floor
// (leaving a shop unpaid is theft, as with a warp).
func (g *Game) lost() {
	g.warpAway()
}

// A scroll that chooses (Shougo, 2026-11-27) lists every item carried,
// whatever it does: unknown, the scroll must not tell by its choices.
// One it cannot work on (nor the goods of a shop) does nothing, and the
// scroll stays unknown.
type chooser struct {
	title string
	// valid says whether it works on the item.
	valid func(g *Game, it *Item) bool
	apply func(g *Game, it *Item)
}

// promptEraseItem asks for the item of the scroll of 印消し, and then
// promptErase for its seal (seals.go).
const promptEraseItem = "erase item"

var choosers = map[string]chooser{
	promptOnigiri: {"どれをおにぎりにする？", func(g *Game, it *Item) bool { return !it.Equipped && g.Dungeon.plainOnigiri() != nil }, (*Game).toOnigiri},
	promptPotUp:   {"どの壺を大きくする？", func(g *Game, it *Item) bool { return it.Def.Category == CategoryPot }, (*Game).potUp},
	promptSuck: {"どの壺の中身を出す？", func(g *Game, it *Item) bool {
		return it.Def.Category == CategoryPot && len(it.Contents) > 0
	}, (*Game).suckOut},
	promptBless: {"どれを祝福する？", func(g *Game, it *Item) bool { return it.cursable() && !it.Blessed }, (*Game).bless},
	promptPlate: {"どれにメッキする？", func(g *Game, it *Item) bool {
		return it.upgradable() && len(it.Seals) < it.slots() && !g.hasNoRust(it)
	}, (*Game).plate},
	promptSlot: {"どれの印の枠を増やす？", func(g *Game, it *Item) bool {
		return it.upgradable() && it.slots() < maxSlots
	}, (*Game).addSlot},
	// (Any that can be cursed: not cursed, it says so.)
	promptUncurse:   {"どれの呪いを解く？", func(g *Game, it *Item) bool { return it.cursable() }, (*Game).uncurseOne},
	promptEraseItem: {"どれの印を消す？", func(g *Game, it *Item) bool { return it.upgradable() && len(it.Seals) > 0 }, nil},
}

// ask asks for an item carried, for the scroll that chooses of kind; it
// says whether there is any.
func (g *Game) ask(kind string) bool {
	c := choosers[kind]
	p := &Prompt{Title: c.title, kind: kind}
	for i, it := range g.Inventory {
		p.Choices = append(p.Choices, g.Entry(it))
		p.items = append(p.items, i)
		p.targets = append(p.targets, it)
	}
	if len(p.Choices) == 0 {
		return false
	}
	g.Prompt = p
	return true
}

// chosen is it chosen for the scroll of the prompt p: its effect, if it
// works on it (then the scroll is known).
func (g *Game) chosen(p *Prompt, it *Item) {
	c := choosers[p.kind]
	if !slices.Contains(g.Inventory, it) {
		g.say("それはもう手元にない。")
		return
	}
	if it.ForSale || !c.valid(g, it) {
		g.say("何も起こらなかった。")
		return
	}
	if p.kind == promptEraseItem {
		g.askSeal(it, p.scroll)
		return
	}
	c.apply(g, it)
	g.used(p)
}

// plainOnigiri returns the kind of the onigiri: the food of the effect
// 満腹 that fills least, or nil.
func (d *Dungeon) plainOnigiri() *ItemDef {
	var o *ItemDef
	for _, def := range d.Items {
		if def.Category == CategoryFood && def.Effect == EffectFood && (o == nil || def.Strength < o.Strength) {
			o = def
		}
	}
	return o
}

// toOnigiri turns it, carried, into an onigiri (a mimic hidden as it
// too: it is gone).
func (g *Game) toOnigiri(it *Item) {
	o := g.Dungeon.plainOnigiri()
	name := g.Name(it)
	*it = Item{Def: o, Glyph: o.Glyph}
	g.say(name + "がおにぎりになった。")
}

// potUp makes the pot it hold one more.
func (g *Game) potUp(it *Item) {
	it.Charges++
	g.say(g.Name(it) + "が大きくなった。")
}

// suckOut takes everything out of the pot it, to the tiles around the
// player (a pot of 合成 or 普通 too); those with no room left are lost.
// The pot keeps its size.
func (g *Game) suckOut(it *Item) {
	for _, in := range it.Contents {
		p, ok := g.freeFloor(g.Player)
		if !ok {
			g.say(g.Name(in) + "は消えてしまった。")
			continue
		}
		in.Pos = p
		g.Items = append(g.Items, in)
	}
	it.Contents = nil
	g.say(g.Name(it) + "の中身が飛び出した。")
}

// bless blesses it; a cursed item loses its curse instead (an item is
// never both).
func (g *Game) bless(it *Item) {
	it.CurseTold = true
	if it.Cursed {
		it.Cursed = false
		g.say(g.Name(it) + "の呪いが解けた。")
		return
	}
	it.Blessed = true
	g.say(g.Name(it) + "は祝福された。")
}

// plate puts 錆 on it (Shougo, 2026-11-27: メッキ is a seal, as in
// Shiren 6; 錆 does not add up, and its risk is a monster of rust taking
// it away).
func (g *Game) plate(it *Item) {
	it.Seals = append(it.Seals, "錆")
	g.say(g.Name(it) + "にサビよけの印が付いた。")
}

// hasNoRust says whether it has サビよけ, as a seal or born with it.
func (g *Game) hasNoRust(it *Item) bool {
	for _, glyph := range append([]string{it.Def.Seal}, it.Seals...) {
		if s := g.Dungeon.SealOn(glyph, it.Def.Category); s != nil && s.Effect == SealNoRust {
			return true
		}
	}
	return false
}
