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
)

// actions returns how many times a creature of speed s acts in turn.
func (s Speed) actions(turn int) int {
	switch s {
	case SpeedSlow:
		return turn % 2
	case SpeedDouble:
		return 2
	}
	return 1
}

// Monster is a creature on the floor: an enemy, or an ally of the player
// (the same creature on the other side).
type Monster struct {
	Pos   Point
	Glyph string
	Name  string
	// Rank is 1 to 4, shown as its color.
	Rank    int
	Ally    bool
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
	// Hunters always know where the player is (the strong monster of the
	// second wind).
	Hunter bool

	// Where it is heading: where it last saw its target, or a tile it
	// wanders to.
	target    Point
	hasTarget bool
}

// monsterKind is a kind of monster of the prototype, until monsters are
// data.
type monsterKind struct {
	Glyph, Name string
	Speed       Speed
}

var monsterKinds = []monsterKind{
	{"鼠", "鼠", SpeedNormal}, {"蛇", "蛇", SpeedNormal}, {"狼", "狼", SpeedDouble},
	{"骸", "骸骨", SpeedSlow}, {"霊", "亡霊", SpeedNormal}, {"鬼", "鬼", SpeedNormal},
	{"竜", "竜", SpeedNormal}, {"盗", "盗賊", SpeedNormal},
}

func newMonster(k monsterKind, rank int) *Monster {
	s := rankStats[rank]
	return &Monster{
		Glyph: k.Glyph, Name: k.Name, Rank: rank, Speed: k.Speed,
		HP: s.HP, MaxHP: s.HP, Attack: s.Attack, Defense: s.Defense,
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
		for range m.Speed.actions(g.Turn) {
			if g.Over != "" || !slices.Contains(g.Monsters, m) {
				break
			}
			g.act(m)
		}
	}
}

// act is one action of m: attack a foe next to it, or step toward where it
// is heading.
func (g *Game) act(m *Monster) {
	if m.Dozing {
		return
	}
	if m.Ally {
		g.actAlly(m)
		return
	}

	// Attack the player first, then an ally.
	for _, d := range around {
		if m.Pos.Add(d) == g.Player && g.clearFrom(m.Pos, d) {
			g.hitPlayer(m)
			return
		}
	}
	for _, d := range around {
		if f := g.MonsterAt(m.Pos.Add(d)); f != nil && f.Ally && g.clearFrom(m.Pos, d) {
			g.hitMonster(m, f)
			return
		}
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
}

// actAlly is one action of an ally: attack an enemy next to it, or follow
// the player.
func (g *Game) actAlly(m *Monster) {
	for _, d := range around {
		if f := g.MonsterAt(m.Pos.Add(d)); f != nil && !f.Ally && g.clearFrom(m.Pos, d) {
			g.hitMonster(m, f)
			return
		}
	}
	if chebyshev(m.Pos, g.Player) > 1 {
		g.stepToward(m, g.Player)
	}
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
// out; creatures are not obstacles.
func (g *Game) distances(to Point) []int {
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
	if !hits(g.rng, monsterHitRate) {
		g.say(m.Name + "の攻撃は外れた。")
		return
	}
	n := damage(g.rng, m.Attack, 0)
	g.HP -= n
	g.say(fmt.Sprintf("%sの攻撃。%dのダメージ。", m.Name, n))
	if g.HP <= 0 {
		g.HP = 0
		g.Over = m.Name + "に倒された"
		g.say(g.Over + "。")
	}
}

// hitMonster is an attack of a monster on another (an ally and an enemy).
func (g *Game) hitMonster(a, m *Monster) {
	m.Dozing = false
	shown := g.sees(g.Player, a.Pos) || g.sees(g.Player, m.Pos)
	if !hits(g.rng, monsterHitRate) {
		if shown {
			g.say(a.Name + "の攻撃は外れた。")
		}
		return
	}
	n := damage(g.rng, a.Attack, m.Defense)
	m.HP -= n
	if shown {
		g.say(fmt.Sprintf("%sは%sに%dのダメージ。", a.Name, m.Name, n))
	}
	if m.HP <= 0 {
		g.remove(m)
		if shown {
			g.say(m.Name + "は倒れた。")
		}
	}
}

func (g *Game) remove(m *Monster) {
	g.Monsters = slices.DeleteFunc(g.Monsters, func(x *Monster) bool { return x == m })
}

// rankUp makes m one rank stronger (up to 4), its HP in proportion.
func rankUp(m *Monster) {
	if m.Rank >= len(rankStats)-1 {
		return
	}
	m.Rank++
	s := rankStats[m.Rank]
	m.HP = max(m.HP*s.HP/m.MaxHP, 1)
	m.MaxHP, m.Attack, m.Defense = s.HP, s.Attack, s.Defense
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
