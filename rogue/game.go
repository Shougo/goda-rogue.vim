package rogue

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
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
	// ShowMap says whether the mini-map is shown; ShowItems the list of
	// the items carried.
	ShowMap, ShowItems bool

	// The player.
	HP, MaxHP   int
	PlayerLevel int
	// Strength is the strength of the player now, MaxStrength what it
	// comes back to (poison lowers the first).
	Strength, MaxStrength int
	// regen counts the turns toward the next HP restored.
	regen int
	// Fullness is how full the player is, up to the rule; hunger counts
	// the turns toward the next point of it lost.
	Fullness int
	hunger   int
	// Lit says whether the floor is lit (a scroll): the player sees it
	// all.
	Lit bool
	// sights are what the prompt of look lists.
	sights []sight
	// ordered is the ally the prompt of orders is about.
	ordered *Monster
	// Dark says whether the floor is dark: the player sees only around.
	Dark bool
	// Traps are the traps of the floor.
	Traps []*Trap
	// Decoys are the mimics that look like the stairs.
	Decoys []Decoy
	// House is the room of the monster house of the floor (-1 for none);
	// houseFound says whether the player has entered it.
	House      int
	houseFound bool
	// States are the states of the player that pass with time
	// (state.go). hasteSkip says whether the monsters skip the next
	// action of a fast player.
	States
	hasteSkip bool
	// distCache keeps the distances worked out (distances).
	distCache distCache
	// Stuck is how many more turns the player is caught in a snare;
	// sprung says whether a trap went off on the last step.
	Stuck  int
	sprung bool
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
	// Started and Ended are when the run began (or was resumed) and
	// ended; Played is the play time before Started (a resumed run).
	Started, Ended time.Time
	Played         time.Duration
	// events are what happened in the run, for its replay file (see
	// event).
	events []string
	// guaranteed are the guaranteed items of each floor
	// (planGuaranteed).
	guaranteed map[int][]string
	// Axis says whether the line the player faces shows (after turning:
	// the axis of a throw, an arrow or a bolt).
	Axis bool
	// DebugMode says whether the run may use the debug mode (debug.go),
	// Debugged whether it did, and Invincible whether the player cannot
	// fall (a debug command).
	DebugMode  bool
	Debugged   bool
	Invincible bool
	// Steps are the steps the player has taken.
	Steps int
	// Shop is the shop of the floor (nil for none); selling the item the
	// shopkeeper offers to buy.
	Shop    *Shop
	selling *Item
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
		HP: r.PlayerHP, MaxHP: r.PlayerHP, PlayerLevel: 1, Strength: r.Strength, MaxStrength: r.Strength,
		Fullness:   r.Fullness,
		rng:        rand.New(rand.NewPCG(seedHash(d, seed), math.MaxUint64)),
		Started:    time.Now(),
		identified: map[string]bool{},
	}
	g.nameItems()
	g.planGuaranteed()
	g.descend()
	return g
}

// planGuaranteed decides on which floor each guaranteed item appears
// (guaranteed_items.txt), by a random source of its own: the other floors
// stay as they are without them.
func (g *Game) planGuaranteed() {
	rng := rand.New(rand.NewPCG(seedHash(g.Dungeon, g.Seed), math.MaxUint64-2))
	g.guaranteed = map[int][]string{}
	for _, r := range g.Dungeon.Guaranteed {
		f := between(rng, r.First, min(r.Last, g.Dungeon.Floors))
		g.guaranteed[f] = append(g.guaranteed[f], r.ID)
	}
}

// floorRNG is the random source of the layout of the floor depth: the
// same for the same seed whatever the player did before.
func (g *Game) floorRNG(depth int) *rand.Rand {
	return rand.New(rand.NewPCG(seedHash(g.Dungeon, g.Seed), uint64(depth)))
}

// seedHash is the seed of the random sources: the seed, in the dungeon as
// it is defined (the same seed with another definition makes other
// floors). A hash that mixes well: with FNV, seeds alike ("1", "2", "3")
// made runs alike (the share of clears of 300 runs of a bot went from 44%
// to 57% from a set of such seeds to another, twice what chance gives).
func seedHash(d *Dungeon, seed string) uint64 {
	sum := sha256.Sum256([]byte(d.ID + "\x00" + d.Hash + "\x00" + seed))
	return binary.LittleEndian.Uint64(sum[:8])
}

// descend makes the next floor. The allies next to the player or in its
// room come along.
func (g *Game) descend() {
	if g.Shop != nil && g.debt() > 0 {
		// Leaving the floor with what it owes (a pit).
		g.steal()
	}
	var along []*Monster
	for _, m := range g.allies() {
		if g.comesAlong(m) {
			along = append(along, m)
		}
	}

	g.Depth++
	g.event("floor %d", g.Depth)
	// (One generator for the floor: the steps below draw from it in this
	// order, which the seed of a run depends on.)
	rng := g.floorRNG(g.Depth)
	g.newLevel(rng)
	rooms, start := g.placeStairs(rng)
	g.populate(rng, rooms, start)
	g.bringAllies(along)
	g.look()
	g.markSafe(g.Player)
	g.say(fmt.Sprintf("%dF に着いた。", g.Depth))
	g.enterHouse()
	g.arriveWithMimics()
}

// newLevel makes the map of a new floor (big room, maze or rooms, as
// floors.txt says), and leaves the state of the last floor behind.
func (g *Game) newLevel(rng *rand.Rand) {
	g.FloorTurns, g.Wind, g.Lit, g.Dark = 0, 0, false, false
	// (The seal of the mouth is for the floor.)
	g.Sealed = false
	c := g.Dungeon.counts(g.Depth)
	switch n := c.BigRoom + c.Maze; {
	case n == 0:
		// No roll: the floors stay as they were without these.
		g.Level = Generate(rng)
	default:
		switch roll := rng.IntN(100); {
		case roll < c.BigRoom:
			g.Level = GenerateBigRoom()
		case roll < n:
			g.Level = GenerateMaze(rng)
		default:
			g.Level = Generate(rng)
		}
	}
	g.House, g.houseFound = -1, false
	g.Shop, g.selling = nil, nil
	g.Seen = make([]bool, len(g.Level.Tiles))
	g.Safe = make([]bool, len(g.Level.Tiles))
	switch d := c.Brightness; {
	case d == BrightnessBright:
		g.light()
	default:
		g.Dark = d == BrightnessDark
	}
	g.Monsters = nil
	g.Items = nil
	g.Traps = nil
	g.Decoys = nil
	g.Stuck = 0
}

// placeStairs puts the player in a room of the new floor and the stairs
// in another (if there is another), and returns the rooms (not dummies)
// and the player's.
func (g *Game) placeStairs(rng *rand.Rand) (rooms []int, start int) {
	for i, r := range g.Level.Rooms {
		if !r.Dummy {
			rooms = append(rooms, i)
		}
	}
	start = rooms[rng.IntN(len(rooms))]
	g.Player = g.randomTile(rng, start)
	stairsRoom := start
	if len(rooms) > 1 {
		for stairsRoom == start {
			stairsRoom = rooms[rng.IntN(len(rooms))]
		}
	}
	g.Stairs = g.randomTile(rng, stairsRoom)
	return rooms, start
}

// populate puts the monsters, items, monster house, shop, guaranteed
// items, traps and mimics of the new floor; none dozes in start, the
// player's room.
func (g *Game) populate(rng *rand.Rand, rooms []int, start int) {
	d := g.Dungeon
	c := d.counts(g.Depth)
	for range between(rng, c.MinMonster, c.MaxMonster) {
		pos := g.spot(rng, rooms)
		m := newMonster(d.Monster(pick(rng, d.MonsterRows, g.Depth)))
		m.Pos = pos
		// Some of them doze (a third, or as monsters.txt 眠り says),
		// but never in the player's room.
		m.Dozing = m.dozes(rng) && g.Level.RoomAt(pos) != start
		g.Monsters = append(g.Monsters, m)
	}
	for range between(rng, c.MinItem, c.MaxItem) {
		pos := g.spot(rng, rooms)
		g.Items = append(g.Items, g.newItem(rng, pos))
	}
	if c.House > 0 && rng.IntN(100) < c.House {
		g.fillHouse(rng, rooms, start)
	}
	if c.Shop > 0 && rng.IntN(100) < c.Shop {
		g.openShop(rng, rooms, start)
	}
	// The guaranteed items: free, so not in the shop.
	for _, id := range g.guaranteed[g.Depth] {
		if pos, ok := g.find(func() Point { return g.spot(rng, rooms) }, needFree|needNoShop); ok {
			g.Items = append(g.Items, g.newItemOf(rng, d.Item(id), pos))
		}
	}
	g.placeTraps(rng, rooms)
	g.hideMimics(rng, rooms)
}

// bringAllies puts the allies that came along (and the first one, on
// the first floor) next to the player.
func (g *Game) bringAllies(along []*Monster) {
	d := g.Dungeon
	if g.Depth == 1 && d.Rules.FirstAlly != "" {
		ally := newMonster(d.Monster(d.Rules.FirstAlly))
		ally.Ally = true
		along = append(along, ally)
	}
	for _, ally := range along {
		if p, ok := g.freeNeighbor(g.Player); ok {
			ally.Pos = p
			// One that waited came along: it follows again.
			if ally.Order == OrderWait {
				ally.Order = OrderFollow
			}
			ally.hasTarget = false
			g.Monsters = append(g.Monsters, ally)
		}
	}
}

// randomTile returns a free tile of the room i.
func (g *Game) randomTile(rng *rand.Rand, i int) Point {
	r := g.Level.Rooms[i]
	if p, ok := g.find(func() Point { return Point{r.X + rng.IntN(r.W), r.Y + rng.IntN(r.H)} }, needFree); ok {
		return p
	}
	return Point{r.X, r.Y}
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
	if i := l.RoomAt(g.Player); i >= 0 && !g.Dark {
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
// floor; only next to it on a dark one.
func (g *Game) Visible(p Point) bool {
	switch {
	case g.Lit:
		return true
	case g.Dark:
		return chebyshev(g.Player, p) <= 1
	}
	return g.sees(g.Player, p)
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
	if g.Over != "" {
		switch action {
		case "redraw", "log":
		case "map":
			g.ShowMap = !g.ShowMap
		case "items":
			g.ShowItems = !g.ShowItems
		default:
			g.beginAction()
			g.say("冒険は終わった。")
		}
		return nil
	}
	// Any other action drops the choice the game was waiting for.
	switch action {
	case "redraw", "log", "map", "items", "choose", "cancel":
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
	case "foot":
		// Nor looking under the player.
		g.beginAction()
		g.openFoot()
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
		if !g.pickUp(false) {
			g.say("足元には何もない。")
			return nil
		}
		g.endTurn(false)
	case "map":
		// Showing the mini-map costs no turn.
		g.ShowMap = !g.ShowMap
	case "items":
		// Nor showing the items.
		g.ShowItems = !g.ShowItems
	case "look":
		// Looking costs no turn.
		g.beginAction()
		g.lookAround()
	case "move":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.beginAction()
		if g.Confused > 0 {
			name, d = g.confusedDirection()
		}
		g.Facing = name
		if g.Stuck > 0 && g.canStep(d) {
			g.struggle()
			return nil
		}
		if g.step(d, true) || g.Confused > 0 {
			// (Confused, a step into a wall costs the turn too.)
			g.endTurn(true)
		}
	case "turn":
		// Facing costs no turn. The axis shows until another action.
		if len(args) == 0 {
			g.beginAction()
			g.faceEnemy()
			g.Axis = true
			return nil
		}
		name, _, err := dir()
		if err != nil {
			return err
		}
		g.Facing = name
		g.Axis = true
	case "dash":
		name, d, err := dir()
		if err != nil {
			return err
		}
		g.beginAction()
		if g.Confused > 0 {
			// Confused, a dash is a step at random.
			name, d = g.confusedDirection()
			g.Facing = name
			g.step(d, true)
			g.endTurn(true)
			return nil
		}
		g.Facing = name
		g.dash(d)
	case "attack":
		g.beginAction()
		if g.Confused > 0 {
			g.Facing, _ = g.confusedDirection()
		}
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
	case "debug":
		// The debug mode costs no turn.
		g.beginAction()
		return g.debug(args)
	case "stairs":
		g.beginAction()
		if i := g.decoyAt(g.Player); i >= 0 {
			g.unmaskDecoy(i, true)
			g.endTurn(false)
			return nil
		}
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
		if m := g.MonsterAt(g.Player.Add(d)); m != nil && m.hostile() && g.clearFrom(g.Player, d) {
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
		if m == nil && g.clearFrom(g.Player, d) {
			g.check(g.Player.Add(d))
		}
		return true
	}
	m.Dozing = false
	g.provoke(m)
	r := g.Dungeon.Rules
	if !g.sealed(SealSure) && !hits(g.rng, r.PlayerHitRate) {
		g.say(m.Name + "にかわされた。")
		return true
	}
	n := damage(g.rng, g.Attack(), m.Defense, r.DefenseK)
	if c := g.sealStrength(SealCrit); c > 0 && g.rng.IntN(100) < c {
		n *= 2
		g.say("会心の一撃！")
	}
	g.use(EffectStrength)
	m.HP -= n
	g.say(fmt.Sprintf("%sに%dのダメージ。", m.Name, n))
	if m.HP > 0 {
		g.wounded(m)
		g.split(m)
	}
	if m.HP <= 0 {
		g.kill(m, nil, true, true)
	}
	return true
}

// endTurn ends a turn of the player: walking or waiting restores HP (other
// actions do not), and then the monsters act.
func (g *Game) endTurn(rest bool) {
	if g.Over != "" {
		// It ended in the action (a trap).
		return
	}
	if g.Hasted > 0 {
		g.Hasted--
		if g.Hasted == 0 {
			g.say("体の速さが元に戻った。")
		}
		g.hasteSkip = !g.hasteSkip
		if g.hasteSkip {
			// The monsters wait for the second action.
			return
		}
	}
	turns := 1
	if g.Slowed > 0 {
		turns = 2
		g.Slowed--
		if g.Slowed == 0 {
			g.say("体が軽くなった。")
		}
	}
	for range turns {
		if g.Over != "" {
			return
		}
		g.tick(rest)
	}
	for g.Asleep > 0 && g.Over == "" {
		g.Asleep--
		g.tick(true)
		if g.Asleep == 0 && g.Over == "" {
			g.say("目が覚めた。")
		}
	}
}

// tick is a turn passing: HP and fullness, the monsters, the floor.
func (g *Game) tick(rest bool) {
	if g.Confused > 0 {
		g.Confused--
		if g.Confused == 0 {
			g.say("混乱が治った。")
		}
	}
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
	g.showSensed()
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
		def := d.Monster(pick(g.rng, d.MonsterRows, g.Depth))
		if a, ok := mimicOf(def); ok {
			g.spawnHidden(def, a)
		} else {
			g.spawn(def)
		}
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
			if !m.Ally && m.Def.Kind == "" {
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
	p, ok := g.find(g.wanderTarget, needNoCreature|needUnseen)
	if !ok {
		return nil
	}
	m := newMonster(def)
	m.Pos = p
	m.Spawned = true
	g.Monsters = append(g.Monsters, m)
	return m
}

// end ends the run, for why.
func (g *Game) end(why string, cleared bool) {
	if !cleared && g.spared() {
		return
	}
	g.Over, g.Cleared, g.Ended = why, cleared, time.Now()
	g.say(why + "。")
	g.event("end %v %s", cleared, why)
}

// event notes what happened, for the replay file: a line of words, the
// turn first. The events are written as comments, for the analysis of
// replays without replaying them, and to check that a replay replays the
// same (see Replay).
func (g *Game) event(format string, args ...any) {
	g.events = append(g.events, fmt.Sprintf("%d ", g.Turn)+fmt.Sprintf(format, args...))
}

// takeEvents returns the events since the last call, and forgets them.
func (g *Game) takeEvents() []string {
	e := g.events
	g.events = nil
	return e
}

// beginAction starts the messages of an action.
func (g *Game) beginAction() {
	g.Message = ""
	g.Axis = false
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
		if m.Keeper && !m.Angry {
			if g.Sealed {
				g.say("口が封じられていて話せない。")
				return false
			}
			g.talkTo(m)
			return false
		}
		if !m.Ally {
			g.say(m.Name + "がいる。")
			return false
		}
		// Change places with an ally.
		m.Pos = g.Player
	}
	// Entering or leaving a room wakes the monsters dozing in it.
	from := g.Level.RoomAt(g.Player)
	wasInShop := g.inShop(g.Player)
	g.Player = to
	g.Steps++
	if room := g.Level.RoomAt(to); room != from {
		g.wake(from)
		g.wake(room)
		g.enterHouse()
		g.enterShop(wasInShop)
	}
	g.look()
	if t := g.TrapAt(to); t != nil {
		depth := g.Depth
		if g.stepOn(t) && (g.Over != "" || g.Depth != depth || g.Player != to) {
			// It fell, went down, or was sent away.
			return true
		}
	} else {
		g.markSafe(to)
	}
	if it, ok := g.ItemAt(to); ok {
		// Walking onto an item picks it up; a dash only stops on it.
		if !pick || !g.pickUp(true) {
			g.say(g.Name(it) + "が落ちている。")
		}
	}
	if to == g.Stairs {
		g.say("階段がある。")
	}
	if i := g.decoyAt(to); i >= 0 {
		g.unmaskDecoy(i, true)
	}
	return true
}

// shove puts the player on to, as moved by a monster (pulled, blown away):
// as a step, but nothing is picked up.
func (g *Game) shove(to Point) {
	from := g.Level.RoomAt(g.Player)
	wasInShop := g.inShop(g.Player)
	g.Player = to
	if room := g.Level.RoomAt(to); room != from {
		g.wake(from)
		g.wake(room)
		g.enterHouse()
		g.enterShop(wasInShop)
	}
	g.look()
	if t := g.TrapAt(to); t != nil {
		if g.stepOn(t) {
			return
		}
	} else {
		g.markSafe(to)
	}
	if it, ok := g.ItemAt(to); ok {
		g.say(g.Name(it) + "が落ちている。")
	}
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
		// A dash stops before a trap the player knows.
		if g.foundTrapAt(g.Player.Add(d)) != nil {
			return
		}
		if g.Stuck > 0 {
			g.struggle()
			return
		}
		seen := g.visibleEnemies()
		hp := g.HP
		g.sprung = false
		if !g.step(d, false) {
			return
		}
		g.endTurn(true)
		if g.Over != "" || g.HP < hp || g.sprung {
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
		if m.hostile() && g.Visible(m.Pos) {
			ms = append(ms, m)
		}
	}
	return ms
}

// Status is the status line, under the map.
func (g *Game) Status() string {
	s := fmt.Sprintf("攻撃 %d  防御 %d  ちから %d/%d  ターン %d（この階 %d）  歩数 %d  時間 %s  向き %s  シード %s",
		g.Attack(), g.Defense(), g.CurrentStrength(), g.MaxStrength+g.braceletStrength(), g.Turn, g.FloorTurns, g.Steps,
		playTime(g.PlayTime()), directionArrows[g.Facing], g.Seed)
	if w := g.Dungeon.Rules.WindTurns; g.Wind > 0 && g.Wind < len(w) {
		s += fmt.Sprintf("  次の風まで %d", w[g.Wind]-g.FloorTurns)
	}
	for _, name := range g.States.names("口封じ") {
		s += "  " + name
	}
	if g.Debugged {
		s += "  デバッグ"
	}
	return s
}

// struggle is a turn caught in a snare: the player tries to move, and
// cannot.
func (g *Game) struggle() {
	g.Stuck--
	if g.Stuck == 0 {
		g.say("トラばさみから抜け出した。")
	} else {
		g.say("トラばさみにかかっていて動けない。")
	}
	g.endTurn(true)
}

// PlayTime is how long the run has been played: until now, or until its
// end.
func (g *Game) PlayTime() time.Duration {
	end := g.Ended
	if end.IsZero() {
		end = time.Now()
	}
	return g.Played + end.Sub(g.Started)
}

// playTime formats d as h:mm:ss.
func playTime(d time.Duration) string {
	d = d.Truncate(time.Second)
	return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}
