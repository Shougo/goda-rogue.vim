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
	// CategoryMoney is not carried: it adds to the player's money.
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
	// used says whether a bracelet had its effect on this floor.
	used bool
}

// equipment says whether the item is worn: a weapon, a shield or a
// bracelet.
func (it *Item) equipment() bool {
	switch it.Def.Category {
	case CategoryWeapon, CategoryShield, CategoryBracelet:
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
// name, its upgrade value, and whether it is equipped.
func (it *Item) Label() string {
	s := it.Glyph + " " + it.Def.Name
	if it.upgradable() && it.Upgrade != 0 {
		s += fmt.Sprintf("%+d", it.Upgrade)
	}
	if it.Cracks > 0 {
		s += fmt.Sprintf(" [ひび%d]", it.Cracks)
	}
	if it.Equipped {
		s += " [装備]"
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

	// What the choice is for: the inventory, or what to do with the
	// item at index item.
	kind string
	item int
}

// The kinds of prompts.
const (
	promptInventory = "inventory"
	promptItem      = "item"
)

// The commands of an item.
const (
	commandEquip   = "装備する"
	commandUnequip = "外す"
	commandDrop    = "置く"
)

// commands returns what can be done with it.
func (it *Item) commands() []string {
	var cs []string
	if it.equipment() {
		if it.Equipped {
			cs = append(cs, commandUnequip)
		} else {
			cs = append(cs, commandEquip)
		}
	}
	return append(cs, commandDrop)
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
// there was one to try.
func (g *Game) pickUp() bool {
	i := slices.IndexFunc(g.Items, func(it *Item) bool { return it.Pos == g.Player })
	if i < 0 {
		return false
	}
	it := g.Items[i]
	if it.Def.Category == CategoryMoney {
		g.Gold += it.Amount
		g.Items = slices.Delete(g.Items, i, i+1)
		g.say(fmt.Sprintf("%dG 拾った。", it.Amount))
		return true
	}
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		g.say("持ち物がいっぱいで、" + it.Def.Name + "を拾えない。")
		return true
	}
	g.Items = slices.Delete(g.Items, i, i+1)
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	g.say(it.Def.Name + "を拾った。")
	return true
}

// openInventory asks which item.
func (g *Game) openInventory() {
	if len(g.Inventory) == 0 {
		g.say("何も持っていない。")
		return
	}
	p := &Prompt{Title: "持ち物", kind: promptInventory}
	for _, it := range g.Inventory {
		p.Choices = append(p.Choices, it.Label())
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
		it := g.Inventory[n]
		g.Prompt = &Prompt{
			Title:   it.Label(),
			Choices: it.commands(),
			kind:    promptItem, item: n,
		}
		return false, nil
	case promptItem:
		switch p.Choices[n] {
		case commandEquip:
			g.equip(g.Inventory[p.item])
			return true, nil
		case commandUnequip:
			g.unequip(g.Inventory[p.item])
			return true, nil
		case commandDrop:
			return g.drop(p.item), nil
		}
	}
	return false, nil
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
	g.say(it.Def.Name + "を置いた。")
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
	g.say(it.Def.Name + "を装備した。")
}

func (g *Game) unequip(it *Item) {
	it.Equipped = false
	g.say(it.Def.Name + "を外した。")
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
	return a
}

// Defense is the defense of the player: its shield.
func (g *Game) Defense() int {
	if s := g.equipped(CategoryShield); s != nil {
		return s.Strength()
	}
	return 0
}

// use notes that the bracelet with the effect e, if the player wears it,
// had its effect.
func (g *Game) use(e Effect) {
	if b := g.equipped(CategoryBracelet); b != nil && b.Def.Effect == e {
		b.used = true
	}
}

// sense notes whether the sensing bracelets had their effect: whether they
// show what the player would not see.
func (g *Game) sense() {
	for _, m := range g.Monsters {
		if !m.Ally && !g.Visible(m.Pos) {
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
			g.say(it.Def.Name + "が壊れた。")
		case it.Cracks == 0:
			it.Cracks++
			g.say(it.Def.Name + "にひびが入った。")
		default:
			it.Cracks++
			g.say(it.Def.Name + "のひびが広がった。")
		}
	}
}
