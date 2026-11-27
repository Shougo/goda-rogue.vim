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

// slash is the scroll of 斬撃: n damage to each enemy around, from the
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
		k := m.harm(n)
		m.HP -= k
		// (A blow frees it of かなしばり.)
		m.Bound = 0
		g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, k))
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
		if !it.ForSale && !it.Stuck && chebyshev(it.Pos, g.Player) > 0 {
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
	promptPlate: {"どれに錆止めする？", func(g *Game, it *Item) bool {
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

// plate puts 錆 on it (Shougo, 2026-11-27: 錆止め is a seal, as in
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

// gatherFoes is the scroll of 引き寄せ (Shougo, 2026-11-27): the enemies of
// the floor (not a shopkeeper, angry or not) come to the tiles around the
// player, the nearest first, each where it can stand; those with no room
// left stay. Asleep, they stay asleep.
func (g *Game) gatherFoes() {
	var ms []*Monster
	for _, m := range g.Monsters {
		if m.hostile() && !m.Keeper && chebyshev(m.Pos, g.Player) > 1 {
			ms = append(ms, m)
		}
	}
	slices.SortStableFunc(ms, func(a, b *Monster) int { return chebyshev(a.Pos, g.Player) - chebyshev(b.Pos, g.Player) })
	moved := 0
	for _, m := range ms {
		if p, ok := g.nearFor(m, g.Player); ok && chebyshev(p, g.Player) < chebyshev(m.Pos, g.Player) {
			m.Pos = p
			moved++
		}
	}
	if moved == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.say("フロアの敵が引き寄せられた！")
}

// nearFor returns the nearest tile to p (not p) where m can stand, with
// no creature on it.
func (g *Game) nearFor(m *Monster, p Point) (Point, bool) {
	for r := 1; r < max(g.Level.W, g.Level.H); r++ {
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				q := p.Add(Point{x, y})
				if chebyshev(p, q) == r && g.passable(m, q) && !g.occupied(q) {
					return q, true
				}
			}
		}
	}
	return Point{}, false
}

// trouble is the scroll of 救い (Shougo, 2026-11-27): the HP comes back
// in full, and one of the troubles the player is in, at random, is
// solved.
func (g *Game) trouble() {
	g.HP = g.MaxHP
	g.say("HPが全回復した。")
	var fixes []func()
	if adjacent := g.adjacentEnemies(); len(adjacent) >= 2 {
		fixes = append(fixes, func() { g.bind(adjacent) })
	}
	if c := g.fullnessCap(); g.Fullness*2 <= c && g.Fullness < c {
		fixes = append(fixes, func() {
			g.Fullness, g.hunger = c, 0
			g.say("お腹がいっぱいになった。")
		})
	}
	var cursed []*Item
	for _, it := range g.Inventory {
		if it.Equipped && it.Cursed {
			cursed = append(cursed, it)
		}
	}
	if len(cursed) > 0 {
		fixes = append(fixes, func() {
			for _, it := range g.Inventory {
				if it.Equipped {
					it.Cursed, it.CurseTold = false, true
				}
			}
			g.say("装備の呪いが解けた。")
		})
	}
	if g.badStates() {
		fixes = append(fixes, g.cure)
	}
	if g.Strength < g.MaxStrength {
		fixes = append(fixes, func() {
			g.Strength = g.MaxStrength
			g.say("ちからが元に戻った。")
		})
	}
	if len(fixes) > 0 {
		fixes[g.rng.IntN(len(fixes))]()
	}
}

// bind binds the monsters ms (かなしばり).
func (g *Game) bind(ms []*Monster) {
	for _, m := range ms {
		m.Bound = boundTurns
		m.Dozing = false
	}
	g.say("敵がかなしばりになった！")
}

// adjacentEnemies are the enemies next to the player (those hidden in
// the walls too).
func (g *Game) adjacentEnemies() []*Monster {
	var ms []*Monster
	for _, m := range g.Monsters {
		if m.hostile() && chebyshev(m.Pos, g.Player) == 1 {
			ms = append(ms, m)
		}
	}
	return ms
}

// bindAround is the scroll of 金縛り (Shougo, 2026-11-27: the binding of
// 救い): the enemies next to the player are bound.
func (g *Game) bindAround() {
	ms := g.adjacentEnemies()
	if len(ms) == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.bind(ms)
}

// badStates says whether the player is in a bad state (those 予防 wards
// off; 睡眠 and 口封じ keep it from reading a scroll anyway).
func (g *Game) badStates() bool {
	return g.Confused > 0 || g.Slowed > 0 || g.Blind > 0 || g.AttackDown > 0 || g.DefenseDown > 0
}

// cure takes the bad states of the player away.
func (g *Game) cure() {
	g.Confused, g.Slowed, g.Blind, g.AttackDown, g.DefenseDown = 0, 0, 0, 0, 0
	g.say("体の調子が元に戻った。")
}

// powerUpRate is the damage (%) of a blow of the player powered up.
const powerUpRate = 150

// powerUp is the scroll of パワーアップ (Shougo, 2026-11-27): n turns (50)
// of blows of 1.5 times the damage.
func (g *Game) powerUp(n int) {
	g.PowerUp = n
	g.say("力がみなぎってきた！")
}

// prevent is the scroll of 予防 (Shougo, 2026-11-27): the bad states pass,
// and none comes on the player for the floor.
func (g *Game) prevent() {
	if g.badStates() {
		g.cure()
	}
	g.Prevent = true
	g.say("悪い状態を防ぐようになった。")
}

// prevents says, with the ward of 予防, that the state e does not come on
// the player (a good one, as 倍速, does).
func (g *Game) prevents(e MagicEffect) bool {
	if !g.Prevent || !slices.Contains([]MagicEffect{MagicConfuse, MagicSlow, MagicSleep, MagicSeal, MagicBlind, MagicBind, MagicPoison}, e) {
		return false
	}
	g.say("予防の力で防いだ。")
	return true
}

// zowa is the scroll of 同士討ち (Shougo, 2026-11-27: the enemies next to
// the player, as 金縛り): for n turns (10), they attack the other way
// (actZowa).
func (g *Game) zowa(n int) {
	ms := g.adjacentEnemies()
	if len(ms) == 0 {
		g.say("何も起こらなかった。")
		return
	}
	for _, m := range ms {
		m.Zowa = n
	}
	g.say("敵は同士討ちしている……。")
}

// zowaPasses is a turn passing for the monsters of 同士討ち.
func (g *Game) zowaPasses() {
	for _, m := range g.Monsters {
		if m.Zowa > 0 {
			m.Zowa--
		}
	}
}

// actZowa is a turn of the enemy m of 同士討ち: no
// ability; next to the player, it attacks the tile on its other side
// (another monster there takes the blow); else it comes on.
func (g *Game) actZowa(m *Monster) bool {
	for _, d := range around {
		if m.Pos.Add(d) != g.Player || !g.clearFrom(m.Pos, d) {
			continue
		}
		back := Point{-d.X, -d.Y}
		if f := g.MonsterAt(m.Pos.Add(back)); f != nil && g.clearFrom(m.Pos, back) {
			g.hitMonster(m, f)
			return true
		}
		if g.sees(g.Player, m.Pos) {
			g.say(m.Name + "はそっぽを向いて攻撃した。")
		}
		return true
	}
	m.target, m.hasTarget = g.Player, true
	g.stepToward(m, m.target)
	return false
}

// bigRoom is the scroll of 大部屋: the floor is one big room, but for its
// edge and its water. The shopkeepers come by the stairs, the first on
// them while the player owes the shop (Shougo, 2026-11-27: before the
// player, even on them), and the shop is the whole floor; a monster
// house, too.
func (g *Game) bigRoom() {
	l := g.Level
	r := Room{X: 1, Y: 1, W: l.W - 2, H: l.H - 2}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if p := (Point{x, y}); l.At(p) != TileWater && l.At(p) != TileRoom {
				l.set(p, TileRoom)
			}
		}
	}
	l.Rooms, l.Maze = []Room{r}, false
	if g.House >= 0 {
		g.House = 0
	}
	g.say("フロアが大部屋になった！")
	if g.Shop != nil {
		g.Shop.Room, g.Shop.Area = 0, r
		g.keepersToStairs()
	}
	g.look()
}

// keepersToStairs moves the shopkeepers not angry by the stairs (a big
// room): the first on them if the player owes (the player, there, steps
// aside), the others next to them; that is their place from then on, the
// stairs their door.
func (g *Game) keepersToStairs() {
	owes := g.debt() > 0
	for i, m := range g.Shop.Keepers {
		if m.Angry || !slices.Contains(g.Monsters, m) {
			continue
		}
		to := g.Stairs
		if i > 0 || !owes {
			p, ok := g.nearFor(m, g.Stairs)
			if !ok {
				continue
			}
			to = p
		}
		if to == g.Player {
			if p, ok := g.freeFloor(g.Player); ok && p != g.Player {
				g.Player = p
			}
		}
		if o := g.MonsterAt(to); o != nil && o != m {
			if p, ok := g.nearFor(o, to); ok {
				o.Pos = p
			}
		}
		m.Pos, m.door = to, g.Stairs
		if i == 0 && owes {
			// (Its place is still by the stairs, once paid.)
			if p, ok := g.nearFor(m, g.Stairs); ok {
				m.home = p
			}
			continue
		}
		m.home = to
	}
}

// monsterRoom is the scroll of 魔物部屋 (Shougo, 2026-11-27: for EXP and
// items): the room the player is in becomes the monster house of the
// floor, its monsters asleep and its items, no traps. Not in a corridor,
// nor in a shop.
func (g *Game) monsterRoom() {
	i := g.Level.RoomAt(g.Player)
	if i < 0 || g.inShop(g.Player) {
		g.say("何も起こらなかった。")
		return
	}
	g.House, g.houseFound = i, true
	g.stockHouse(g.rng, i, false)
	g.say("モンスターハウスだ！")
}

// curseCarried is the scroll of 呪い (Shougo, 2026-11-27: with 呪い力): n
// of the items carried (3), at random, are cursed, equipped or not (a
// blessed one loses its blessing instead).
func (g *Game) curseCarried(n int) {
	done := false
	for range max(n, 1) {
		var can []*Item
		for _, it := range g.Inventory {
			if !it.Cursed && it.cursable() && !it.ForSale {
				can = append(can, it)
			}
		}
		if len(can) == 0 {
			break
		}
		done = true
		g.curseShown(can[g.rng.IntN(len(can))])
	}
	if !done {
		g.say("何も起こらなかった。")
	}
}

// trapsAround is the scroll of 罠 (Shougo, 2026-11-27: for the traps, and
// with 罠使い): traps of the floor's kinds on the tiles around the player
// where one may lie, found (seen).
func (g *Game) trapsAround() {
	d := g.Dungeon
	n := 0
	for _, dir := range around {
		p := g.Player.Add(dir)
		def := d.Trap(pick(g.rng, d.TrapRows, g.Depth))
		if def == nil || !g.Level.At(p).Walkable() || !g.trapSpot(p, def) || g.inShop(p) {
			continue
		}
		g.Traps = append(g.Traps, &Trap{Def: def, Pos: p, Found: true})
		n++
	}
	if n == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.say("周りに罠ができた！")
}

// sanctuary says whether a scroll of 聖域 lies on p.
func (g *Game) sanctuary(p Point) bool {
	it, ok := g.ItemAt(p)
	return ok && it.Mimic == nil && it.Def.Effect == EffectSanctuary
}

// actSanctuary is a turn of the enemy m while the player stands on a
// scroll of 聖域: it attacks no one but an ally next to it, uses no
// ability on the player, and comes on.
func (g *Game) actSanctuary(m *Monster) bool {
	for _, d := range around {
		if f := g.MonsterAt(m.Pos.Add(d)); f != nil && f.Ally && g.clearFrom(m.Pos, d) {
			g.hitMonster(m, f)
			return true
		}
	}
	if chebyshev(m.Pos, g.Player) > 1 {
		m.target, m.hasTarget = g.Player, true
		g.stepToward(m, m.target)
	}
	return false
}
