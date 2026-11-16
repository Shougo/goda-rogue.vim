package rogue

import (
	"fmt"
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
}

// Label is the item as the inventory shows it: its glyph as an icon, and
// its name.
func (it *Item) Label() string {
	return it.Glyph + " " + it.Def.Name
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
const commandDrop = "置く"

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
		g.Prompt = &Prompt{
			Title:   g.Inventory[n].Label(),
			Choices: []string{commandDrop},
			kind:    promptItem, item: n,
		}
		return false, nil
	case promptItem:
		switch p.Choices[n] {
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
	it.Pos = g.Player
	g.Items = append(g.Items, it)
	g.say(it.Def.Name + "を置いた。")
	return true
}
