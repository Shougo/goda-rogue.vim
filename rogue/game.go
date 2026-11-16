package rogue

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
)

// Monster is a creature on the floor. The prototype's monsters do not act.
type Monster struct {
	Pos   Point
	Glyph string
	Name  string
	// Rank is 1 to 4, shown as its color.
	Rank int
	Ally bool
}

// Item is an item lying on the floor.
type Item struct {
	Pos   Point
	Glyph string
}

// Game is one run.
type Game struct {
	// Seed is the text the run was started from: the same seed makes the
	// same floors.
	Seed     string
	Depth    int
	Turn     int
	Level    *Level
	Player   Point
	Facing   string
	Stairs   Point
	Monsters []*Monster
	Items    []Item
	Seen     []bool
	Message  string
}

// The kinds of monsters and items of the prototype.
var (
	monsterKinds = []struct{ Glyph, Name string }{
		{"鼠", "鼠"}, {"蛇", "蛇"}, {"狼", "狼"}, {"骸", "骸骨"},
		{"霊", "亡霊"}, {"鬼", "鬼"}, {"竜", "竜"}, {"盗", "盗賊"},
	}
	itemGlyphs = []string{"剣", "盾", "腕", "草", "巻", "杖", "壺", "矢", "金", "食"}
)

// Directions are the names of the eight directions.
var Directions = map[string]Point{
	"left": {-1, 0}, "right": {1, 0}, "up": {0, -1}, "down": {0, 1},
	"upleft": {-1, -1}, "upright": {1, -1}, "downleft": {-1, 1}, "downright": {1, 1},
}

// directionNames are the directions as the status line shows them.
var directionNames = map[string]string{
	"left": "左", "right": "右", "up": "上", "down": "下",
	"upleft": "左上", "upright": "右上", "downleft": "左下", "downright": "右下",
}

// NewGame starts a run on the first floor.
func NewGame(seed string) *Game {
	g := &Game{Seed: seed, Facing: "down"}
	g.descend()
	return g
}

// floorRNG is the random source of the layout of the floor depth: the
// same for the same seed whatever the player did before.
func floorRNG(seed string, depth int) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return rand.New(rand.NewPCG(h.Sum64(), uint64(depth)))
}

// descend makes the next floor. An ally next to the player comes along.
func (g *Game) descend() {
	var ally *Monster
	for _, m := range g.Monsters {
		if m.Ally && chebyshev(m.Pos, g.Player) <= 1 {
			ally = m
		}
	}

	g.Depth++
	rng := floorRNG(g.Seed, g.Depth)
	g.Level = Generate(rng)
	g.Seen = make([]bool, len(g.Level.Tiles))
	g.Monsters = nil
	g.Items = nil

	var rooms []int
	for i, r := range g.Level.Rooms {
		if !r.Dummy {
			rooms = append(rooms, i)
		}
	}
	start := rooms[rng.IntN(len(rooms))]
	g.Player = g.randomTile(rng, start)
	stairsRoom := start
	if len(rooms) > 1 {
		for stairsRoom == start {
			stairsRoom = rooms[rng.IntN(len(rooms))]
		}
	}
	g.Stairs = g.randomTile(rng, stairsRoom)

	for range between(rng, 5, 8) {
		k := monsterKinds[rng.IntN(len(monsterKinds))]
		pos := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		g.Monsters = append(g.Monsters, &Monster{Pos: pos, Glyph: k.Glyph, Name: k.Name, Rank: 1 + rng.IntN(4)})
	}
	for range between(rng, 6, 10) {
		pos := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		g.Items = append(g.Items, Item{Pos: pos, Glyph: itemGlyphs[rng.IntN(len(itemGlyphs))]})
	}

	if g.Depth == 1 {
		// The prototype starts with an ally, to see how one looks.
		k := monsterKinds[rng.IntN(len(monsterKinds))]
		ally = &Monster{Glyph: k.Glyph, Name: k.Name, Rank: 1 + rng.IntN(4), Ally: true}
	}
	if ally != nil {
		if p, ok := g.freeNeighbor(g.Player); ok {
			ally.Pos = p
			g.Monsters = append(g.Monsters, ally)
		}
	}
	g.look()
	g.Message = fmt.Sprintf("%dF に着いた。", g.Depth)
}

// randomTile returns a free tile of the room i.
func (g *Game) randomTile(rng *rand.Rand, i int) Point {
	r := g.Level.Rooms[i]
	for range 100 {
		p := Point{r.X + rng.IntN(r.W), r.Y + rng.IntN(r.H)}
		if g.free(p) {
			return p
		}
	}
	return Point{r.X, r.Y}
}

// free says whether nothing stands or lies on p.
func (g *Game) free(p Point) bool {
	if p == g.Player || p == g.Stairs || g.MonsterAt(p) != nil {
		return false
	}
	return !slices.ContainsFunc(g.Items, func(it Item) bool { return it.Pos == p })
}

// freeNeighbor returns a free walkable tile next to p.
func (g *Game) freeNeighbor(p Point) (Point, bool) {
	for _, d := range []Point{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		q := p.Add(d)
		if g.Level.At(q).Walkable() && g.free(q) {
			return q, true
		}
	}
	return Point{}, false
}

// MonsterAt returns the monster on p, or nil.
func (g *Game) MonsterAt(p Point) *Monster {
	for _, m := range g.Monsters {
		if m.Pos == p {
			return m
		}
	}
	return nil
}

// ItemAt returns the item on p.
func (g *Game) ItemAt(p Point) (Item, bool) {
	for _, it := range g.Items {
		if it.Pos == p {
			return it, true
		}
	}
	return Item{}, false
}

// look marks what the player sees as seen: the whole room it is in with
// its walls, and the tiles around it.
func (g *Game) look() {
	l := g.Level
	if i := l.RoomAt(g.Player); i >= 0 {
		r := l.Rooms[i]
		for y := r.Y - 1; y <= r.Y+r.H; y++ {
			for x := r.X - 1; x <= r.X+r.W; x++ {
				g.markSeen(Point{x, y})
			}
		}
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			g.markSeen(g.Player.Add(Point{dx, dy}))
		}
	}
}

func (g *Game) markSeen(p Point) {
	if p.X >= 0 && p.Y >= 0 && p.X < g.Level.W && p.Y < g.Level.H {
		g.Seen[p.Y*g.Level.W+p.X] = true
	}
}

// IsSeen says whether the player has seen p.
func (g *Game) IsSeen(p Point) bool {
	if p.X < 0 || p.Y < 0 || p.X >= g.Level.W || p.Y >= g.Level.H {
		return false
	}
	return g.Seen[p.Y*g.Level.W+p.X]
}

// Visible says whether the player sees what is on p now: in the same room
// (walls and doorways included), or next to it.
func (g *Game) Visible(p Point) bool {
	if chebyshev(p, g.Player) <= 1 {
		return true
	}
	i := g.Level.RoomAt(g.Player)
	return i >= 0 && g.Level.Rooms[i].Around(p)
}

func chebyshev(a, b Point) int {
	return max(abs(a.X-b.X), abs(a.Y-b.Y))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Do runs the action with its arguments.
func (g *Game) Do(action string, args []string) error {
	dir := func() (string, Point, error) {
		if len(args) != 1 {
			return "", Point{}, fmt.Errorf("%s: a direction expected", action)
		}
		d, ok := Directions[args[0]]
		if !ok {
			return "", Point{}, fmt.Errorf("%s: unknown direction %q", action, args[0])
		}
		return args[0], d, nil
	}
	switch action {
	case "redraw":
	case "move":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.Message = ""
		g.Facing = name
		g.step(d)
	case "turn":
		// Facing costs no turn.
		name, _, err := dir()
		if err != nil {
			return err
		}
		g.Facing = name
	case "dash":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.Message = ""
		g.Facing = name
		g.dash(d)
	case "wait":
		g.Message = ""
		g.Turn++
	case "stairs":
		if g.Player != g.Stairs {
			g.Message = "ここに階段はない。"
			return nil
		}
		g.Turn++
		g.descend()
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

// canStep says whether the player can go from its tile by d: onto a
// walkable tile, and not diagonally around a corner of rock.
func (g *Game) canStep(d Point) bool {
	l := g.Level
	to := g.Player.Add(d)
	if !l.At(to).Walkable() {
		return false
	}
	if d.X != 0 && d.Y != 0 {
		return l.At(g.Player.Add(Point{d.X, 0})).Walkable() && l.At(g.Player.Add(Point{0, d.Y})).Walkable()
	}
	return true
}

// step moves the player by d; it says whether it moved.
func (g *Game) step(d Point) bool {
	if !g.canStep(d) {
		return false
	}
	to := g.Player.Add(d)
	if m := g.MonsterAt(to); m != nil {
		if !m.Ally {
			g.Message = m.Name + "がいる。"
			return false
		}
		// Change places with an ally.
		m.Pos = g.Player
	}
	g.Player = to
	g.Turn++
	g.look()
	if it, ok := g.ItemAt(to); ok {
		g.Message = it.Glyph + "が落ちている。"
	}
	if to == g.Stairs {
		g.Message = "階段がある。"
	}
	return true
}

// dash moves the player by d until something stops it: rock or a monster
// ahead, an item or the stairs under it, or entering or leaving a room.
func (g *Game) dash(d Point) {
	for range g.Level.W * g.Level.H {
		room := g.Level.RoomAt(g.Player)
		ahead := g.MonsterAt(g.Player.Add(d))
		if ahead != nil && ahead.Ally {
			return
		}
		if !g.step(d) {
			return
		}
		if _, ok := g.ItemAt(g.Player); ok || g.Player == g.Stairs {
			return
		}
		if g.Level.RoomAt(g.Player) != room {
			return
		}
	}
}

// Status is the status line.
func (g *Game) Status() string {
	return fmt.Sprintf("%dF  ターン %d  向き %s  シード %s", g.Depth, g.Turn, directionNames[g.Facing], g.Seed)
}
