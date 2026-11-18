package rogue

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// TestTwoStagePicks checks that the kinds come out by their weights,
// whatever the number of items of each kind.
func TestTwoStagePicks(t *testing.T) {
	d := cave()
	table := d.ItemTable("床")
	weights := map[Category]int{}
	total := 0
	for _, r := range table.Categories {
		if on(1, r.First, r.Last) {
			weights[r.Category] += r.Weight
			total += r.Weight
		}
	}
	const n = 50000
	got := map[Category]int{}
	rng := rand.New(rand.NewPCG(1, 2))
	for range n {
		got[table.pick(rng, d, 1).Category]++
	}
	for c, w := range weights {
		want := float64(n) * float64(w) / float64(total)
		if math.Abs(float64(got[c])-want) > want*0.1+50 {
			t.Errorf("%s: %d picks, want about %.0f", c, got[c], want)
		}
	}
	// An item of a kind only appears from its floor (katana from 3F).
	for range 2000 {
		if it := table.pick(rng, d, 1); it.ID == "katana" {
			t.Fatal("katana on 1F")
		}
	}
}

// TestMoreKindsKeepShares adds many grasses: the share of grasses stays.
func TestMoreKindsKeepShares(t *testing.T) {
	m := copyFS(t)
	m["items.txt"].Data = append(m["items.txt"].Data, ""+
		"k1\t草\t草1\t草\t1\tHP回復\t0\t-\t-\t0\t10\n"+
		"k2\t草\t草2\t草\t1\tHP回復\t0\t-\t-\t0\t10\n"+
		"k3\t草\t草3\t草\t1\tHP回復\t0\t-\t-\t0\t10\n"...)
	m["item_tables.txt"].Data = append(m["item_tables.txt"].Data, "床\t1\t30\tk1\t10\n床\t1\t30\tk2\t10\n床\t1\t30\tk3\t10\n"...)
	m["fake_names.txt"].Data = append(m["fake_names.txt"].Data, "草\tあ草\n草\tい草\n草\tう草\n"...)
	d, err := LoadDungeon(m, DefaultDungeon)
	if err != nil {
		t.Fatal(err)
	}
	share := func(d *Dungeon) float64 {
		rng := rand.New(rand.NewPCG(3, 4))
		grass := 0
		for range 20000 {
			if d.ItemTable("床").pick(rng, d, 5).Category == CategoryGrass {
				grass++
			}
		}
		return float64(grass) / 20000
	}
	if a, b := share(cave()), share(d); math.Abs(a-b) > 0.02 {
		t.Errorf("the share of grasses went from %.3f to %.3f", a, b)
	}
}

// TestGuaranteed puts a guaranteed item on one floor of its range.
func TestGuaranteed(t *testing.T) {
	d := *cave()
	// With shops everywhere: a guaranteed item is never in one.
	d.FloorRows = append([]FloorCounts(nil), d.FloorRows...)
	for i := range d.FloorRows {
		d.FloorRows[i].Shop = 100
	}
	// Katana is not on the floors before 3F otherwise.
	d.Guaranteed = []FloorRow{{First: 1, Last: 2, ID: "katana"}}
	for seed := range 20 {
		g := NewGame(&d, "guaranteed"+itoa(seed))
		found := 0
		for f := 1; f <= 2; f++ {
			if f > 1 {
				g.descend()
			}
			found += len(slices.DeleteFunc(slices.Clone(g.Items), func(it *Item) bool {
				return it.Def.ID != "katana" || it.ForSale
			}))
		}
		if found != 1 {
			t.Errorf("seed %d: %d katana on 1F-2F", seed, found)
		}
	}
	// It is placed after the monsters, the items and the shop of the
	// floor: they stay as they are.
	none := *cave()
	none.Guaranteed = nil
	g, h := NewGame(&none, "same"), NewGame(cave(), "same")
	if g.Player != h.Player || len(g.Monsters) != len(h.Monsters) {
		t.Error("a guaranteed item changed the floor")
	}
}
