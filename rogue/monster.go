package rogue

import (
	"fmt"
	"slices"
)

// Speed is how often a creature acts.
type Speed int

const (
	// SpeedSlow acts every other turn.
	SpeedSlow Speed = iota
	// SpeedNormal acts once a turn.
	SpeedNormal
	// SpeedDouble acts twice a turn.
	SpeedDouble
	// SpeedMoveDouble acts twice a turn, but attacks once at most: it
	// moves fast and hits as a normal one.
	SpeedMoveDouble
)

// actions returns how many times a creature of speed s acts in turn.
func (s Speed) actions(turn int) int {
	switch s {
	case SpeedSlow:
		return turn % 2
	case SpeedDouble, SpeedMoveDouble:
		return 2
	}
	return 1
}

// attacks returns how many times a creature of speed s can attack in
// turn, standing next to its foe.
func (s Speed) attacks(turn int) int {
	if s == SpeedMoveDouble {
		return 1
	}
	return s.actions(turn)
}

// fast says whether s moves twice a turn.
func (s Speed) fast() bool { return s == SpeedDouble || s == SpeedMoveDouble }

// Monster is a creature on the floor: an enemy, or an ally of the player
// (the same creature on the other side).
type Monster struct {
	// Def is its kind.
	Def   *MonsterDef
	Pos   Point
	Glyph string
	Name  string
	// Rank is 1 to 4, shown as its color.
	Rank int
	Ally bool
	// Order is what the player told it, an ally ("": OrderFollow).
	Order   Order
	HP      int
	MaxHP   int
	Attack  int
	Defense int
	Speed   Speed
	// Dozing monsters do nothing until the player enters or leaves their
	// room, or they are attacked.
	Dozing bool
	// Spawned monsters came after the floor began: they give no rewards,
	// so that lingering to fight them is not worth it.
	Spawned bool
	// House says whether it was put in a monster house.
	House bool
	// Keeper says whether it is a shopkeeper, Angry whether it fights the
	// player (a thief); home is its place in the shop, door the door it
	// guards.
	Keeper, Angry bool
	home, door    Point
	// Hunters always know where the player is (the strong monster of the
	// second wind).
	Hunter bool
	// Enraged says whether its anger (AbilityAnger) raised its attack.
	Enraged bool
	// States are its states that pass with time (state.go).
	States
	// EXP is what defeating it gives. An ally has Gained EXP, and its
	// Level from them.
	EXP    int
	Gained int
	Level  int

	// Where it is heading: where it last saw its target, or a tile it
	// wanders to.
	target    Point
	hasTarget bool
	// loot is the item a mimic looked like, which it may drop; waits
	// says whether it waits its next action out (after its first
	// strike).
	loot  *Item
	waits bool
}

func newMonster(def *MonsterDef) *Monster {
	return &Monster{
		Def: def, Glyph: def.Glyph, Name: def.Name, Rank: def.Rank, Speed: def.Speed,
		HP: def.HP, MaxHP: def.HP, Attack: def.Attack, Defense: def.Defense,
		EXP: def.EXP, Level: 1,
	}
}

// around are the eight directions, in a fixed order (no map iteration:
// the same seed and actions must give the same run).
var around = []Point{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// clearFrom says whether nothing is in the way from p by d: a diagonal is
// not around a corner of rock. The rule is the same for every creature.
func (g *Game) clearFrom(p, d Point) bool {
	l := g.Level
	if d.X != 0 && d.Y != 0 {
		return l.At(p.Add(Point{d.X, 0})).Walkable() && l.At(p.Add(Point{0, d.Y})).Walkable()
	}
	return true
}

// occupied says whether a creature (the player included) stands on p.
func (g *Game) occupied(p Point) bool {
	return p == g.Player || g.MonsterAt(p) != nil
}

// sees says whether something on a sees what is on b: in the same room
// (walls and doorways included), or next to it.
func (g *Game) sees(a, b Point) bool {
	if chebyshev(a, b) <= 1 {
		return true
	}
	l := g.Level
	if i := l.RoomAt(a); i >= 0 && l.Rooms[i].Around(b) {
		return true
	}
	i := l.RoomAt(b)
	return i >= 0 && l.Rooms[i].Around(a)
}

// monstersAct lets every monster act, as many times as its speed allows.
func (g *Game) monstersAct() {
	// A copy: monsters may die while others act.
	for _, m := range slices.Clone(g.Monsters) {
		if m.waits {
			m.waits = false
			continue
		}
		for range m.States.actions(m.Speed.actions(g.Turn), g.Turn) {
			if g.Over != "" || !slices.Contains(g.Monsters, m) {
				break
			}
			if g.act(m) && m.Speed == SpeedMoveDouble {
				break
			}
		}
	}
}

// act is one action of m: attack a foe next to it, or step toward where it
// is heading. It says whether m attacked.
func (g *Game) act(m *Monster) bool {
	if m.Keeper && !m.Angry {
		g.actKeeper(m)
		return false
	}
	if m.Dozing {
		g.stir(m)
		return false
	}
	if m.Confused > 0 {
		return g.actConfused(m)
	}
	if m.Ally {
		return g.actAlly(m)
	}
	if g.erratic(m) {
		return false
	}

	// Attack the player first, then an ally.
	for _, d := range around {
		if m.Pos.Add(d) == g.Player && g.clearFrom(m.Pos, d) {
			g.hitPlayer(m)
			return true
		}
	}
	for _, d := range around {
		if f := g.MonsterAt(m.Pos.Add(d)); f != nil && f.Ally && g.clearFrom(m.Pos, d) {
			g.hitMonster(m, f)
			return true
		}
	}
	if g.reachAbility(m) {
		return true
	}

	if m.Hunter || g.sees(m.Pos, g.Player) {
		m.target, m.hasTarget = g.Player, true
	} else if !m.hasTarget || m.Pos == m.target {
		// Wander: head to a tile of a random room.
		m.target, m.hasTarget = g.wanderTarget(), true
	}
	if !g.stepToward(m, m.target) && !g.sees(m.Pos, g.Player) {
		// Stuck: go somewhere else next time.
		m.hasTarget = false
	}
	return false
}

// allyReach is how far from the player an ally goes to fight an enemy.
const allyReach = 5

// actAlly is one action of an ally: attack an enemy next to it, then as
// it was told: go for one it sees near the player or follow the player,
// go where it likes, or wait. It says whether the ally
// attacked.
func (g *Game) actAlly(m *Monster) bool {
	for _, d := range around {
		if f := g.MonsterAt(m.Pos.Add(d)); f != nil && f.hostile() && g.clearFrom(m.Pos, d) {
			g.hitMonster(m, f)
			return true
		}
	}
	switch m.order() {
	case OrderWait:
	case OrderFree:
		// Where it likes: to an enemy it sees, else wandering.
		if f := g.allyTarget(m); f != nil {
			m.target, m.hasTarget = f.Pos, true
		} else if !m.hasTarget || m.Pos == m.target {
			m.target, m.hasTarget = g.wanderTarget(), true
		}
		if !g.stepToward(m, m.target) {
			m.hasTarget = false
		}
	default:
		if f := g.allyTarget(m); f != nil && g.stepToward(m, f.Pos) {
			return false
		}
		if chebyshev(m.Pos, g.Player) > 1 {
			g.stepToward(m, g.Player)
		}
	}
	return false
}

// allyTarget returns the enemy the ally m goes for: the nearest awake one
// it sees (it lets sleeping ones sleep), and, unless it is free, not far
// from the player.
func (g *Game) allyTarget(m *Monster) *Monster {
	var best *Monster
	for _, f := range g.Monsters {
		if !f.hostile() || f.Dozing || !g.sees(m.Pos, f.Pos) ||
			m.order() != OrderFree && chebyshev(f.Pos, g.Player) > allyReach {
			continue
		}
		if best == nil || chebyshev(f.Pos, m.Pos) < chebyshev(best.Pos, m.Pos) {
			best = f
		}
	}
	return best
}

// wanderTarget returns a random tile of a random room.
func (g *Game) wanderTarget() Point {
	var rooms []Room
	for _, r := range g.Level.Rooms {
		if !r.Dummy {
			rooms = append(rooms, r)
		}
	}
	r := rooms[g.rng.IntN(len(rooms))]
	return Point{r.X + g.rng.IntN(r.W), r.Y + g.rng.IntN(r.H)}
}

// stepToward moves m one step along a shortest way to to, around other
// creatures; it says whether it moved.
func (g *Game) stepToward(m *Monster, to Point) bool {
	dist := g.distances(to)
	best, bestDist := Point{}, -1
	here, ok := g.index(m.Pos)
	if !ok || dist[here] < 0 {
		return false
	}
	for _, d := range around {
		p := m.Pos.Add(d)
		i, ok := g.index(p)
		if !ok || dist[i] < 0 || dist[i] >= dist[here] || g.occupied(p) || !g.clearFrom(m.Pos, d) {
			continue
		}
		if bestDist < 0 || dist[i] < bestDist {
			best, bestDist = p, dist[i]
		}
	}
	if bestDist < 0 {
		return false
	}
	m.Pos = best
	return true
}

// distances returns the number of steps from every tile to to (-1 where it
// cannot be reached), by the walkable tiles, diagonals around corners left
// out; creatures are not obstacles. The tiles alone decide it, so it is
// kept while they stay the same (the monsters going for the player, or
// wandering to the same tile, ask again and again): not to be changed by
// the caller.
func (g *Game) distances(to Point) []int {
	c := &g.distCache
	if c.level != g.Level || c.version != g.Level.version || len(c.dist) >= maxDistances {
		*c = distCache{level: g.Level, version: g.Level.version, dist: map[Point][]int{}}
	}
	if dist, ok := c.dist[to]; ok {
		return dist
	}
	dist := g.workOutDistances(to)
	c.dist[to] = dist
	return dist
}

// distCache keeps the distances on a level, as its tiles are, by where
// they lead.
type distCache struct {
	level   *Level
	version int
	dist    map[Point][]int
}

// maxDistances is how many distances distCache keeps at most (each is as
// big as the level): past it, it starts again.
const maxDistances = 64

// workOutDistances is distances, worked out.
func (g *Game) workOutDistances(to Point) []int {
	l := g.Level
	dist := make([]int, len(l.Tiles))
	for i := range dist {
		dist[i] = -1
	}
	start, ok := g.index(to)
	if !ok {
		return dist
	}
	dist[start] = 0
	queue := []Point{to}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		pi, _ := g.index(p)
		for _, d := range around {
			q := p.Add(d)
			qi, ok := g.index(q)
			if !ok || dist[qi] >= 0 || !l.At(q).Walkable() || !g.clearFrom(p, d) {
				continue
			}
			dist[qi] = dist[pi] + 1
			queue = append(queue, q)
		}
	}
	return dist
}

// hitPlayer is an attack of m on the player.
func (g *Game) hitPlayer(m *Monster) {
	r := g.Dungeon.Rules
	if !hits(g.rng, r.MonsterHitRate) {
		g.say(m.Name + "の攻撃は外れた。")
		return
	}
	n := g.critical(m, damage(g.rng, m.Attack, g.Defense(), r.DefenseK), true)
	g.HP -= n
	g.say(fmt.Sprintf("%sの攻撃。%dのダメージ。", m.Name, n))
	g.event("hurt %d %s", n, m.Def.ID)
	if g.HP <= 0 {
		g.HP = 0
		g.end(m.Name+"に倒された", false)
		return
	}
	if c := g.sealStrength(SealCounter); c > 0 && g.rng.IntN(100) < c {
		g.say("反撃した！")
		g.hurt(m, n)
	}
}

// hitMonster is an attack of a monster on another (an ally and an enemy).
func (g *Game) hitMonster(a, m *Monster) {
	m.Dozing = false
	shown := g.sees(g.Player, a.Pos) || g.sees(g.Player, m.Pos)
	r := g.Dungeon.Rules
	rate := r.MonsterHitRate
	if a.Ally {
		rate = r.AllyHitRate
	}
	if !hits(g.rng, rate) {
		if shown {
			g.say(a.Name + "の攻撃は外れた。")
		}
		return
	}
	n := g.critical(a, damage(g.rng, a.Attack, m.Defense, r.DefenseK), shown)
	m.HP -= n
	if shown {
		g.say(fmt.Sprintf("%sは%sに%dのダメージ。", a.Name, m.Name, n))
	}
	if m.HP > 0 {
		g.wounded(m)
	}
	if m.HP <= 0 {
		g.remove(m)
		if shown {
			g.say(m.Name + "は倒れた。")
		}
		if a.Ally {
			g.defeated(m)
		}
		g.killed(a, m, shown)
	}
}

// defeated is an enemy defeated by the player or an ally: its EXP goes,
// whole, to the player and to every ally. Spawned monsters give nothing.
func (g *Game) defeated(m *Monster) {
	g.Kills++
	g.event("kill %s", m.Def.ID)
	if m.Spawned || m.EXP == 0 {
		return
	}
	g.dropFrom(m)
	g.EXP += m.EXP
	growth := g.Dungeon.Growth[g.Dungeon.Rules.Growth]
	for g.PlayerLevel-1 < len(growth) && g.EXP >= growth[g.PlayerLevel-1].EXP {
		gr := growth[g.PlayerLevel-1]
		g.PlayerLevel++
		g.MaxHP += gr.HP
		g.HP += gr.HP
		g.event("level %d", g.PlayerLevel)
		g.say(fmt.Sprintf("主はレベル%dに上がった。", g.PlayerLevel))
	}
	for _, a := range g.Monsters {
		if !a.Ally {
			continue
		}
		a.Gained += m.EXP
		growth := g.Dungeon.Growth[a.Def.Growth]
		for a.Level-1 < len(growth) && a.Gained >= growth[a.Level-1].EXP {
			gr := growth[a.Level-1]
			a.Level++
			a.MaxHP += gr.HP
			a.HP += gr.HP
			a.Attack += gr.Attack
			a.Defense += gr.Defense
			g.say(fmt.Sprintf("%sはレベル%dに上がった。", a.Name, a.Level))
		}
	}
}

// rankUp makes m the kind of its family one rank above, if there is one,
// its HP in proportion. Its EXP stays what it was worth.
func (g *Game) rankUp(m *Monster) {
	next := g.Dungeon.nextRank(m.Def)
	if next == nil {
		return
	}
	m.Def, m.Name, m.Rank, m.Speed, m.Enraged = next, next.Name, next.Rank, next.Speed, false
	m.HP = max(m.HP*next.HP/m.MaxHP, 1)
	m.MaxHP, m.Attack, m.Defense = next.HP, next.Attack, next.Defense
}

// wake wakes the dozing monsters in the room i.
func (g *Game) wake(i int) {
	if i < 0 {
		return
	}
	r := g.Level.Rooms[i]
	for _, m := range g.Monsters {
		if r.Contains(m.Pos) {
			m.Dozing = false
		}
	}
}

func (g *Game) remove(m *Monster) {
	g.Monsters = slices.DeleteFunc(g.Monsters, func(x *Monster) bool { return x == m })
	if m.Keeper && g.Shop != nil {
		g.Shop.Keepers = slices.DeleteFunc(g.Shop.Keepers, func(x *Monster) bool { return x == m })
		if len(g.Shop.Keepers) == 0 {
			g.closeShop()
		}
	}
}

// dropFrom is m, killed by the player, dropping an item by chance where
// it fell: as its kind says (monsters.txt 落とし物), or by the dungeon's
// rule (an item of the floor's item table).
func (g *Game) dropFrom(m *Monster) {
	d := g.Dungeon
	what, rate := DropItem, d.Rules.DropRate
	if m.Def.Drop != nil {
		what, rate = m.Def.Drop.What, m.Def.Drop.Rate
	}
	if what == DropNone || rate == 0 || g.rng.IntN(100) >= rate {
		return
	}
	if it := m.loot; it != nil {
		// A mimic: the item it looked like (the shop's, in the shop).
		it.ForSale = it.ForSale && g.inShop(m.Pos)
		g.land(it, m.Pos)
		return
	}
	var def *ItemDef
	switch what {
	case DropItem:
		def = d.ItemTable(d.counts(g.Depth).ItemTable).pick(g.rng, d, g.Depth)
	case DropMoney:
		for _, it := range d.Items {
			if it.Category == CategoryMoney {
				def = it
				break
			}
		}
	default:
		if c := Category(what); slices.Contains(categories, c) {
			def = d.ItemTable(d.counts(g.Depth).ItemTable).pickOf(g.rng, d, g.Depth, c)
		} else {
			def = d.Item(what)
		}
	}
	if def == nil {
		return
	}
	g.land(g.newItemOf(g.rng, def, m.Pos), m.Pos)
}
