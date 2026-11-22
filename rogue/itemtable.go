package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// ItemTable is a named item table: it picks an item in two stages, first
// its kind (category) by weight, then an item of that kind by weight. So
// adding items of a kind does not crowd out the other kinds: each kind
// keeps its share. The floors (floors.txt 道具表) and the shops (shops.txt
// 品表) name the table they take their items from.
type ItemTable struct {
	Name string
	// Categories are the weights of the kinds, by floor.
	Categories []CategoryRow
	// Items are the weights of the items among those of their kind, by
	// floor.
	Items []FloorRow
}

// CategoryRow is the weight of a kind of items on the floors [First,
// Last].
type CategoryRow struct {
	First, Last int
	Category    Category
	Weight      int
}

// on says whether the floor is in [first, last].
func on(floor, first, last int) bool { return first <= floor && floor <= last }

// categories returns the kinds the table can give on the floor.
func (t *ItemTable) categories(floor int) []Category {
	var cs []Category
	for _, r := range t.Categories {
		if on(floor, r.First, r.Last) && r.Weight > 0 && !slices.Contains(cs, r.Category) {
			cs = append(cs, r.Category)
		}
	}
	return cs
}

// pick returns an item of the table for the floor: a kind by weight, then
// an item of that kind by weight.
func (t *ItemTable) pick(rng *rand.Rand, d *Dungeon, floor int) *ItemDef {
	total := 0
	for _, r := range t.Categories {
		if on(floor, r.First, r.Last) {
			total += r.Weight
		}
	}
	n := rng.IntN(total)
	var c Category
	for _, r := range t.Categories {
		if !on(floor, r.First, r.Last) {
			continue
		}
		if n < r.Weight {
			c = r.Category
			break
		}
		n -= r.Weight
	}
	return t.pickOf(rng, d, floor, c)
}

// pickOf picks an item of the category c of the table for the floor, by
// weight; nil if the table has none there.
func (t *ItemTable) pickOf(rng *rand.Rand, d *Dungeon, floor int, c Category) *ItemDef {
	var rows []FloorRow
	for _, r := range t.Items {
		if on(floor, r.First, r.Last) && d.Item(r.ID).Category == c {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return d.Item(pick(rng, rows, floor))
}

// ItemTable returns the item table name, or nil.
func (d *Dungeon) ItemTable(name string) *ItemTable { return d.ItemTables[name] }

// loadItemTables reads category_tables.txt and item_tables.txt.
func (l *loader) loadItemTables(d *Dungeon) {
	d.ItemTables = map[string]*ItemTable{}
	table := func(name string) *ItemTable {
		t, ok := d.ItemTables[name]
		if !ok {
			t = &ItemTable{Name: name}
			d.ItemTables[name] = t
		}
		return t
	}
	for _, row := range l.table("category_tables.txt", "表", "開始", "終了", "種類", "頻度") {
		r := CategoryRow{Category: Category(row.cols["種類"]), Weight: l.int(row, "頻度", 0)}
		r.First, r.Last = l.floors(row)
		if !slices.Contains(categories, r.Category) {
			l.errorf(row.file, row.n, "種類: no kind %q", r.Category)
			continue
		}
		t := table(row.cols["表"])
		t.Categories = append(t.Categories, r)
	}
	for _, row := range l.table("item_tables.txt", "表", "開始", "終了", "ID", "頻度") {
		r := FloorRow{ID: row.cols["ID"], Weight: l.int(row, "頻度", 1)}
		r.First, r.Last = l.floors(row)
		if d.Item(r.ID) == nil {
			l.errorf(row.file, row.n, "ID: no item %q", r.ID)
			continue
		}
		name := row.cols["表"]
		if _, ok := d.ItemTables[name]; !ok {
			l.errorf(row.file, row.n, "表: no table %q in category_tables.txt", name)
			continue
		}
		t := table(name)
		t.Items = append(t.Items, r)
	}
}

// checkItemTable says what is wrong with the table name on the floor:
// that it gives nothing there, a kind with no item, an item of a kind it
// never gives, or (for a shop) money.
func (d *Dungeon) checkItemTable(name string, floor int, shop bool) error {
	t := d.ItemTable(name)
	if t == nil {
		return fmt.Errorf("no item table %q", name)
	}
	cs := t.categories(floor)
	if len(cs) == 0 {
		return fmt.Errorf("the item table %q gives nothing on %dF", name, floor)
	}
	for _, c := range cs {
		if shop && c == CategoryMoney {
			return fmt.Errorf("the item table %q of a shop has money", name)
		}
		if !slices.ContainsFunc(t.Items, func(r FloorRow) bool {
			return on(floor, r.First, r.Last) && r.Weight > 0 && d.Item(r.ID).Category == c
		}) {
			return fmt.Errorf("the item table %q has no item of %s on %dF", name, c, floor)
		}
	}
	for _, r := range t.Items {
		if on(floor, r.First, r.Last) && !slices.Contains(cs, d.Item(r.ID).Category) {
			return fmt.Errorf("the item table %q never gives %s on %dF (no weight for %s)",
				name, r.ID, floor, d.Item(r.ID).Category)
		}
	}
	return nil
}
