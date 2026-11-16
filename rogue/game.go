package rogue

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
)

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
	// Seen are the tiles the player has seen, Safe those known to have
	// no trap: walked on (a trap there would have gone off and shown).
	Seen    []bool
	Safe    []bool
	Message string
	// ShowMap says whether the mini-map is shown.
	ShowMap bool

	// The player.
	HP, MaxHP   int
	PlayerLevel int
	Strength    int
	// regen counts the turns toward the next HP restored.
	regen int
	// Over is why the run is over ("" while it goes on).
	Over string

	// rng is the random source of everything but the layout of the
	// floors (combat, for now), so that what the player does never
	// changes the floors of a seed.
	rng *rand.Rand
}

// itemGlyphs are the kinds of items of the prototype.
var itemGlyphs = []string{"剣", "盾", "腕", "草", "巻", "杖", "壺", "矢", "金", "食"}

// Directions are the names of the eight directions.
var Directions = map[string]Point{
	"left": {-1, 0}, "right": {1, 0}, "up": {0, -1}, "down": {0, 1},
	"upleft": {-1, -1}, "upright": {1, -1}, "downleft": {-1, 1}, "downright": {1, 1},
}

// directionArrows are the directions as the status line shows them. The
// arrows are of ambiguous width, which is fine there: the status line is
// not a part of the map.
var directionArrows = map[string]string{
	"left": "←", "right": "→", "up": "↑", "down": "↓",
	"upleft": "↖", "upright": "↗", "downleft": "↙", "downright": "↘",
}

// NewGame starts a run on the first floor.
func NewGame(seed string) *Game {
	g := &Game{
		Seed: seed, Facing: "down",
		HP: playerHP, MaxHP: playerHP, PlayerLevel: 1, Strength: playerStrength,
		rng: rand.New(rand.NewPCG(seedHash(seed), math.MaxUint64)),
	}
	g.descend()
	return g
}

// floorRNG is the random source of the layout of the floor depth: the
// same for the same seed whatever the player did before.
func floorRNG(seed string, depth int) *rand.Rand {
	return rand.New(rand.NewPCG(seedHash(seed), uint64(depth)))
}

func seedHash(seed string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return h.Sum64()
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
	g.Safe = make([]bool, len(g.Level.Tiles))
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
		m := newMonster(k, floorRank(rng, g.Depth))
		m.Pos = pos
		// A third of them doze, but never in the player's room.
		m.Dozing = rng.IntN(3) == 0 && g.Level.RoomAt(pos) != start
		g.Monsters = append(g.Monsters, m)
	}
	for range between(rng, 6, 10) {
		pos := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		g.Items = append(g.Items, Item{Pos: pos, Glyph: itemGlyphs[rng.IntN(len(itemGlyphs))]})
	}

	if g.Depth == 1 {
		// The prototype starts with an ally, to see how one looks.
		k := monsterKinds[rng.IntN(len(monsterKinds))]
		ally = newMonster(k, 1)
		ally.Ally = true
	}
	if ally != nil {
		if p, ok := g.freeNeighbor(g.Player); ok {
			ally.Pos = p
			g.Monsters = append(g.Monsters, ally)
		}
	}
	g.look()
	g.markSafe(g.Player)
	g.Message = fmt.Sprintf("%dF に着いた。", g.Depth)
}

// floorRank returns the rank of a monster on the floor depth: 1 on the
// first floors, one more every four floors, and sometimes one above.
func floorRank(rng *rand.Rand, depth int) int {
	rank := 1 + (depth-1)/4
	if rng.IntN(5) == 0 {
		rank++
	}
	return min(rank, 4)
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
	if i, ok := g.index(p); ok {
		g.Seen[i] = true
	}
}

func (g *Game) markSafe(p Point) {
	if i, ok := g.index(p); ok {
		g.Safe[i] = true
	}
}

// index returns the index of p in Level.Tiles, Seen and Safe.
func (g *Game) index(p Point) (int, bool) {
	if p.X < 0 || p.Y < 0 || p.X >= g.Level.W || p.Y >= g.Level.H {
		return 0, false
	}
	return p.Y*g.Level.W + p.X, true
}

// IsSeen says whether the player has seen p.
func (g *Game) IsSeen(p Point) bool {
	i, ok := g.index(p)
	return ok && g.Seen[i]
}

// IsSafe says whether p is known to have no trap. The prototype has no
// traps yet; checking for traps will mark tiles safe too, and a trap set
// later on a tile will unmark it.
func (g *Game) IsSafe(p Point) bool {
	i, ok := g.index(p)
	return ok && g.Safe[i]
}

// Visible says whether the player sees what is on p now: in the same room
// (walls and doorways included), or next to it.
func (g *Game) Visible(p Point) bool { return g.sees(g.Player, p) }

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
	if g.Over != "" {
		switch action {
		case "redraw":
		case "map":
			g.ShowMap = !g.ShowMap
		default:
			g.Message = "冒険は終わった。"
		}
		return nil
	}
	switch action {
	case "redraw":
	case "map":
		// Showing the mini-map costs no turn.
		g.ShowMap = !g.ShowMap
	case "move":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.Message = ""
		g.Facing = name
		if g.step(d) {
			g.endTurn(true)
		}
	case "turn":
		// Facing costs no turn.
		if len(args) == 0 {
			g.faceEnemy()
			return nil
		}
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
	case "attack":
		g.Message = ""
		if g.attack() {
			g.endTurn(false)
		}
	case "wait":
		g.Message = ""
		g.endTurn(true)
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

// clockwise are the directions clockwise from up.
var clockwise = []string{"up", "upright", "right", "downright", "down", "downleft", "left", "upleft"}

// faceEnemy faces the next enemy next to the player that it can attack,
// clockwise from the direction it faces: again and again, it faces each of
// them in turn.
func (g *Game) faceEnemy() {
	i := slices.Index(clockwise, g.Facing)
	for n := 1; n <= len(clockwise); n++ {
		name := clockwise[(i+n)%len(clockwise)]
		d := Directions[name]
		if m := g.MonsterAt(g.Player.Add(d)); m != nil && !m.Ally && g.clearFrom(g.Player, d) {
			g.Facing = name
			g.Message = ""
			return
		}
	}
	g.Message = "周りに敵はいない。"
}

// attack attacks the tile the player faces; it says whether it took the
// turn: always, even when it hits nothing, but an ally is not attacked.
func (g *Game) attack() bool {
	d := Directions[g.Facing]
	m := g.MonsterAt(g.Player.Add(d))
	if m != nil && m.Ally {
		g.say("仲間は攻撃しない。")
		return false
	}
	// No attack around a corner of rock.
	if m == nil || !g.clearFrom(g.Player, d) {
		g.say("空振りした。")
		return true
	}
	m.Dozing = false
	if !hits(g.rng, playerHitRate) {
		g.say(m.Name + "にかわされた。")
		return true
	}
	n := damage(g.rng, playerAttack(g.PlayerLevel, g.Strength), m.Defense)
	m.HP -= n
	g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
	if m.HP <= 0 {
		g.remove(m)
		g.say(m.Name + "を倒した。")
	}
	return true
}

// endTurn ends a turn of the player: walking or waiting restores HP (other
// actions do not), and then the monsters act.
func (g *Game) endTurn(rest bool) {
	g.Turn++
	if rest && g.HP < g.MaxHP {
		g.regen++
		if g.regen >= regenTurns {
			g.regen = 0
			g.HP++
		}
	}
	g.monstersAct()
}

// say adds msg to the message of the action.
func (g *Game) say(msg string) {
	g.Message += msg
}

// canStep says whether the player can go from its tile by d: onto a
// walkable tile, and not diagonally around a corner of rock.
func (g *Game) canStep(d Point) bool {
	return g.Level.At(g.Player.Add(d)).Walkable() && g.clearFrom(g.Player, d)
}

// step moves the player by d; it says whether it moved.
func (g *Game) step(d Point) bool {
	if !g.canStep(d) {
		return false
	}
	to := g.Player.Add(d)
	if m := g.MonsterAt(to); m != nil {
		if !m.Ally {
			g.say(m.Name + "がいる。")
			return false
		}
		// Change places with an ally.
		m.Pos = g.Player
	}
	// Entering or leaving a room wakes the monsters dozing in it.
	from := g.Level.RoomAt(g.Player)
	g.Player = to
	if room := g.Level.RoomAt(to); room != from {
		g.wake(from)
		g.wake(room)
	}
	g.look()
	g.markSafe(to)
	if it, ok := g.ItemAt(to); ok {
		g.say(it.Glyph + "が落ちている。")
	}
	if to == g.Stairs {
		g.say("階段がある。")
	}
	return true
}

// dash moves the player by d until something stops it: rock or a monster
// ahead, an item or the stairs under it, entering or leaving a room, an
// enemy coming into sight, or getting hurt.
func (g *Game) dash(d Point) {
	for range g.Level.W * g.Level.H {
		room := g.Level.RoomAt(g.Player)
		ahead := g.MonsterAt(g.Player.Add(d))
		if ahead != nil && ahead.Ally {
			return
		}
		seen := g.visibleEnemies()
		hp := g.HP
		if !g.step(d) {
			return
		}
		g.endTurn(true)
		if g.Over != "" || g.HP < hp {
			return
		}
		if _, ok := g.ItemAt(g.Player); ok || g.Player == g.Stairs {
			return
		}
		if g.Level.RoomAt(g.Player) != room {
			return
		}
		for _, m := range g.visibleEnemies() {
			if !slices.Contains(seen, m) {
				return
			}
		}
	}
}

// visibleEnemies returns the enemies the player sees.
func (g *Game) visibleEnemies() []*Monster {
	var ms []*Monster
	for _, m := range g.Monsters {
		if !m.Ally && g.Visible(m.Pos) {
			ms = append(ms, m)
		}
	}
	return ms
}

// Status is the status line, under the map.
func (g *Game) Status() string {
	return fmt.Sprintf("ターン %d  向き %s  シード %s", g.Turn, directionArrows[g.Facing], g.Seed)
}
