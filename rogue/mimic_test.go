package rogue

import (
	"slices"
	"strings"
	"testing"
)

// hidden puts a mimic of the kind id on the floor at p, looking like the
// item of the kind as.
func hidden(g *Game, id, as string, p Point) *Item {
	it := item(g, as, p)
	it.Mimic = kind(id)
	return it
}

// mimics returns the mimics that have shown themselves.
func mimics(g *Game) []*Monster {
	var ms []*Monster
	for _, m := range g.Monsters {
		if _, ok := mimicOf(m.Def); ok {
			ms = append(ms, m)
		}
	}
	return ms
}

// sure makes every blow of the monsters hit.
func sure(g *Game) {
	d := *g.Dungeon
	d.Rules.MonsterHitRate = 100
	g.Dungeon = &d
}

// TestMimicPickedUp: a shallow mimic shows itself when the player walks
// onto it to pick it up: next to the player, with one strike that turn;
// defeated, it drops the item it looked like.
func TestMimicPickedUp(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Monsters = nil
	g.HP, g.MaxHP = 999, 999
	sure(g)
	it := hidden(g, "bake1", "yakusou", Point{r.X + 1, r.Y})
	if s := g.Render(Size{80, 24}); !strings.Contains(strings.Join(s.Lines, "\n"), "草") {
		t.Error("the mimic does not look like a grass on the screen")
	}
	move(t, g, "right")
	ms := mimics(g)
	if len(ms) != 1 || len(g.Inventory) != 0 || slices.Contains(g.Items, it) {
		t.Fatalf("%d mimics, inventory %v", len(ms), g.Inventory)
	}
	m := ms[0]
	if chebyshev(m.Pos, g.Player) != 1 || !strings.Contains(g.Message, "薬草は化け子だった！") {
		t.Errorf("the mimic at %v, the player at %v, %q", m.Pos, g.Player, g.Message)
	}
	if n := strings.Count(g.Message, "のダメージ"); n != 1 {
		t.Errorf("%d strikes in the turn it showed itself, want 1: %q", n, g.Message)
	}

	// It drops what it looked like (at its drop rate: here, always).
	def := *m.Def
	def.Drop = &Drop{What: DropItem, Rate: 100}
	m.Def = &def
	g.remove(m)
	g.defeated(m)
	if !slices.Contains(g.Items, it) || it.Mimic != nil || it.Pos != m.Pos {
		t.Errorf("items %v: want the grass it looked like, where it fell", g.Items)
	}
}

// TestMimicSensed: a sensing bracelet in reach, or eye drops, show a
// shallow mimic (without a strike), not a deep one.
func TestMimicSensed(t *testing.T) {
	g, r := roomGame(t, 10, 3)
	g.Monsters = nil
	hidden(g, "bake2", "yakusou", Point{r.X + 6, r.Y})
	deep := hidden(g, "bake3", "otogiri", Point{r.X + 6, r.Y + 2})
	b := carry(g, "kehai", 0)
	command(t, g, b, commandEquip)
	ms := mimics(g)
	if len(ms) != 1 || ms[0].Def.ID != "bake2" || ms[0].Pos != (Point{r.X + 6, r.Y}) || !slices.Contains(g.Items, deep) {
		t.Fatalf("mimics %v: want the shallow one shown where it lay", ms)
	}

	g, r = roomGame(t, 10, 3)
	g.Monsters = nil
	hidden(g, "bake1", "yakusou", Point{r.X + 6, r.Y})
	deep = hidden(g, "bake4", "otogiri", Point{r.X + 6, r.Y + 2})
	tr := trap(g, "harahe", Point{r.X + 3, r.Y + 1})
	use(t, g, "megusuri")
	if ms := mimics(g); len(ms) != 1 || ms[0].Def.ID != "bake1" || !slices.Contains(g.Items, deep) || !tr.Found {
		t.Errorf("mimics %v, trap found %v: want the shallow one and the trap shown", ms, tr.Found)
	}
}

// TestMimicCarried: a deep mimic is picked up and carried; using it
// shows it, with a strike; identified, it is listed as the monster, and
// putting it down shows it all the same.
func TestMimicCarried(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Monsters = nil
	g.HP, g.MaxHP = 999, 999
	sure(g)
	it := hidden(g, "bake3", "yakusou", Point{r.X + 1, r.Y})
	move(t, g, "right")
	if len(g.Inventory) != 1 || len(mimics(g)) != 0 {
		t.Fatalf("inventory %v, %d mimics: want it carried", g.Inventory, len(mimics(g)))
	}
	hp := g.HP
	command(t, g, it, commandDrink)
	if len(g.Inventory) != 0 || len(mimics(g)) != 1 || g.HP >= hp || !strings.Contains(g.Message, "薬草は大化け物だった！") {
		t.Errorf("inventory %v, %d mimics, HP %d/%d, %q", g.Inventory, len(mimics(g)), g.HP, hp, g.Message)
	}

	// Identified: listed as the monster, and no longer to identify.
	g, r = roomGame(t, 8, 3)
	g.Monsters = nil
	g.HP, g.MaxHP = 999, 999
	it = carry(g, "yakusou", 0)
	it.Mimic = kind("bake4")
	g.identified["yakusou"] = false
	use(t, g, "shikibetsu")
	if g.Prompt == nil || len(g.Prompt.Choices) != 1 {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if !it.Unmasked || g.Entry(it) != "化   化け主" || g.unknown(it) || g.identified["yakusou"] {
		t.Errorf("unmasked %v, entry %q", it.Unmasked, g.Entry(it))
	}
	command(t, g, it, commandDrop)
	if len(g.Inventory) != 0 || len(g.Items) != 0 || len(mimics(g)) != 1 || !strings.Contains(g.Message, "化け主が正体を現した！") {
		t.Errorf("inventory %v, items %v, %d mimics, %q", g.Inventory, g.Items, len(mimics(g)), g.Message)
	}
	_ = r
}

// TestMimicNextFloor: a mimic carried to the next floor shows itself
// there.
func TestMimicNextFloor(t *testing.T) {
	g, _ := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 999, 999
	it := carry(g, "yakusou", 0)
	it.Mimic = kind("bake3")
	g.descend()
	var shown *Monster
	for _, m := range mimics(g) {
		if m.loot == it {
			shown = m
		}
	}
	if shown == nil || slices.Contains(g.Inventory, it) || chebyshev(shown.Pos, g.Player) != 1 {
		t.Errorf("inventory %v, shown %v: want the mimic next to the player", g.Inventory, shown)
	}
}

// TestMimicStairs: a mimic that looks like the stairs is drawn as
// stairs, and shows itself when the player steps on it.
func TestMimicStairs(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Monsters = nil
	g.HP, g.MaxHP = 999, 999
	sure(g)
	p := Point{r.X + 1, r.Y}
	g.Decoys = []Decoy{{Pos: p, Def: kind("bake3")}}
	if glyph, _ := g.glyph(p); glyph != glyphStairs {
		t.Errorf("the decoy is drawn as %q", glyph)
	}
	depth, hp := g.Depth, g.HP
	move(t, g, "right")
	if len(g.Decoys) != 0 || len(mimics(g)) != 1 || g.Depth != depth || g.HP >= hp || !strings.Contains(g.Message, "階段は大化け物だった！") {
		t.Errorf("decoys %v, %d mimics, HP %d/%d, %q", g.Decoys, len(mimics(g)), g.HP, hp, g.Message)
	}
}

// TestMimicsHide: on the floors of the cave the mimics hide: none walks
// about on a new floor, and some are goods of a shop (the cave has no
// mimics of ranks 3 and 4, which may be stairs: TestMimicHidesAsStairs).
func TestMimicsHide(t *testing.T) {
	d := cave()
	items, decoys, goods := 0, 0, 0
	for seed := range 30 {
		g := NewGame(d, "mimic"+string(rune('a'+seed)))
		for g.Depth < d.Floors {
			g.descend()
			if ms := mimics(g); len(ms) != 0 {
				t.Fatalf("seed %d, %dF: %d mimics walk about", seed, g.Depth, len(ms))
			}
			decoys += len(g.Decoys)
			for _, dc := range g.Decoys {
				if dc.Pos == g.Stairs || !g.Level.At(dc.Pos).Walkable() {
					t.Fatalf("a decoy at %v (the stairs at %v)", dc.Pos, g.Stairs)
				}
			}
			for _, it := range g.Items {
				if it.Mimic == nil {
					continue
				}
				items++
				if it.ForSale {
					goods++
				}
				if c := it.Def.Category; !it.shallow() && (c == CategoryMoney || c == CategoryArrow) {
					t.Fatalf("a deep mimic looks like %s", it.Def.Name)
				}
			}
		}
	}
	if items == 0 || decoys != 0 || goods == 0 {
		t.Errorf("%d items, %d decoys, %d goods: want items and goods, no decoy", items, decoys, goods)
	}
}

// TestMimicHidesAsStairs: a deep mimic hides as stairs by its chance,
// away from the real ones, and as an item that is no money nor arrows.
func TestMimicHidesAsStairs(t *testing.T) {
	g := NewGame(cave(), "decoy")
	rooms := make([]int, len(g.Level.Rooms))
	for i := range rooms {
		rooms[i] = i
	}
	def := kind("bake3")
	always := Ability{Name: AbilityMimic, Values: []int{mimicDeep, 100, 0}}
	if !g.hide(g.rng, rooms, def, g.Stairs, always) || len(g.Decoys) != 1 {
		t.Fatalf("decoys %v", g.Decoys)
	}
	if p := g.Decoys[0].Pos; p == g.Stairs || !g.Level.At(p).Walkable() || g.Level.RoomAt(p) == g.Level.RoomAt(g.Player) {
		t.Errorf("the decoy at %v (the stairs %v, the player %v)", p, g.Stairs, g.Player)
	}
	never := Ability{Name: AbilityMimic, Values: []int{mimicDeep, 0, 0}}
	for range 200 {
		n := len(g.Items)
		if !g.hide(g.rng, rooms, def, g.Stairs, never) {
			continue
		}
		it := g.Items[n]
		if c := it.Def.Category; it.Mimic != def || c == CategoryMoney || c == CategoryArrow {
			t.Fatalf("a deep mimic looks like %s", it.Def.Name)
		}
		g.Items = g.Items[:n]
	}
}
