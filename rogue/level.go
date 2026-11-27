package rogue

import (
	"math/rand/v2"
	"slices"
)

// The size of a floor in tiles, and its 3x3 sections, each holding a room
// or a dummy (a corridor junction), as in rogue and Shiren.
const (
	MapWidth  = 66
	MapHeight = 36
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
	// TileWater is a waterway: no one walks there but swimmers and
	// fliers (monsters.txt 属性) and the player with a bracelet of 水渡り;
	// what flies passes over it, what falls in sinks.
	TileWater
)

// Walkable says whether a creature can stand on t (on foot: not water).
func (t Tile) Walkable() bool { return t == TileRoom || t == TileCorridor }

// Open says whether t lets what flies (and the sight) through: a floor,
// or water.
func (t Tile) Open() bool { return t.Walkable() || t == TileWater }

// Point is a tile position.
type Point struct{ X, Y int }

// Add returns p moved by d.
func (p Point) Add(d Point) Point { return Point{p.X + d.X, p.Y + d.Y} }

// Sub returns the step from q to p: p - q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

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
	// Maze says whether it is a maze: things lie anywhere in it, not only
	// in its rooms.
	Maze bool
	// Island marks the tiles of islands, the floor in a ring of water
	// (flood): nothing is put there but their own items (Isles).
	Island map[Point]bool
	// Isles are the tiles of the islands where their items go.
	Isles []Point
	// version counts the changes of its tiles (set), for what is worked
	// out from them (Game.distances).
	version int
}

// At returns the tile at p; outside the floor, rock.
func (l *Level) At(p Point) Tile {
	if p.X < 0 || p.Y < 0 || p.X >= l.W || p.Y >= l.H {
		return TileRock
	}
	return l.Tiles[p.Y*l.W+p.X]
}

func (l *Level) set(p Point, t Tile) {
	l.Tiles[p.Y*l.W+p.X] = t
	l.version++
}

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
		// The interior keeps three tiles from the edges of its section:
		// its wall, and room for a corridor that turns one tile away from
		// the walls (see connect).
		minX, maxX := ox+3, ox+secWidth-4
		minY, maxY := oy+3, oy+secHeight-4
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
// It leaves a door straight, and turns at least two tiles away from it, so
// that it never runs along a wall: rooms do not look merged.
func (l *Level) connect(rng *rand.Rand, a, b int) {
	ra, rb := l.Rooms[a], l.Rooms[b]
	if b == a+1 {
		p, q := door(rng, ra, Point{1, 0}), door(rng, rb, Point{-1, 0})
		mid := between(rng, p.X+2, q.X-2)
		l.dig(p, Point{mid, p.Y})
		l.dig(Point{mid, p.Y}, Point{mid, q.Y})
		l.dig(Point{mid, q.Y}, q)
		return
	}
	p, q := door(rng, ra, Point{0, 1}), door(rng, rb, Point{0, -1})
	mid := between(rng, p.Y+2, q.Y-2)
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

// GenerateBigRoom makes a floor of one big room.
func GenerateBigRoom() *Level {
	l := &Level{W: MapWidth, H: MapHeight, Tiles: make([]Tile, MapWidth*MapHeight)}
	r := Room{X: 3, Y: 3, W: MapWidth - 6, H: MapHeight - 6}
	l.Rooms = append(l.Rooms, r)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			l.set(Point{x, y}, TileRoom)
		}
	}
	return l
}

// mazeRooms is how many small rooms a maze has, where things are put.
const mazeRooms = 4

// mazeCols and mazeRows are the size of a maze, in cells (a cell and the
// wall after it are 2 tiles): a part of the floor, not all of it, so that
// even with bad luck the stairs are found before the wind.
const mazeCols, mazeRows = 20, 10

// GenerateMaze makes a floor of a maze of corridors, with a few loops, and
// mazeRooms small rooms (3x3) in it, somewhere on the floor; the rest is
// rock.
func GenerateMaze(rng *rand.Rand) *Level {
	l := &Level{W: MapWidth, H: MapHeight, Tiles: make([]Tile, MapWidth*MapHeight), Maze: true}
	// The cells of the maze are on odd tiles from an odd corner, leaving
	// rock at the edges of the floor.
	cols, rows := min(mazeCols, (MapWidth-2)/2), min(mazeRows, (MapHeight-2)/2)
	ox := 2 * rng.IntN((MapWidth-2)/2-cols+1)
	oy := 2 * rng.IntN((MapHeight-2)/2-rows+1)
	cell := func(cx, cy int) Point { return Point{ox + 1 + 2*cx, oy + 1 + 2*cy} }
	visited := make([]bool, cols*rows)
	var stack [][2]int
	start := [2]int{rng.IntN(cols), rng.IntN(rows)}
	visited[start[1]*cols+start[0]] = true
	l.set(cell(start[0], start[1]), TileCorridor)
	stack = append(stack, start)
	steps := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		var next [][2]int
		for _, s := range steps {
			n := [2]int{c[0] + s[0], c[1] + s[1]}
			if n[0] >= 0 && n[0] < cols && n[1] >= 0 && n[1] < rows && !visited[n[1]*cols+n[0]] {
				next = append(next, n)
			}
		}
		if len(next) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		n := next[rng.IntN(len(next))]
		visited[n[1]*cols+n[0]] = true
		a, b := cell(c[0], c[1]), cell(n[0], n[1])
		l.set(Point{(a.X + b.X) / 2, (a.Y + b.Y) / 2}, TileCorridor)
		l.set(b, TileCorridor)
		stack = append(stack, n)
	}
	// A few loops: walls between two cells broken.
	for range cols * rows / 10 {
		c := cell(rng.IntN(cols-1), rng.IntN(rows-1))
		if rng.IntN(2) == 0 {
			l.set(c.Add(Point{1, 0}), TileCorridor)
		} else {
			l.set(c.Add(Point{0, 1}), TileCorridor)
		}
	}
	// Small rooms, around cells apart from each other.
	for range 1000 {
		if len(l.Rooms) == mazeRooms {
			break
		}
		c := cell(1+rng.IntN(cols-2), 1+rng.IntN(rows-2))
		r := Room{X: c.X - 1, Y: c.Y - 1, W: 3, H: 3}
		if slices.ContainsFunc(l.Rooms, func(o Room) bool {
			return abs(o.X-r.X) < 6 && abs(o.Y-r.Y) < 6
		}) {
			continue
		}
		l.Rooms = append(l.Rooms, r)
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				l.set(Point{x, y}, TileRoom)
			}
		}
	}
	return l
}

// floodRooms is how many rooms of a floor with water get some: 1 to it.
const floodRooms = 2

// islandItems is how many items an island holds (as in Shiren 5).
const islandItems = 3

// flood puts water in a room or two of the floor: a pond in the room, or
// an island (a floor of at least islandItems tiles in a ring of water,
// once a floor). The water keeps off the edge of the room, so its doors
// stay joined by the floor around it.
func (l *Level) flood(rng *rand.Rand) {
	var rooms []int
	for i, r := range l.Rooms {
		if !r.Dummy && r.W >= 5 && r.H >= 5 {
			rooms = append(rooms, i)
		}
	}
	rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })
	island := false
	for _, i := range rooms[:min(len(rooms), 1+rng.IntN(floodRooms))] {
		r := l.Rooms[i]
		// The inside of the room, its edge left out.
		x0, y0, iw, ih := r.X+1, r.Y+1, r.W-2, r.H-2
		if !island && iw >= 5 && ih >= 3 && rng.IntN(2) == 0 {
			island = true
			cw := between(rng, 1, min(iw-2, 4))
			ch := between(rng, 1, min(ih-2, 3))
			if cw*ch < islandItems {
				cw = min(iw-2, (islandItems+ch-1)/ch)
			}
			if cw*ch < islandItems {
				continue
			}
			x, y := x0+rng.IntN(iw-cw-1), y0+rng.IntN(ih-ch-1)
			if l.Island == nil {
				l.Island = map[Point]bool{}
			}
			for dy := range ch + 2 {
				for dx := range cw + 2 {
					p := Point{x + dx, y + dy}
					if dx == 0 || dy == 0 || dx == cw+1 || dy == ch+1 {
						l.set(p, TileWater)
					} else {
						l.Island[p] = true
						l.Isles = append(l.Isles, p)
					}
				}
			}
			continue
		}
		// A pond: a rectangle, its corners cut now and then.
		pw, ph := between(rng, 1, iw), between(rng, 1, ih)
		x, y := x0+rng.IntN(iw-pw+1), y0+rng.IntN(ih-ph+1)
		for dy := range ph {
			for dx := range pw {
				corner := (dx == 0 || dx == pw-1) && (dy == 0 || dy == ph-1)
				if corner && pw > 2 && ph > 2 && rng.IntN(2) == 0 {
					continue
				}
				l.set(Point{x + dx, y + dy}, TileWater)
			}
		}
	}
}

// wet says whether the room r has water (flood): no shop opens there.
func (l *Level) wet(r Room) bool {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if l.At(Point{x, y}) == TileWater {
				return true
			}
		}
	}
	return false
}
