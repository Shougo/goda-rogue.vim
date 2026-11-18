package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
)

// Category is what an item is; it decides what the item can do.
type Category string

// The categories, in the order of the inventory.
const (
	CategoryWeapon   Category = "武器"
	CategoryShield   Category = "盾"
	CategoryBracelet Category = "腕輪"
	CategoryArrow    Category = "矢"
	CategoryGrass    Category = "草"
	CategoryScroll   Category = "巻物"
	CategoryStaff    Category = "杖"
	CategoryPot      Category = "壺"
	CategoryFood     Category = "食料"
	// CategoryMoney adds to the player's money when walked on; picked up
	// (or swapped), it is carried, to throw.
	CategoryMoney Category = "お金"
)

var categories = []Category{
	CategoryWeapon, CategoryShield, CategoryBracelet, CategoryArrow, CategoryGrass,
	CategoryScroll, CategoryStaff, CategoryPot, CategoryFood, CategoryMoney,
}

// Item is an item, lying on the floor or carried.
type Item struct {
	Def   *ItemDef
	Pos   Point
	Glyph string
	// Amount is how much money it is, for money.
	Amount int
	// Upgrade is the upgrade value of a weapon or a shield (+3, -1).
	Upgrade  int
	Equipped bool
	// Cracks are the cracks of a bracelet.
	Cracks int
	// Charges are how many more times a staff can be waved.
	Charges int
	// Count is how many arrows it is.
	Count int
	// ForSale says whether it belongs to the shop: lying there, or
	// carried and not paid for.
	ForSale bool
	// Seals are the seals of a weapon or a shield (their glyphs).
	Seals []string
	// Contents are the items in a pot; Charges is how many it holds.
	Contents []*Item
	// Known says whether the player knows the upgrade value of a weapon
	// or a shield, the charges of a staff.
	Known bool
	// used says whether a bracelet had its effect on this floor.
	used bool
}

// equipment says whether the item is worn: a weapon, a shield or a
// bracelet.
func (it *Item) equipment() bool {
	switch it.Def.Category {
	case CategoryWeapon, CategoryShield, CategoryBracelet, CategoryArrow:
		return true
	}
	return false
}

// upgradable says whether the item has an upgrade value: a weapon or a
// shield.
func (it *Item) upgradable() bool {
	return it.Def.Category == CategoryWeapon || it.Def.Category == CategoryShield
}

// Strength is the strength of a weapon or a shield, upgrade included.
func (it *Item) Strength() int { return max(it.Def.Strength+it.Upgrade, 0) }

// Label is the item as the inventory shows it: its glyph as an icon, its
// name ("?" after the name of an unidentified kind), its upgrade value or
// charges once known, its cracks, and whether it is equipped.
func (g *Game) Label(it *Item) string {
	return it.Glyph + " " + g.describe(it)
}

// Entry is the line of it in a list of items: its icon, a column that marks
// equipped items with E, and its description, so the names line up.
func (g *Game) Entry(it *Item) string {
	mark := " "
	if it.Equipped {
		mark = "E"
	}
	return it.Glyph + " " + mark + " " + g.describe(it)
}

// describe is the label of it without its icon.
func (g *Game) describe(it *Item) string {
	s := g.Name(it)
	if !g.identified[it.Def.ID] {
		s += "?"
	}
	switch {
	case it.upgradable():
		switch {
		case !it.Known && g.identified[it.Def.ID]:
			// Not equipped nor identified yet: "木刀?", as a bracelet.
			s += "?"
		case it.Known && it.Upgrade != 0:
			s += fmt.Sprintf("%+d", it.Upgrade)
		}
		s += sealsLabel(it)
	case it.Def.Category == CategoryPot:
		s += g.potLabel(it)
	case it.Def.Category == CategoryArrow:
		s += fmt.Sprintf("[%d]", it.Count)
	case it.Def.Category == CategoryStaff && g.identified[it.Def.ID]:
		if it.Known {
			s += fmt.Sprintf("[%d]", it.Charges)
		} else {
			s += "[?]"
		}
	}
	if it.Cracks > 0 {
		s += fmt.Sprintf(" [ひび%d]", it.Cracks)
	}
	if it.ForSale {
		s += fmt.Sprintf(" %dG", price(it))
	}
	return s
}

// rollUpgrade returns the upgrade value of a weapon or a shield found on
// the floor (provisional, until it is data): mostly 0.
func rollUpgrade(rng *rand.Rand) int {
	switch n := rng.IntN(20); {
	case n < 14:
		return 0
	case n < 17:
		return 1
	case n < 18:
		return 2
	case n < 19:
		return 3
	}
	return -1
}

// Prompt is a choice the game asks the player for: the Vim script shows
// it with the editor's selection UI, and answers with the "choose" action
// (the index of the choice, from 0) or "cancel".
type Prompt struct {
	Title   string   `json:"title"`
	Choices []string `json:"choices"`

	// What the choice is for: the inventory, what to do with the item at
	// index item, or which item to identify (items are the indexes of
	// the choices in the inventory).
	kind  string
	item  int
	items []int
}

// The kinds of prompts.
const (
	promptInventory = "inventory"
	promptItem      = "item"
	promptIdentify  = "identify"
	promptFoot      = "foot"
	promptFootTrap  = "foot trap"
	promptSwap      = "swap"
)

// The commands of an item.
const (
	commandEquip   = "装備する"
	commandUnequip = "外す"
	commandDrop    = "置く"
	commandDrink   = "飲む"
	commandRead    = "読む"
	commandWave    = "振る"
	commandEat     = "食べる"
	commandThrow   = "投げる"
	commandPickUp  = "拾う"
	commandPurse   = "財布に入れる"
	commandSwap    = "交換"
)

// useCommands are the commands that use an item, by its category.
var useCommands = map[Category]string{
	CategoryGrass:  commandDrink,
	CategoryScroll: commandRead,
	CategoryStaff:  commandWave,
	CategoryFood:   commandEat,
}

// commands returns what can be done with it.
func (it *Item) commands() []string {
	var cs []string
	if c, ok := useCommands[it.Def.Category]; ok {
		cs = append(cs, c)
	}
	if it.Def.Category == CategoryPot {
		cs = append(cs, commandLook, commandPutIn)
	}
	if it.Def.Category == CategoryMoney {
		cs = append(cs, commandPurse)
	}
	if it.equipment() {
		if it.Equipped {
			cs = append(cs, commandUnequip)
		} else {
			cs = append(cs, commandEquip)
		}
	}
	return append(cs, commandThrow, commandDrop, commandInfo)
}

// sortInventory sorts the inventory by category, and by the order of the
// items of the dungeon in it.
func (g *Game) sortInventory() {
	order := func(it *Item) (int, int) {
		return slices.Index(categories, it.Def.Category), slices.Index(g.Dungeon.Items, it.Def)
	}
	slices.SortStableFunc(g.Inventory, func(a, b *Item) int {
		ca, ia := order(a)
		cb, ib := order(b)
		if ca != cb {
			return ca - cb
		}
		return ia - ib
	})
}

// pickUp picks up the item under the player, if any; it says whether
// there was one to try. Money walked on goes to the purse (the gold), as
// in Shiren; picked up on purpose, it is carried.
func (g *Game) pickUp(walked bool) bool {
	i := slices.IndexFunc(g.Items, func(it *Item) bool { return it.Pos == g.Player })
	if i < 0 {
		return false
	}
	it := g.Items[i]
	if it.Def.Category == CategoryMoney && walked {
		g.toPurse(it)
		return true
	}
	// Arrows join those of their kind.
	if it.Def.Category == CategoryArrow {
		if j := slices.IndexFunc(g.Inventory, func(x *Item) bool { return x.Def == it.Def && x.ForSale == it.ForSale }); j >= 0 {
			g.Items = slices.Delete(g.Items, i, i+1)
			g.Inventory[j].Count += it.Count
			g.say(fmt.Sprintf("%sを%d本拾った。", g.Name(it), it.Count))
			return true
		}
	}
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		g.say("持ち物がいっぱいで、" + g.Name(it) + "を拾えない。")
		return true
	}
	g.Items = slices.Delete(g.Items, i, i+1)
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	g.say(g.Name(it) + "を拾った。")
	g.event("pickup %s", it.Def.ID)
	if it.ForSale {
		g.say(fmt.Sprintf("店主「%dG です。」", price(it)))
	}
	return true
}

// foot is the item under the player, to choose from the inventory as in
// Shiren, or nil. Money is there only when the player did not walk on it
// (a dash stops on it without picking it up).
func (g *Game) foot() *Item {
	it, ok := g.ItemAt(g.Player)
	if !ok {
		return nil
	}
	return it
}

// toPurse puts the money it (carried, or on the floor) in the purse.
func (g *Game) toPurse(it *Item) {
	g.discard(it)
	g.Gold += it.Amount
	g.say(fmt.Sprintf("%dG を財布に入れた。", it.Amount))
}

// footIndex and footTrapIndex stand for the item and the found trap under
// the player in the items of a prompt.
const (
	footIndex     = -1
	footTrapIndex = -2
)

// item returns the item at index i of the inventory, or the item under
// the player for footIndex.
func (g *Game) item(i int) *Item {
	if i == footIndex {
		return g.foot()
	}
	return g.Inventory[i]
}

// footEntry is the line of the item under the player in a list of items.
func (g *Game) footEntry(it *Item) string { return g.Entry(it) + " [足元]" }

// openInventory asks which item: one carried, or the one under the
// player, last (and the trap the player stands on, if found).
func (g *Game) openInventory() {
	choices, items := g.inventory()
	if len(choices) == 0 {
		g.say("何も持っていない。")
		return
	}
	g.Prompt = &Prompt{Title: "持ち物", Choices: choices, kind: promptInventory, items: items}
}

// inventory returns the entries of the inventory, and the indexes of
// their items: those carried, then the item under the player and the
// found trap there.
func (g *Game) inventory() (entries []string, items []int) {
	for i, it := range g.Inventory {
		entries = append(entries, g.Entry(it))
		items = append(items, i)
	}
	if foot := g.foot(); foot != nil {
		entries = append(entries, g.footEntry(foot))
		items = append(items, footIndex)
	}
	if t := g.foundTrapAt(g.Player); t != nil {
		entries = append(entries, glyphTrap+" "+t.Def.Name+" [足元]")
		items = append(items, footTrapIndex)
	}
	return entries, items
}

// itemList is the list of the items shown beside the map.
func (g *Game) itemList() *ItemList {
	l := &ItemList{Title: fmt.Sprintf("持ち物 %d/%d", len(g.Inventory), g.Dungeon.Rules.Capacity)}
	l.Lines, _ = g.inventory()
	if len(l.Lines) == 0 {
		l.Lines = []string{"何も持っていない"}
	}
	return l
}

// footCommands returns what can be done with the item under the player:
// pick it up, swap it with one carried, use it where it is, throw it.
func (g *Game) footCommands(it *Item) []string {
	cs := []string{commandPickUp}
	if it.Def.Category == CategoryMoney {
		cs = append(cs, commandPurse)
	}
	if g.canSwap() && len(g.Inventory) > 0 {
		cs = append(cs, commandSwap)
	}
	if c, ok := useCommands[it.Def.Category]; ok {
		cs = append(cs, c)
	}
	return append(cs, commandThrow, commandInfo)
}

// itemCommands returns what can be done with it, carried: with an item
// under the player, swapping them too.
func (g *Game) itemCommands(it *Item) []string {
	cs := it.commands()
	if g.canSwap() && g.foot() != nil {
		// Before 説明, the last.
		cs = slices.Insert(cs, len(cs)-1, commandSwap)
	}
	return cs
}

// canSwap says whether the player can swap an item it carries with the
// one under it: not in a shop, where putting down and picking up are
// selling and buying.
func (g *Game) canSwap() bool { return !g.inShop(g.Player) }

// swap puts it (carried) down where the player stands, and picks up the
// item that was there, in one turn (as in Shiren).
func (g *Game) swap(it *Item) bool {
	foot := g.foot()
	if foot == nil || !g.canSwap() {
		return false
	}
	if it.Equipped {
		g.unequip(it)
	}
	it.used = false
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
	g.Items = slices.DeleteFunc(g.Items, func(x *Item) bool { return x == foot })
	it.Pos = g.Player
	g.Items = append(g.Items, it)
	g.Inventory = append(g.Inventory, foot)
	g.sortInventory()
	g.say(g.Name(it) + "を置いて、" + g.Name(foot) + "を拾った。")
	g.event("pickup %s", foot.Def.ID)
	return true
}

// askSwap asks which carried item to swap with the one under the player.
func (g *Game) askSwap() {
	p := &Prompt{Title: "どれと交換する？", kind: promptSwap}
	for _, it := range g.Inventory {
		p.Choices = append(p.Choices, g.Entry(it))
	}
	g.Prompt = p
}

// choose answers the prompt with the choice n; it says whether the turn
// passes.
func (g *Game) choose(arg string) (bool, error) {
	p := g.Prompt
	if p == nil {
		return false, fmt.Errorf("choose: nothing to choose")
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 || n >= len(p.Choices) {
		return false, fmt.Errorf("choose: no choice %q", arg)
	}
	g.Prompt = nil
	switch p.kind {
	case promptInventory:
		i := p.items[n]
		if i == footTrapIndex {
			t := g.foundTrapAt(g.Player)
			g.Prompt = &Prompt{
				Title:   t.Def.Name + " [足元]",
				Choices: []string{commandStep},
				kind:    promptFootTrap,
			}
			return false, nil
		}
		if i == footIndex {
			it := g.foot()
			g.Prompt = &Prompt{
				Title:   g.Label(it) + " [足元]",
				Choices: g.footCommands(it),
				kind:    promptFoot,
			}
			return false, nil
		}
		it := g.Inventory[i]
		g.Prompt = &Prompt{
			Title:   g.Label(it),
			Choices: g.itemCommands(it),
			kind:    promptItem, item: i,
		}
		return false, nil
	case promptFoot:
		return g.doAtFoot(p.Choices[n]), nil
	case promptSwap:
		return g.swap(g.Inventory[n]), nil
	case promptFootTrap:
		// Stepped on on purpose: it goes off even when found traps are
		// safe.
		if t := g.foundTrapAt(g.Player); t != nil {
			g.spring(t, true)
		}
		return true, nil
	case promptInfo:
		return false, nil
	case promptDebug, promptDebugFloor, promptDebugItem, promptDebugMonster:
		return false, g.chooseDebug(p, n)
	case promptLook:
		g.lookAt(n)
		return false, nil
	case promptPutIn:
		g.putIn(g.Inventory[p.item], g.Inventory[p.items[n]])
		return true, nil
	case promptPot:
		g.askInPot(p.item, n)
		return false, nil
	case promptInPot:
		return g.doInPot(p.item, p.items[0], p.Choices[n]), nil
	case promptIdentify:
		it := g.item(p.items[n])
		g.learn(it)
		g.say(g.describe(it) + "を識別した。")
		return false, nil
	case promptPay:
		if n == 0 {
			g.pay()
		}
		return false, nil
	case promptSell:
		if n == 0 {
			g.sell()
		}
		g.selling = nil
		return false, nil
	case promptItem:
		it := g.Inventory[p.item]
		switch c := p.Choices[n]; {
		case it.ForSale && c != commandDrop && c != commandEquip && c != commandUnequip:
			g.say("店主「お代を払ってからにしてください。」")
			return false, nil
		}
		switch p.Choices[n] {
		// Arrows are equipped and taken off without a turn, as in Shiren.
		case commandEquip:
			g.equip(it)
			return it.Def.Category != CategoryArrow, nil
		case commandUnequip:
			g.unequip(it)
			return it.Def.Category != CategoryArrow, nil
		case commandDrop:
			return g.drop(p.item), nil
		case commandSwap:
			return g.swap(it), nil
		case commandPutIn:
			return g.askPutIn(p.item), nil
		case commandLook:
			g.lookIn(p.item)
			return false, nil
		case commandInfo:
			g.showInfo(it)
			return false, nil
		case commandPurse:
			// Putting money away costs no turn.
			g.toPurse(it)
			return false, nil
		case commandThrow:
			g.throwItem(g.Inventory[p.item])
			return true, nil
		case commandDrink, commandRead, commandWave, commandEat:
			g.useItem(g.Inventory[p.item])
			return true, nil
		}
	}
	return false, nil
}

// doAtFoot does the command c with the item under the player; it says
// whether the turn passes.
func (g *Game) doAtFoot(c string) bool {
	it := g.foot()
	if it == nil {
		return false
	}
	switch c {
	case commandPickUp:
		return g.pickUp(false)
	case commandPurse:
		g.toPurse(it)
		return false
	case commandSwap:
		g.askSwap()
		return false
	case commandInfo:
		g.showInfo(it)
		return false
	}
	if it.ForSale {
		g.say("店主「お代を払ってからにしてください。」")
		return false
	}
	switch c {
	case commandThrow:
		g.throwItem(it)
	case commandDrink, commandRead, commandWave, commandEat:
		g.useItem(it)
	default:
		return false
	}
	return true
}

// drop puts the item at index i of the inventory under the player; it
// says whether it did (and the turn passes).
func (g *Game) drop(i int) bool {
	it := g.Inventory[i]
	if _, ok := g.ItemAt(g.Player); ok || g.Player == g.Stairs {
		g.say("ここには置けない。")
		return false
	}
	g.Inventory = slices.Delete(g.Inventory, i, i+1)
	it.Equipped, it.used = false, false
	it.Pos = g.Player
	g.Items = append(g.Items, it)
	g.say(g.Name(it) + "を置いた。")
	if g.inShop(g.Player) {
		g.offer(it)
	}
	return true
}

// equipped returns the equipped item of the category c, or nil.
func (g *Game) equipped(c Category) *Item {
	for _, it := range g.Inventory {
		if it.Equipped && it.Def.Category == c {
			return it
		}
	}
	return nil
}

// equip equips it, in place of the item of its category.
func (g *Game) equip(it *Item) {
	if old := g.equipped(it.Def.Category); old != nil {
		old.Equipped = false
	}
	it.Equipped = true
	g.say(g.Name(it) + "を装備した。")
	// Wearing a weapon or a shield tells its upgrade value; a bracelet
	// tells what it is only when its effect shows (use).
	if it.Def.Category != CategoryBracelet {
		g.learn(it)
	}
}

func (g *Game) unequip(it *Item) {
	it.Equipped = false
	g.say(g.Name(it) + "を外した。")
}

// wears says whether the player wears a bracelet with the effect e.
func (g *Game) wears(e Effect) bool {
	b := g.equipped(CategoryBracelet)
	return b != nil && b.Def.Effect == e
}

// CurrentStrength is the strength of the player, with what its bracelet
// adds.
func (g *Game) CurrentStrength() int {
	s := g.Strength
	if b := g.equipped(CategoryBracelet); b != nil && b.Def.Effect == EffectStrength {
		s += b.Def.Strength
	}
	return s
}

// Attack is the attack of the player: its level and strength, and its
// weapon (as Shiren 6: weapon x (0.75 + strength / 32)).
func (g *Game) Attack() int {
	s := g.CurrentStrength()
	a := playerAttack(g.PlayerLevel, s)
	if w := g.equipped(CategoryWeapon); w != nil {
		a += int(float64(w.Strength()) * (0.75 + float64(s)/32))
	}
	return a + g.sealStrength(SealAttack)
}

// Defense is the defense of the player: its shield.
func (g *Game) Defense() int {
	d := g.sealStrength(SealDefense)
	if s := g.equipped(CategoryShield); s != nil {
		d += s.Strength()
	}
	return d
}

// use notes that the bracelet with the effect e, if the player wears it,
// had its effect: that tells what it is.
func (g *Game) use(e Effect) {
	if b := g.equipped(CategoryBracelet); b != nil && b.Def.Effect == e {
		b.used = true
		// Its effect showing tells what it is.
		g.identify(b)
	}
}

// sense notes whether the sensing bracelets had their effect: whether they
// show what the player would not see.
func (g *Game) sense() {
	for _, m := range g.Monsters {
		if m.hostile() && !g.Visible(m.Pos) {
			g.use(EffectSenseEnemy)
			break
		}
	}
	for _, it := range g.Items {
		if !g.IsSeen(it.Pos) {
			g.use(EffectSenseItem)
			break
		}
	}
}

// wearAndTear cracks, as the player leaves the floor, the bracelets that
// had their effect on it: with ひび x (cracks + 1) % times the wear of the
// dungeon, so a crack is more likely with each crack, and one crack too
// many breaks a bracelet.
func (g *Game) wearAndTear() {
	for _, it := range slices.Clone(g.Inventory) {
		if !it.used {
			continue
		}
		it.used = false
		chance := it.Def.Crack * (it.Cracks + 1) * g.Dungeon.Rules.Wear // in 1/10000
		if chance == 0 || g.rng.IntN(10000) >= chance {
			continue
		}
		switch {
		case it.Cracks >= g.Dungeon.Rules.CrackStages:
			g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == it })
			g.say(g.Name(it) + "が壊れた。")
		case it.Cracks == 0:
			it.Cracks++
			g.say(g.Name(it) + "にひびが入った。")
		default:
			it.Cracks++
			g.say(g.Name(it) + "のひびが広がった。")
		}
	}
}
