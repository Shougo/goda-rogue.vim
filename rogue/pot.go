package rogue

import (
	"fmt"
	"slices"
)

// The commands of a pot, and of an item in it.
const (
	commandLook    = "見る"
	commandPutIn   = "入れる"
	commandTakeOut = "出す"
)

// The prompts of a pot: the item to put in it, an item in it, what to do
// with that item.
const (
	promptPutIn = "put in"
	promptPot   = "pot"
	promptInPot = "in pot"
)

// potLabel shows how many more items it holds, once known, and the icons
// of what is in it: "[2]【草巻】".
func (g *Game) potLabel(it *Item) string {
	s := ""
	switch {
	case !g.identified[it.Def.ID]:
	case !it.Known:
		s = "[?]"
	default:
		s = fmt.Sprintf("[%d]", it.Charges-len(it.Contents))
	}
	if len(it.Contents) > 0 {
		s += "【"
		for _, c := range it.Contents {
			s += c.Glyph
		}
		s += "】"
	}
	return s
}

// askPutIn asks which item to put in the pot it (at index i); it says
// whether the turn passes (a cursed pot takes nothing).
func (g *Game) askPutIn(i int) bool {
	pot := g.Inventory[i]
	if g.curseStops(pot) {
		return true
	}
	if len(pot.Contents) >= pot.Charges {
		g.learn(pot)
		g.say(g.Name(pot) + "はいっぱいだ。")
		return false
	}
	p := &Prompt{Title: g.Name(pot) + "に入れる", Multi: true, kind: promptPutIn, item: i}
	// Not the goods not paid for: in the pot, they would be the
	// player's; nor what a curse keeps on the player.
	for j, it := range g.Inventory {
		if it.Def.Category != CategoryPot && !it.ForSale && !(it.Equipped && it.Cursed) {
			p.Choices = append(p.Choices, g.Entry(it))
			p.items = append(p.items, j)
		}
	}
	if len(p.Choices) == 0 {
		g.say("入れるものがない。")
		return false
	}
	g.Prompt = p
	return false
}

// putIn puts it in pot: it is kept, merged with what is in a pot of
// synthesis, or paid to a pot of healing.
func (g *Game) putIn(pot, it *Item) {
	if it.Mimic != nil {
		g.unmask(it, true)
		return
	}
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	it.Equipped, it.used = false, false
	g.say(g.Name(it) + "を" + g.Name(pot) + "に入れた。")
	// Putting things in tells what the pot is, and how much it holds.
	g.learn(pot)
	if pot.Def.Effect == EffectSynthesis {
		for _, base := range pot.Contents {
			if g.merge(base, it, false) {
				// What is merged takes its room all the same: two
				// items merged into one take two.
				pot.Charges--
				return
			}
		}
	}
	if pot.Def.Effect == EffectBottomless {
		// (As a pot of healing: its room goes with the item.)
		pot.Charges--
		g.say(g.Name(it) + "は底から抜けて消えた。")
		return
	}
	switch pot.Def.Effect {
	case EffectAppraise, EffectStronger, EffectWeaker, EffectCash, EffectChange,
		EffectBlessPot, EffectCursePot, EffectClearPot, EffectSlotPot, EffectMendPot, EffectSleepPot, EffectSmokePot:
		pot.Contents = append(pot.Contents, g.changedIn(pot, it))
		return
	}
	if pot.Def.Effect == EffectHealPot {
		// The item is gone, and its room with it: the pot heals as many
		// times as it holds, whatever is put in it.
		pot.Charges--
		g.HP = g.MaxHP
		g.say(g.Name(it) + "は消えて、HPが全部回復した。")
		return
	}
	pot.Contents = append(pot.Contents, it)
}

// merge merges it into base, and says whether it did: a weapon or a
// shield gets the seals (as many as its slots hold) and the upgrade value
// of another of its category; with mixed, also the seal of a material for
// its category (異種合成: a seal for weapons or for shields, items.txt
// 印); a bracelet is mended by the same bracelet. A pot of synthesis
// merges items of a kind only; mixed synthesis is left to a monster, as
// in Shiren.
func (g *Game) merge(base, it *Item, mixed bool) bool {
	c := base.Def.Category
	switch {
	case (c == CategoryWeapon || c == CategoryShield) && it.Def.Category == c:
		base.Upgrade += it.Upgrade
		// The innate ability of the material becomes a seal; that of
		// the base stays as it was.
		if it.Def.Seal != "" {
			g.addSeal(base, it.Def.Seal)
		}
		for _, s := range it.Seals {
			g.addSeal(base, s)
		}
		base.Known = base.Known && it.Known
		// A curse comes along with the rest, unless a blessing of
		// either takes it; a blessing does not come along.
		base.Cursed = (base.Cursed || it.Cursed) && !(base.Blessed || it.Blessed)
		base.Blessed = false
		g.say(g.Name(base) + "と" + g.Name(it) + "が合成された！")
		return true
	case mixed && (c == CategoryWeapon || c == CategoryShield) && it.Def.Category != CategoryWeapon &&
		it.Def.Category != CategoryShield && g.Dungeon.SealOn(it.Def.Seal, c) != nil:
		// (Merged even with no room: the material is used up, as the
		// player chose to put it in.)
		g.addSeal(base, it.Def.Seal)
		g.learn(it)
		g.say(g.Name(base) + "と" + g.Name(it) + "が合成された！")
		return true
	case c == CategoryBracelet && it.Def == base.Def:
		base.Cracks = 0
		g.say(g.Name(base) + "のひびが直った！")
		return true
	}
	return false
}

// addSeal adds the seal glyph to base, if it has room (the same seal
// again too: they add up, seal.go), but サビよけ (錆), which does not add
// up (Shougo, 2026-11-27): not on one with サビよけ already.
func (g *Game) addSeal(base *Item, glyph string) {
	if s := g.Dungeon.SealOn(glyph, base.Def.Category); s != nil && s.Effect == SealNoRust && g.hasNoRust(base) {
		return
	}
	if s := g.Dungeon.SealOn(glyph, base.Def.Category); s != nil && s.AsSeal == 0 {
		// (An ability that is no seal: 掘る, a heavy shield's.)
		return
	}
	if len(base.Seals) < base.slots() {
		base.Seals = append(base.Seals, glyph)
	}
}

// lookIn shows what is in the pot it (at index i), to choose one.
func (g *Game) lookIn(i int) {
	pot := g.Inventory[i]
	if len(pot.Contents) == 0 {
		g.say(g.Name(pot) + "には何も入っていない。")
		return
	}
	p := &Prompt{Title: g.Name(pot) + "の中", kind: promptPot, item: i}
	for _, it := range pot.Contents {
		p.Choices = append(p.Choices, g.Entry(it))
	}
	g.Prompt = p
}

// inPotCommands returns what can be done with an item in pot: in a pot of
// keeping, it is used (or thrown) where it is; from any pot, it can be
// taken out.
func inPotCommands(pot, it *Item) []string {
	var cs []string
	if pot.Def.Effect == EffectKeep {
		if c, ok := useCommands[it.Def.Category]; ok {
			cs = append(cs, c)
		}
		cs = append(cs, commandThrow)
	}
	return append(cs, commandTakeOut, commandInfo)
}

// askInPot asks what to do with the item at index n in the pot at index i
// of the inventory.
func (g *Game) askInPot(i, n int) {
	pot := g.Inventory[i]
	it := pot.Contents[n]
	g.Prompt = &Prompt{
		Title:   g.Label(it),
		Choices: inPotCommands(pot, it),
		kind:    promptInPot, item: i, items: []int{n},
	}
}

// doInPot does the command c with the item at index n in the pot at index
// i; it says whether the turn passes.
func (g *Game) doInPot(i, n int, c string) bool {
	pot := g.Inventory[i]
	it := pot.Contents[n]
	if ok, turn := g.allowed(it, pot, placePot, c); !ok {
		return turn
	}
	switch c {
	case commandInfo:
		g.showInfo(it)
		return false
	case commandTakeOut:
		if pot.Def.Effect == EffectSynthesis || pot.Def.Effect == EffectPlain {
			g.learn(pot)
			g.say(g.Name(pot) + "からは出せない。")
			return true
		}
		return g.takeOut(pot, n)
	case commandThrow:
		g.throwItem(it)
		return true
	case commandDrink, commandRead, commandWave, commandEat:
		g.useItem(it)
		return true
	}
	return false
}

// takeOut takes the item at index n out of pot, into the inventory.
func (g *Game) takeOut(pot *Item, n int) bool {
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		g.say("持ち物がいっぱいで、出せない。")
		return false
	}
	it := pot.Contents[n]
	pot.Contents = slices.Delete(pot.Contents, n, n+1)
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	g.say(g.Name(it) + "を" + g.Name(pot) + "から出した。")
	return true
}

// discard takes it away from where the player has it: the inventory, a
// pot in it, or under its feet (an item used or thrown).
func (g *Game) discard(it *Item) {
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == it })
	for _, pot := range g.Inventory {
		pot.Contents = slices.DeleteFunc(pot.Contents, func(x *Item) bool { return x == it })
	}
}

// smash breaks a thrown pot on p: what was in it falls around; a pot of
// 底抜け leaves a pit there.
func (g *Game) smash(pot *Item, p Point) {
	g.say(g.Name(pot) + "は割れた。")
	for _, it := range pot.Contents {
		g.land(it, p)
	}
	pot.Contents = nil
	switch pot.Def.Effect {
	case EffectBottomless:
		g.pitAt(p)
	case EffectSleepPot:
		g.potCloud(p, MagicSleep)
	case EffectSmokePot:
		g.potCloud(p, MagicBlind)
	}
}

// potCloud brings the bolt e on the monsters of the room where a pot
// broke on p (in a corridor, those next to p), and on the player if it is
// there.
func (g *Game) potCloud(p Point, e MagicEffect) {
	room := g.Level.RoomAt(p)
	near := func(q Point) bool {
		if room >= 0 {
			return g.Level.RoomAt(q) == room
		}
		return chebyshev(q, p) <= 1
	}
	if e == MagicSleep {
		g.say("眠りの煙が広がった。")
	} else {
		g.say("煙が広がった。")
	}
	for _, m := range slices.Clone(g.Monsters) {
		if near(m.Pos) && m.HP > 0 {
			g.magicOn(m, e, Point{})
		}
	}
	if near(g.Player) {
		g.afflict(e)
	}
}

// pitAt opens a pit on p (a pot of 底抜け broken), seen: who stands on it
// falls, the player (forced) or a monster. Not on water or stairs, a trap
// already there, in a shop, nor on the last floor.
func (g *Game) pitAt(p Point) {
	var def *TrapDef
	for _, d := range g.Dungeon.Traps {
		if d.Effect == TrapPit {
			def = d
			break
		}
	}
	if def == nil || !g.Level.At(p).Walkable() || g.stairsAt(p) || g.TrapAt(p) != nil || g.inShop(p) || g.Depth >= g.Dungeon.Floors {
		return
	}
	t := &Trap{Def: def, Pos: p, Found: true}
	g.Traps = append(g.Traps, t)
	g.say("落とし穴ができた。")
	switch m := g.MonsterAt(p); {
	case p == g.Player:
		g.spring(t, true)
	case m != nil:
		g.monsterSprings(m, t)
	}
}

// changedIn is what it becomes in pot, a pot of 鑑定 (identified, its
// curse told), 強化 or 弱化 (a weapon or a shield: its upgrade value by
// the pot's strength), 換金 (money: what a shop would pay for it) or 変化
// (an item of the item table 変化, Shougo 2026-11-27: some items come
// only of it, some never).
func (g *Game) changedIn(pot, it *Item) *Item {
	n := pot.Def.Strength
	switch pot.Def.Effect {
	case EffectAppraise:
		g.learn(it)
		it.CurseTold = true
	case EffectStronger, EffectWeaker:
		if !it.upgradable() {
			break
		}
		if pot.Def.Effect == EffectWeaker {
			n = -n
		}
		it.Upgrade += n
		if n > 0 {
			g.say(fmt.Sprintf("%sは%d強くなった。", g.Name(it), n))
		} else {
			g.say(fmt.Sprintf("%sは%d弱くなった。", g.Name(it), -n))
		}
	case EffectCash:
		money := g.Dungeon.moneyDef()
		if money == nil || it.Def.Category == CategoryMoney {
			break
		}
		cash := &Item{Def: money, Glyph: money.Glyph, Amount: max(sellPrice(it), 1)}
		g.say(fmt.Sprintf("%sは%dGになった。", g.Name(it), cash.Amount))
		return cash
	case EffectBlessPot:
		if it.cursable() && !it.Blessed {
			g.bless(it)
		}
	case EffectCursePot:
		// (Put in on purpose: no ward keeps it off.)
		switch {
		case !it.cursable() || it.Cursed:
		case it.Blessed:
			it.Blessed, it.CurseTold = false, true
			g.say(g.Name(it) + "の祝福が消えた。")
		default:
			it.Cursed, it.CurseTold = true, true
			g.say(g.Name(it) + "は呪われた。")
		}
	case EffectClearPot:
		if it.Cursed {
			it.Cursed, it.CurseTold = false, true
			g.say(g.Name(it) + "の呪いが解けた。")
		}
	case EffectSlotPot:
		if it.upgradable() && it.slots() < maxSlots {
			g.addSlot(it)
		}
	case EffectMendPot:
		if it.Cracks > 0 {
			it.Cracks = 0
			g.say(g.Name(it) + "のひびが直った。")
		}
	case EffectChange:
		t := g.Dungeon.ItemTable(changeTable)
		if t == nil || len(t.categories(g.Depth)) == 0 {
			break
		}
		def := t.pick(g.rng, g.Dungeon, g.Depth)
		if def == nil {
			break
		}
		to := g.newItemOf(g.rng, def, Point{})
		g.say(g.Name(it) + "は別の物に変わった。")
		return to
	}
	return it
}

// changeTable is the item table of the pots of 変化.
const changeTable = "変化"

// moneyDef is the kind of money, or nil.
func (d *Dungeon) moneyDef() *ItemDef {
	for _, def := range d.Items {
		if def.Category == CategoryMoney {
			return def
		}
	}
	return nil
}
