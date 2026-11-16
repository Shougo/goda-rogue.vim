package rogue

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// reachable returns the walkable tiles reachable from p.
func reachable(l *Level, p Point) map[Point]bool {
	seen := map[Point]bool{p: true}
	queue := []Point{p}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := q.Add(d)
			if l.At(n).Walkable() && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

func TestGenerateConnected(t *testing.T) {
	for seed := range 200 {
		g := NewGame(fmt.Sprint(seed))
		l := g.Level
		if len(l.Rooms) != gridCols*gridRows {
			t.Fatalf("seed %d: %d rooms", seed, len(l.Rooms))
		}
		from := reachable(l, g.Player)
		for i, tile := range l.Tiles {
			p := Point{i % l.W, i / l.W}
			if tile.Walkable() && !from[p] {
				t.Fatalf("seed %d: %v not reachable from the player %v", seed, p, g.Player)
			}
			// The edges of the floor stay rock.
			if tile.Walkable() && (p.X == 0 || p.Y == 0 || p.X == l.W-1 || p.Y == l.H-1) {
				t.Fatalf("seed %d: %v on the edge is walkable", seed, p)
			}
		}
		if !from[g.Stairs] {
			t.Fatalf("seed %d: the stairs %v are not reachable", seed, g.Stairs)
		}
		for _, m := range g.Monsters {
			if !l.At(m.Pos).Walkable() || m.Pos == g.Player {
				t.Fatalf("seed %d: monster at %v", seed, m.Pos)
			}
		}
	}
}

// TestSeed: the same seed makes the same floors, whatever the player did
// on the floors before.
func TestSeed(t *testing.T) {
	a, b := NewGame("vim"), NewGame("vim")
	if !slices.Equal(a.Level.Tiles, b.Level.Tiles) || a.Player != b.Player || a.Stairs != b.Stairs {
		t.Fatal("the same seed made different first floors")
	}
	for range 30 {
		for _, d := range []string{"right", "down", "left", "up"} {
			if err := a.Do("move", []string{d}); err != nil {
				t.Fatal(err)
			}
		}
	}
	a.descend()
	b.descend()
	if !slices.Equal(a.Level.Tiles, b.Level.Tiles) || a.Stairs != b.Stairs {
		t.Fatal("the same seed made different second floors")
	}
	if c := NewGame("emacs"); slices.Equal(a.Level.Tiles, c.Level.Tiles) {
		t.Fatal("different seeds made the same floor")
	}
}

func TestRender(t *testing.T) {
	g := NewGame("render")
	for _, size := range []Size{{80, 24}, {200, 60}, {7, 3}} {
		s := g.Render(size)
		maps := s.Lines[:len(s.Lines)-statusLines]
		wantW := min(max(size.Width/2, 1), MapWidth)
		wantH := min(max(size.Height-statusLines, 1), MapHeight)
		if len(maps) != wantH {
			t.Errorf("%v: %d lines, want %d", size, len(maps), wantH)
		}
		for i, line := range maps {
			if n := utf8.RuneCountInString(line); n != wantW || len(line) != wantW*glyphBytes {
				t.Errorf("%v: line %d has %d tiles (%d bytes), want %d", size, i+1, n, len(line), wantW)
			}
		}
		c := s.Cursor
		if c[0] < 1 || c[0] > len(maps) || !strings.HasPrefix(maps[c[0]-1][c[1]-1:], glyphPlayer) {
			t.Errorf("%v: the cursor %v is not on the player", size, c)
		}
		for group, spans := range s.Highlights {
			for _, sp := range spans {
				line := maps[sp.Line-1]
				if sp.Col < 1 || sp.End > len(line)+1 || (sp.Col-1)%glyphBytes != 0 || (sp.End-sp.Col)%glyphBytes != 0 {
					t.Errorf("%v: %s span %v off the glyphs", size, group, sp)
				}
			}
		}
		if got := s.Highlights[hlPlayer]; len(got) != 1 || got[0].Line != c[0] || got[0].Col != c[1] {
			t.Errorf("%v: player highlight %v, cursor %v", size, got, c)
		}
	}
}

// find returns a game of some seed for which ok holds.
func find(t *testing.T, ok func(*Game) bool) *Game {
	t.Helper()
	for seed := range 1000 {
		g := NewGame(fmt.Sprint(seed))
		if ok(g) {
			return g
		}
	}
	t.Fatal("no such game")
	return nil
}

func TestMoveIntoRock(t *testing.T) {
	g := find(t, func(g *Game) bool { return !g.Level.At(g.Player.Add(Point{0, -1})).Walkable() })
	g.Monsters = nil
	before := g.Player
	if err := g.Do("move", []string{"up"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != before || g.Turn != 0 || g.Facing != "up" {
		t.Errorf("moved into rock: %v, turn %d, facing %s", g.Player, g.Turn, g.Facing)
	}
}

func TestTurnCostsNothing(t *testing.T) {
	g := NewGame("turn")
	if err := g.Do("turn", []string{"upleft"}); err != nil {
		t.Fatal(err)
	}
	if g.Turn != 0 || g.Facing != "upleft" {
		t.Errorf("turn %d, facing %s", g.Turn, g.Facing)
	}
	if err := g.Do("turn", []string{"nowhere"}); err == nil {
		t.Error("an unknown direction was accepted")
	}
}

func TestSwapWithAlly(t *testing.T) {
	g := NewGame("ally")
	var ally *Monster
	for _, m := range g.Monsters {
		if m.Ally {
			ally = m
		}
	}
	if ally == nil {
		t.Fatal("no ally")
	}
	d := Point{ally.Pos.X - g.Player.X, ally.Pos.Y - g.Player.Y}
	var name string
	for n, p := range Directions {
		if p == d {
			name = n
		}
	}
	from, to := g.Player, ally.Pos
	if !g.canStep(d) {
		t.Skip("the ally is around a corner")
	}
	if err := g.Do("move", []string{name}); err != nil {
		t.Fatal(err)
	}
	if g.Player != to || ally.Pos != from {
		t.Errorf("player %v, ally %v; want %v, %v", g.Player, ally.Pos, to, from)
	}
}

func TestStairs(t *testing.T) {
	g := NewGame("stairs")
	if err := g.Do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 1 {
		t.Fatal("went down without stairs")
	}
	g.Player = g.Stairs
	if err := g.Do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 2 {
		t.Errorf("depth %d", g.Depth)
	}
}

func TestDash(t *testing.T) {
	g := NewGame("dash")
	g.Monsters = nil
	g.Items = nil
	g.Stairs = Point{-1, -1}
	i := g.Level.RoomAt(g.Player)
	r := g.Level.Rooms[i]
	g.Player = Point{r.X, r.Y}
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	// Stops at the wall, or where it leaves the room by a doorway.
	if g.Player.X != r.X+r.W-1 && g.Level.RoomAt(g.Player) == i {
		t.Errorf("dash stopped at %v in the room %+v", g.Player, r)
	}
}

// TestCorridorsAwayFromWalls: next to the wall of a room, a corridor is
// only right outside a doorway, so rooms do not look merged.
func TestCorridorsAwayFromWalls(t *testing.T) {
	for seed := range 200 {
		l := NewGame(fmt.Sprint(seed)).Level
		for _, r := range l.Rooms {
			if r.Dummy {
				continue
			}
			// The tiles one outside the wall ring.
			for y := r.Y - 2; y <= r.Y+r.H+1; y++ {
				for x := r.X - 2; x <= r.X+r.W+1; x++ {
					p := Point{x, y}
					if r.Around(p) || l.At(p) != TileCorridor {
						continue
					}
					// The wall tile next to it, toward the room.
					w := Point{min(max(x, r.X-1), r.X+r.W), min(max(y, r.Y-1), r.Y+r.H)}
					corner := (x < r.X-1 || x > r.X+r.W) && (y < r.Y-1 || y > r.Y+r.H)
					if corner || l.At(w) != TileCorridor {
						t.Fatalf("seed %d: corridor %v along the wall of %+v", seed, p, r)
					}
				}
			}
		}
	}
}

func TestSafe(t *testing.T) {
	g := NewGame("safe")
	start := g.Player
	if !g.IsSafe(start) {
		t.Fatal("the start tile is not safe")
	}
	for _, d := range []string{"left", "right", "up", "down", "upleft", "upright", "downleft", "downright"} {
		if err := g.Do("move", []string{d}); err != nil {
			t.Fatal(err)
		}
		if g.Player != start {
			break
		}
	}
	if g.Player == start {
		t.Skip("could not move")
	}
	if !g.IsSafe(g.Player) || !g.IsSafe(start) {
		t.Error("walked tiles are not safe")
	}
	n := 0
	for _, s := range g.Safe {
		if s {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d safe tiles, want 2", n)
	}
	if _, group := g.glyph(start); group != hlSafe {
		t.Errorf("the start tile shows as %s", group)
	}
	g.descend()
	if n := len(slices.DeleteFunc(slices.Clone(g.Safe), func(s bool) bool { return !s })); n != 1 {
		t.Errorf("%d safe tiles on a new floor, want 1", n)
	}
}
