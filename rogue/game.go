package rogue

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"time"
)

// Game is one run.
type Game struct {
	// Dungeon is what the run is in.
	Dungeon *Dungeon
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
	// Items are the items on the floor, Inventory those the player
	// carries (sorted), and Gold its money.
	Items     []*Item
	Inventory []*Item
	Gold      int
	// Prompt is the choice the game waits for, if any.
	Prompt *Prompt
	// Seen are the tiles the player has seen, Safe those known to have
	// no trap: walked on (a trap there would have gone off and shown).
	Seen []bool
	Safe []bool
	// Message is the messages of the last action, Log all of them (the
	// last maxLog); the last action's begin at actionStart.
	Message     string
	Log         []string
	actionStart int
	// ShowMap says whether the mini-map is shown.
	ShowMap bool

	// The player.
	HP, MaxHP   int
	PlayerLevel int
	Strength    int
	// regen counts the turns toward the next HP restored.
	regen int
	// Fullness is how full the player is, up to the rule; hunger counts
	// the turns toward the next point of it lost.
	Fullness int
	hunger   int
	// Lit says whether the floor is lit (a scroll): the player sees it
	// all.
	Lit bool
	// identified are the item kinds (IDs) the player knows; fakeNames the
	// names of the others, for this run.
	identified map[string]bool
	fakeNames  map[string]string
	// Over is why the run is over ("" while it goes on), and Cleared
	// whether the player went down the last stairs.
	Over    string
	Cleared bool
	EXP     int
	// Kills are the enemies the player and its allies defeated.
	Kills int
	// Started and Ended are when the run began and ended (play time).
	Started, Ended time.Time
	// FloorTurns are the turns on this floor, and Wind how many times
	// Kron's wind has blown on it.
	FloorTurns int
	Wind       int

	// rng is the random source of everything but the layout of the
	// floors (combat, for now), so that what the player does never
	// changes the floors of a seed.
	rng *rand.Rand
}

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

// NewGame starts a run in the dungeon d on its first floor.
func NewGame(d *Dungeon, seed string) *Game {
	r := d.Rules
	g := &Game{
		Dungeon: d, Seed: seed, Facing: "down",
		HP: r.PlayerHP, MaxHP: r.PlayerHP, PlayerLevel: 1, Strength: r.Strength,
		Fullness:   r.Fullness,
		rng:        rand.New(rand.NewPCG(seedHash(d, seed), math.MaxUint64)),
		Started:    time.Now(),
		identified: map[string]bool{},
	}
	g.nameItems()
	g.descend()
	return g
}

// floorRNG is the random source of the layout of the floor depth: the
// same for the same seed whatever the player did before.
func (g *Game) floorRNG(depth int) *rand.Rand {
	return rand.New(rand.NewPCG(seedHash(g.Dungeon, g.Seed), uint64(depth)))
}

// seedHash is the seed of the random sources: the seed, in the dungeon as
// it is defined (the same seed with another definition makes other
// floors).
func seedHash(d *Dungeon, seed string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(d.ID + "\x00" + d.Hash + "\x00" + seed))
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
	g.FloorTurns, g.Wind, g.Lit = 0, 0, false
	rng := g.floorRNG(g.Depth)
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

	d := g.Dungeon
	c := d.counts(g.Depth)
	for range between(rng, c.MinMonster, c.MaxMonster) {
		pos := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		m := newMonster(d.Monster(pick(rng, d.MonsterRows, g.Depth)))
		m.Pos = pos
		// A third of them doze, but never in the player's room.
		m.Dozing = rng.IntN(3) == 0 && g.Level.RoomAt(pos) != start
		g.Monsters = append(g.Monsters, m)
	}
	for range between(rng, c.MinItem, c.MaxItem) {
		pos := g.randomTile(rng, rooms[rng.IntN(len(rooms))])
		def := d.Item(pick(rng, d.ItemRows, g.Depth))
		it := &Item{Def: def, Pos: pos, Glyph: def.Glyph}
		switch {
		case def.Category == CategoryMoney:
			it.Amount = between(rng, 10, 30+g.Depth*10)
		case it.upgradable():
			it.Upgrade = rollUpgrade(rng)
		case def.Category == CategoryStaff:
			it.Charges = between(rng, def.Charges[0], def.Charges[1])
		case def.Category == CategoryArrow:
			it.Count = between(rng, def.Charges[0], def.Charges[1])
		}
		g.Items = append(g.Items, it)
	}

	if g.Depth == 1 && d.Rules.FirstAlly != "" {
		ally = newMonster(d.Monster(d.Rules.FirstAlly))
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
	g.say(fmt.Sprintf("%dF に着いた。", g.Depth))
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
	return !slices.ContainsFunc(g.Items, func(it *Item) bool { return it.Pos == p })
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
func (g *Game) ItemAt(p Point) (*Item, bool) {
	for _, it := range g.Items {
		if it.Pos == p {
			return it, true
		}
	}
	return nil, false
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
// (walls and doorways included), or next to it, or anywhere on a lit
// floor.
func (g *Game) Visible(p Point) bool { return g.Lit || g.sees(g.Player, p) }

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
		case "redraw", "log":
		case "map":
			g.ShowMap = !g.ShowMap
		default:
			g.beginAction()
			g.say("冒険は終わった。")
		}
		return nil
	}
	// Any other action drops the choice the game was waiting for.
	switch action {
	case "redraw", "log", "map", "choose", "cancel":
	default:
		g.Prompt = nil
	}
	switch action {
	case "redraw", "log":
		// (The Vim script asks for the log with the screen.)
	case "inventory":
		// Looking costs no turn.
		g.beginAction()
		g.openInventory()
	case "choose":
		if len(args) != 1 {
			return fmt.Errorf("choose: a choice expected")
		}
		g.beginAction()
		turn, err := g.choose(args[0])
		if err != nil {
			return err
		}
		if turn {
			g.endTurn(false)
		}
	case "cancel":
		g.Prompt = nil
	case "pickup":
		g.beginAction()
		if !g.pickUp() {
			g.say("足元には何もない。")
			return nil
		}
		g.endTurn(false)
	case "map":
		// Showing the mini-map costs no turn.
		g.ShowMap = !g.ShowMap
	case "move":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.beginAction()
		g.Facing = name
		if g.step(d, true) {
			g.endTurn(true)
		}
	case "turn":
		// Facing costs no turn.
		if len(args) == 0 {
			g.beginAction()
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
		g.beginAction()
		g.Facing = name
		g.dash(d)
	case "attack":
		g.beginAction()
		if g.attack() {
			g.endTurn(false)
		}
	case "shoot":
		g.beginAction()
		if g.shoot() {
			g.endTurn(false)
		}
	case "wait":
		g.beginAction()
		g.endTurn(true)
	case "stairs":
		g.beginAction()
		if g.Player != g.Stairs {
			g.say("ここに階段はない。")
			return nil
		}
		g.Turn++
		if g.Depth >= g.Dungeon.Floors {
			g.end(fmt.Sprintf("%dF を踏破した", g.Depth), true)
			return nil
		}
		g.wearAndTear()
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
			return
		}
	}
	g.say("周りに敵はいない。")
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
	r := g.Dungeon.Rules
	if !hits(g.rng, r.PlayerHitRate) {
		g.say(m.Name + "にかわされた。")
		return true
	}
	n := damage(g.rng, g.Attack(), m.Defense, r.DefenseK)
	g.use(EffectStrength)
	m.HP -= n
	g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
	if m.HP <= 0 {
		g.remove(m)
		g.say(m.Name + "を倒した。")
		g.defeated(m)
	}
	return true
}

// endTurn ends a turn of the player: walking or waiting restores HP (other
// actions do not), and then the monsters act.
func (g *Game) endTurn(rest bool) {
	g.Turn++
	if rest && g.HP < g.MaxHP && g.Fullness > 0 {
		g.regen++
		if g.wears(EffectRegen) {
			g.regen++
			g.use(EffectRegen)
		}
		if g.regen >= g.Dungeon.Rules.RegenTurns {
			g.regen = 0
			g.HP++
		}
	}
	g.getHungry()
	if g.Over != "" {
		return
	}
	g.monstersAct()
	g.sense()
	if g.Over == "" {
		g.FloorTurns++
		g.linger()
	}
}

// linger is what lingering on a floor brings: new monsters now and then,
// and Kron's wind.
func (g *Game) linger() {
	d := g.Dungeon
	r := d.Rules
	if g.FloorTurns%r.SpawnTurns == 0 && len(g.Monsters) < r.MaxMonsters {
		g.spawn(d.Monster(pick(g.rng, d.MonsterRows, g.Depth)))
	}
	if g.Wind >= len(r.WindTurns) || g.FloorTurns < r.WindTurns[g.Wind] {
		return
	}
	g.Wind++
	switch g.Wind {
	case 1:
		g.say("風が吹いてきた……。")
	case 2:
		g.say("強い風が吹いてきた！　何かが近づいてくる。")
		for _, m := range g.Monsters {
			if !m.Ally {
				g.rankUp(m)
			}
		}
		// A monster ranks above the floor's.
		rank := d.Monster(pick(g.rng, d.MonsterRows, g.Depth)).Rank + r.StrongerBy
		kinds := d.ofRank(min(rank, maxRank))
		if m := g.spawn(kinds[g.rng.IntN(len(kinds))]); m != nil {
			m.Hunter = true
		}
	case 3:
		g.end("クロンの風に吹き飛ばされた", false)
	}
}

// spawn puts a new monster of the kind def on a free tile out of the
// player's sight, and returns it (nil if there is no room).
func (g *Game) spawn(def *MonsterDef) *Monster {
	for range 100 {
		p := g.wanderTarget()
		if !g.Level.At(p).Walkable() || g.occupied(p) || g.sees(g.Player, p) || chebyshev(p, g.Player) <= 4 {
			continue
		}
		m := newMonster(def)
		m.Pos = p
		m.Spawned = true
		g.Monsters = append(g.Monsters, m)
		return m
	}
	return nil
}

// end ends the run, for why.
func (g *Game) end(why string, cleared bool) {
	g.Over, g.Cleared, g.Ended = why, cleared, time.Now()
	g.say(why + "。")
}

// beginAction starts the messages of an action.
func (g *Game) beginAction() {
	g.Message = ""
	g.actionStart = len(g.Log)
}

// say adds msg (a sentence) to the messages of the action, and to the log.
func (g *Game) say(msg string) {
	g.Message += msg
	g.Log = append(g.Log, msg)
	if n := len(g.Log) - maxLog; n > 0 {
		g.Log = slices.Delete(g.Log, 0, n)
		g.actionStart = max(g.actionStart-n, 0)
	}
}

// maxLog is how many messages the log keeps.
const maxLog = 1000

// canStep says whether the player can go from its tile by d: onto a
// walkable tile, and not diagonally around a corner of rock.
func (g *Game) canStep(d Point) bool {
	return g.Level.At(g.Player.Add(d)).Walkable() && g.clearFrom(g.Player, d)
}

// step moves the player by d; it says whether it moved. With pick, it
// picks up the item it steps on.
func (g *Game) step(d Point, pick bool) bool {
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
		// Walking onto an item picks it up; a dash only stops on it.
		if !pick || !g.pickUp() {
			g.say(g.Name(it) + "が落ちている。")
		}
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
		if !g.step(d, false) {
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
	s := fmt.Sprintf("攻撃 %d  防御 %d  ちから %d  ターン %d  向き %s  シード %s",
		g.Attack(), g.Defense(), g.CurrentStrength(), g.Turn, directionArrows[g.Facing], g.Seed)
	if w := g.Dungeon.Rules.WindTurns; g.Wind > 0 && g.Wind < len(w) {
		s += fmt.Sprintf("  次の風まで %d", w[g.Wind]-g.FloorTurns)
	}
	return s
}
