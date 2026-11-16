package rogue

import (
	"math/rand/v2"
)

// The size of a floor in tiles, and its 3x3 sections, each holding a room
// or a dummy (a corridor junction), as in rogue and Shiren.
const (
	MapWidth  = 63
	MapHeight = 33
	gridCols  = 3
	gridRows  = 3
	secWidth  = MapWidth / gridCols
	secHeight = MapHeight / gridRows
)

// Tile is what a tile of a floor is made of.
type Tile uint8

const (
	// TileRock is the solid rock between the rooms; shown as a wall next
	// to a room or a corridor.
	TileRock Tile = iota
	// TileRoom is the floor of a room.
	TileRoom
	// TileCorridor is the floor of a corridor (doorways included).
	TileCorridor
)

// Walkable says whether a creature can stand on t.
func (t Tile) Walkable() bool { return t == TileRoom || t == TileCorridor }

// Point is a tile position.
type Point struct{ X, Y int }

// Add returns p moved by d.
func (p Point) Add(d Point) Point { return Point{p.X + d.X, p.Y + d.Y} }

// Room is the interior of a room. A dummy is a single corridor tile where
// corridors meet.
type Room struct {
	X, Y, W, H int
	Dummy      bool
}

// Contains says whether p is in the interior of r.
func (r Room) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

// Around says whether p is in r or in the wall ring around it.
func (r Room) Around(p Point) bool {
	return p.X >= r.X-1 && p.X <= r.X+r.W && p.Y >= r.Y-1 && p.Y <= r.Y+r.H
}

// Level is the layout of one floor.
type Level struct {
	W, H  int
	Tiles []Tile
	Rooms []Room
}

// At returns the tile at p; outside the floor, rock.
func (l *Level) At(p Point) Tile {
	if p.X < 0 || p.Y < 0 || p.X >= l.W || p.Y >= l.H {
		return TileRock
	}
	return l.Tiles[p.Y*l.W+p.X]
}

func (l *Level) set(p Point, t Tile) { l.Tiles[p.Y*l.W+p.X] = t }

// RoomAt returns the index of the room (not a dummy) whose interior holds
// p, or -1.
func (l *Level) RoomAt(p Point) int {
	for i, r := range l.Rooms {
		if !r.Dummy && r.Contains(p) {
			return i
		}
	}
	return -1
}

// Generate makes the layout of a floor from rng: a room or a dummy in each
// of the 3x3 sections, connected by corridors along a random spanning tree
// of the sections plus a few loops.
func Generate(rng *rand.Rand) *Level {
	l := &Level{W: MapWidth, H: MapHeight, Tiles: make([]Tile, MapWidth*MapHeight)}

	// Up to two dummies, never more: a floor needs rooms.
	dummies := map[int]bool{}
	for range rng.IntN(3) {
		dummies[rng.IntN(gridCols*gridRows)] = true
	}

	for i := range gridCols * gridRows {
		ox, oy := i%gridCols*secWidth, i/gridCols*secHeight
		// The interior keeps two tiles from the edges of its section: its
		// wall, and room for a corridor between it and its neighbor's.
		minX, maxX := ox+2, ox+secWidth-3
		minY, maxY := oy+2, oy+secHeight-3
		if dummies[i] {
			p := Point{between(rng, minX, maxX), between(rng, minY, maxY)}
			l.Rooms = append(l.Rooms, Room{X: p.X, Y: p.Y, W: 1, H: 1, Dummy: true})
			l.set(p, TileCorridor)
			continue
		}
		w := between(rng, 4, maxX-minX+1)
		h := between(rng, 3, maxY-minY+1)
		r := Room{X: between(rng, minX, maxX-w+1), Y: between(rng, minY, maxY-h+1), W: w, H: h}
		l.Rooms = append(l.Rooms, r)
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				l.set(Point{x, y}, TileRoom)
			}
		}
	}

	for _, e := range connections(rng) {
		l.connect(rng, e[0], e[1])
	}
	return l
}

// between returns a random integer in [lo, hi].
func between(rng *rand.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + rng.IntN(hi-lo+1)
}

// connections returns the pairs of sections to connect, the first of each
// left of or above the second: a random spanning tree of the grid, and
// each other neighboring pair with a chance of one in four.
func connections(rng *rand.Rand) [][2]int {
	var all [][2]int
	for i := range gridCols * gridRows {
		if i%gridCols < gridCols-1 {
			all = append(all, [2]int{i, i + 1})
		}
		if i/gridCols < gridRows-1 {
			all = append(all, [2]int{i, i + gridCols})
		}
	}
	in := make([]bool, gridCols*gridRows)
	in[rng.IntN(len(in))] = true
	used := make([]bool, len(all))
	var out [][2]int
	for n := 1; n < len(in); n++ {
		// The edges from the tree to a section not in it yet.
		var frontier []int
		for j, e := range all {
			if in[e[0]] != in[e[1]] {
				frontier = append(frontier, j)
			}
		}
		j := frontier[rng.IntN(len(frontier))]
		used[j] = true
		in[all[j][0]], in[all[j][1]] = true, true
		out = append(out, all[j])
	}
	for j, e := range all {
		if !used[j] && rng.IntN(4) == 0 {
			out = append(out, e)
		}
	}
	return out
}

// connect digs a corridor from the section a to b (right of or below it).
func (l *Level) connect(rng *rand.Rand, a, b int) {
	ra, rb := l.Rooms[a], l.Rooms[b]
	if b == a+1 {
		p, q := door(rng, ra, Point{1, 0}), door(rng, rb, Point{-1, 0})
		mid := between(rng, p.X+1, q.X-1)
		l.dig(p, Point{mid, p.Y})
		l.dig(Point{mid, p.Y}, Point{mid, q.Y})
		l.dig(Point{mid, q.Y}, q)
		return
	}
	p, q := door(rng, ra, Point{0, 1}), door(rng, rb, Point{0, -1})
	mid := between(rng, p.Y+1, q.Y-1)
	l.dig(p, Point{p.X, mid})
	l.dig(Point{p.X, mid}, Point{q.X, mid})
	l.dig(Point{q.X, mid}, q)
}

// door returns where a corridor leaves r on its side d: a tile of its wall,
// or the dummy itself.
func door(rng *rand.Rand, r Room, d Point) Point {
	switch {
	case r.Dummy:
		return Point{r.X, r.Y}
	case d.X > 0:
		return Point{r.X + r.W, between(rng, r.Y, r.Y+r.H-1)}
	case d.X < 0:
		return Point{r.X - 1, between(rng, r.Y, r.Y+r.H-1)}
	case d.Y > 0:
		return Point{between(rng, r.X, r.X+r.W-1), r.Y + r.H}
	default:
		return Point{between(rng, r.X, r.X+r.W-1), r.Y - 1}
	}
}

// dig makes the rock on the straight line from p to q a corridor.
func (l *Level) dig(p, q Point) {
	d := Point{sign(q.X - p.X), sign(q.Y - p.Y)}
	for {
		if l.At(p) == TileRock {
			l.set(p, TileCorridor)
		}
		if p == q {
			return
		}
		p = p.Add(d)
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}
