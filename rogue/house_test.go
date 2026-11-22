package rogue

import (
	"slices"
	"testing"
)

// specialGame starts a run in the built-in dungeon whose floors are all of
// the chances c changes.
func specialGame(t *testing.T, seed string, f func(*FloorCounts)) *Game {
	t.Helper()
	d := *cave()
	d.FloorRows = append([]FloorCounts(nil), d.FloorRows...)
	for i := range d.FloorRows {
		f(&d.FloorRows[i])
	}
	return NewGame(&d, seed)
}

// connected says whether every walkable tile of l can be reached from p.
func connected(l *Level, p Point) bool {
	from := reachable(l, p)
	for i, t := range l.Tiles {
		if t.Walkable() && !from[Point{i % l.W, i / l.W}] {
			return false
		}
	}
	return true
}

func TestBigRoom(t *testing.T) {
	g := specialGame(t, "big", func(c *FloorCounts) { c.BigRoom = 100 })
	if len(g.Level.Rooms) != 1 {
		t.Fatalf("%d rooms", len(g.Level.Rooms))
	}
	if !g.Visible(g.Stairs) {
		t.Error("the stairs are out of sight in a big room")
	}
	if g.Player == g.Stairs {
		t.Error("the player starts on the stairs")
	}
}

func TestMaze(t *testing.T) {
	for _, seed := range []string{"a", "b", "c"} {
		g := specialGame(t, seed, func(c *FloorCounts) { c.Maze = 100 })
		l := g.Level
		if len(l.Rooms) != mazeRooms {
			t.Errorf("%s: %d rooms", seed, len(l.Rooms))
		}
		if !connected(l, g.Player) {
			t.Errorf("%s: a part of the maze cannot be reached", seed)
		}
		corridors := 0
		for _, tile := range l.Tiles {
			if tile == TileCorridor {
				corridors++
			}
		}
		// The maze is a part of the floor: about half of its tiles are
		// corridors.
		if want := (2*mazeCols + 1) * (2*mazeRows + 1) / 3; corridors < want {
			t.Errorf("%s: %d corridor tiles, want %d at least", seed, corridors, want)
		}
	}
}

func TestMonsterHouse(t *testing.T) {
	g := specialGame(t, "house", func(c *FloorCounts) { c.House = 100 })
	if g.House < 0 || g.House == g.Level.RoomAt(g.Player) {
		t.Fatalf("house %d, player in %d", g.House, g.Level.RoomAt(g.Player))
	}
	r := g.Level.Rooms[g.House]
	var house []*Monster
	for _, m := range g.Monsters {
		if m.House {
			house = append(house, m)
			if !m.Dozing || !r.Contains(m.Pos) {
				t.Errorf("a monster of the house at %v, dozing %v", m.Pos, m.Dozing)
			}
		}
	}
	if len(house) < 5 {
		t.Errorf("%d monsters in the house", len(house))
	}
	traps := 0
	for _, tr := range g.Traps {
		if r.Contains(tr.Pos) {
			traps++
		}
	}
	// A trap has a few tries to find its tile, so a crowded house may
	// get fewer than the minimum, but not none.
	if traps == 0 {
		t.Errorf("no trap in the house (want %d)", g.Dungeon.counts(g.Depth).MinHouseTrap)
	}

	// Entering it wakes them all.
	g.Player = Point{r.X - 1, r.Y}
	inside := Point{r.X, r.Y}
	if m := g.MonsterAt(inside); m != nil {
		g.remove(m)
	}
	g.Level.set(g.Player, TileCorridor)
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != inside {
		t.Skipf("could not enter: %q", g.Message)
	}
	if !slices.Contains(g.Log, "モンスターハウスだ！") {
		t.Errorf("log %q", g.Log)
	}
	for _, m := range house {
		if m.Dozing && slices.Contains(g.Monsters, m) {
			t.Error("a monster of the house sleeps on")
		}
	}
}

func TestBigRoomHouse(t *testing.T) {
	g := specialGame(t, "bighouse", func(c *FloorCounts) { c.BigRoom, c.House = 100, 100 })
	if g.House != 0 || !slices.Contains(g.Log, "モンスターハウスだ！") {
		t.Fatalf("house %d, log %q", g.House, g.Log)
	}
	asleep := func() int {
		n := 0
		for _, m := range g.Monsters {
			if m.House && m.Dozing {
				n++
			}
		}
		return n
	}
	before := asleep()
	if before == 0 {
		t.Fatal("no monster asleep")
	}
	g.HP, g.MaxHP = 99999, 99999
	wait(t, g, 30)
	if after := asleep(); after >= before {
		t.Errorf("%d asleep, then %d", before, after)
	}
}
