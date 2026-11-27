package rogue

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// waterGame is a room with a column of water across it at x = r.X+3.
func waterGame(t *testing.T) (*Game, Room) {
	g, r := trapGame(t)
	for y := r.Y; y < r.Y+r.H; y++ {
		g.Level.set(Point{r.X + 3, y}, TileWater)
	}
	g.Player = Point{r.X + 2, r.Y + 1}
	g.Facing = "right"
	return g, r
}

// TestWaterWalk: the player crosses water only with a bracelet of 水蜘蛛;
// taking it off there, it falls in and is washed away onto the floor.
func TestWaterWalk(t *testing.T) {
	g, r := waterGame(t)
	if g.step(Directions["right"], false) {
		t.Fatal("into the water without the bracelet")
	}
	b := wear(g, "mizugumo")
	if !g.step(Directions["right"], false) || g.Player != (Point{r.X + 3, r.Y + 1}) {
		t.Fatalf("not on the water: %v", g.Player)
	}
	g.overboard()
	if g.Level.At(g.Player) != TileWater {
		t.Fatal("washed away with the bracelet on")
	}
	g.unequip(b)
	g.overboard()
	if !g.Level.At(g.Player).Walkable() {
		t.Errorf("still in the water at %v", g.Player)
	}
}

// TestDrain: the scroll of 水がれ dries the waterways up.
func TestDrain(t *testing.T) {
	g, r := waterGame(t)
	if !g.drain() || g.Level.At(Point{r.X + 3, r.Y}) != TileRoom || g.drain() {
		t.Error("the water stays, or dries twice")
	}
}

// TestWaterThrow: what is thrown flies over water; what falls in sinks.
func TestWaterThrow(t *testing.T) {
	g, r := waterGame(t)
	m := place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 1})
	m.HP, m.MaxHP = 999, 999
	g.Dungeon.Rules.PlayerThrowRate = 100
	it := carry(g, "kitate", 0)
	g.throwItem(it)
	if m.HP == 999 {
		t.Error("the item did not fly over the water")
	}
	n := len(g.Items)
	g.land(carry(g, "yakusou", 0), Point{r.X + 3, r.Y + 2})
	if len(g.Items) != n {
		t.Error("an item on the water")
	}
}

// TestWaterMonsters: one that swims or floats crosses water, the others
// go around it, or do not.
func TestWaterMonsters(t *testing.T) {
	g, r := waterGame(t)
	g.Player = Point{r.X + 5, r.Y + 1}
	for _, c := range []struct {
		id   string
		swim bool
	}{{"kani1", true}, {"rei1", true}, {"nezumi1", false}} {
		m := place(g, kind(c.id), Point{r.X + 2, r.Y + 1})
		if m.swims() != c.swim {
			t.Errorf("%s swims %v", c.id, m.swims())
		}
		for range 3 {
			g.stepToward(m, g.Player)
			if g.Level.At(m.Pos) == TileWater && !c.swim {
				t.Fatalf("%s on the water", c.id)
			}
		}
		if c.swim && m.Pos.X <= r.X+2 {
			t.Errorf("%s did not cross: %v", c.id, m.Pos)
		}
		g.remove(m)
	}
}

// TestFlood: a pond or an island keeps the doors of its room joined; an
// island holds its items, and nothing else is put on it.
func TestFlood(t *testing.T) {
	islands := 0
	for seed := range 300 {
		rng := rand.New(rand.NewPCG(uint64(seed), 1))
		l := Generate(rng)
		l.flood(rng)
		// Every floor tile reaches every other, the islands left out.
		var start Point
		total := 0
		for i, tl := range l.Tiles {
			p := Point{i % l.W, i / l.W}
			if tl.Walkable() && !l.Island[p] {
				start = p
				total++
			}
		}
		seen := map[Point]bool{start: true}
		queue := []Point{start}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, d := range around {
				q := p.Add(d)
				if !seen[q] && l.At(q).Walkable() && !l.Island[q] && (d.X == 0 || d.Y == 0 || l.At(p.Add(Point{d.X, 0})).Open() && l.At(p.Add(Point{0, d.Y})).Open()) {
					seen[q] = true
					queue = append(queue, q)
				}
			}
		}
		if len(seen) != total {
			t.Fatalf("seed %d: %d of %d tiles joined", seed, len(seen), total)
		}
		if len(l.Isles) > 0 {
			islands++
			if len(l.Isles) < islandItems {
				t.Fatalf("seed %d: an island of %d tiles", seed, len(l.Isles))
			}
			for _, p := range l.Isles {
				for _, d := range around {
					if q := p.Add(d); !l.Island[q] && l.At(q) != TileWater {
						t.Fatalf("seed %d: the island at %v open at %v", seed, p, q)
					}
				}
			}
		}
	}
	if islands == 0 {
		t.Error("no island in 300 floors")
	}
}

// TestIslandItems: a floor with an island has its items on it, and no
// stairs, monster or trap.
func TestIslandItems(t *testing.T) {
	d := cave()
	found := 0
	for i := range 200 {
		g := NewGame(d, fmt.Sprint("isle", i))
		for g.Depth < d.Floors && found == 0 {
			g.DebugMode = true
			if err := g.Do("debug", []string{debugFloor, fmt.Sprint(g.Depth + 1)}); err != nil {
				t.Fatal(err)
			}
			if len(g.Level.Isles) == 0 {
				continue
			}
			found++
			on := 0
			for _, it := range g.Items {
				if g.Level.Island[it.Pos] {
					on++
				}
			}
			if on != islandItems {
				t.Errorf("%d items on the island", on)
			}
			if g.Level.Island[g.Stairs] || g.Level.Island[g.Player] {
				t.Error("the stairs or the player on the island")
			}
			for _, m := range g.Monsters {
				if g.Level.Island[m.Pos] {
					t.Errorf("%s on the island", m.Name)
				}
			}
			for _, tr := range g.Traps {
				if g.Level.Island[tr.Pos] {
					t.Error("a trap on the island")
				}
			}
		}
		if found > 0 {
			break
		}
	}
	if found == 0 {
		t.Error("no island")
	}
}

// TestFlungIntoWater: blown into the water, the player (without the
// bracelet) and a monster that does not swim are washed away onto the
// floor; over it, they fly on.
func TestFlungIntoWater(t *testing.T) {
	g, r := waterGame(t)
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	// One beyond the water stops the flight on it.
	block := place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 1})
	g.flingPlayer(Directions["right"], m)
	if !g.Level.At(g.Player).Walkable() {
		t.Errorf("the player in the water at %v", g.Player)
	}
	g.remove(block)
	g.Player = Point{r.X, r.Y}
	m.Pos = Point{r.X + 2, r.Y + 2}
	place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 2})
	m.HP, m.MaxHP = 999, 999
	g.fling(m, Directions["right"], true)
	if !g.Level.At(m.Pos).Walkable() {
		t.Errorf("the monster in the water at %v", m.Pos)
	}
}

// TestFloatBracelet: floating, the player crosses water and sets no trap
// off by walking.
func TestFloatBracelet(t *testing.T) {
	g, r := waterGame(t)
	wear(g, "fuyuu")
	if !g.step(Directions["right"], false) {
		t.Fatal("not over the water")
	}
	tr := trap(g, "otoshiana", Point{r.X + 4, r.Y + 1})
	if !g.step(Directions["right"], false) || g.Depth != 1 || g.Player != tr.Pos {
		t.Errorf("at %v on %dF: the pit went off", g.Player, g.Depth)
	}
}

// TestWallsBracelet: in the rock with a bracelet of 壁抜け, 1 HP a turn
// (never the last); taken off there, the player is pushed out.
func TestWallsBracelet(t *testing.T) {
	g, r := trapGame(t)
	// (A tile of the top row with rock above, not a door.)
	g.Player = Point{r.X, r.Y}
	for g.Level.At(g.Player.Add(Directions["up"])) != TileRock {
		g.Player.X++
	}
	g.HP = 2
	if g.step(Directions["up"], false) {
		t.Fatal("into the rock without the bracelet")
	}
	b := wear(g, "kabenuke")
	if !g.step(Directions["up"], false) || g.Level.At(g.Player) != TileRock {
		t.Fatalf("not in the rock: %v", g.Player)
	}
	g.inWalls()
	g.inWalls()
	if g.HP != 1 {
		t.Errorf("HP %d", g.HP)
	}
	g.unequip(b)
	g.overboard()
	if !g.Level.At(g.Player).Walkable() {
		t.Errorf("still in the rock at %v", g.Player)
	}
}

// TestCursePowerBracelet: each cursed item carried adds to the strength,
// not those in pots.
func TestCursePowerBracelet(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	s := g.CurrentStrength()
	b := wear(g, "noroiryoku")
	if g.CurrentStrength() != s {
		t.Error("strength without curses")
	}
	b.Cursed = true
	carry(g, "yakusou", 0).Cursed = true
	pot := carry(g, "hozon", 0)
	pot.Contents = append(pot.Contents, &Item{Def: g.Dungeon.Item("yakusou"), Cursed: true})
	if g.CurrentStrength() != s+2 {
		t.Errorf("strength %d, want %d", g.CurrentStrength(), s+2)
	}
}

// TestChargeOverWater: a boar does not rush over water to the player; a
// crab (aquatic) would.
func TestChargeOverWater(t *testing.T) {
	g, r := waterGame(t)
	g.Player = Point{r.X + 5, r.Y + 1}
	m := place(g, kind("inoshishi2"), Point{r.X + 1, r.Y + 1})
	a, _ := m.ability(AbilityCharge)
	if g.charge(m, a) || m.Pos != (Point{r.X + 1, r.Y + 1}) {
		t.Errorf("charged over the water to %v", m.Pos)
	}
}
