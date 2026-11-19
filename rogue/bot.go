package rogue

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
)

// BotStyle is how a bot plays.
type BotStyle int

const (
	// BotCareful explores, rests to heal, heals early and leaves a floor
	// when it is explored or the wind comes.
	BotCareful BotStyle = iota
	// BotReckless never rests, heals late and goes down as soon as it
	// finds the stairs.
	BotReckless
)

// BotStyles are the styles by name.
var BotStyles = map[string]BotStyle{"careful": BotCareful, "reckless": BotReckless}

func (s BotStyle) String() string {
	for name, t := range BotStyles {
		if t == s {
			return name
		}
	}
	return fmt.Sprint(int(s))
}

// Bot plays a run by rules, through Do only, like a player: it knows what
// the player knows (the tiles seen, the monsters in sight, the items seen,
// the kinds identified), not the floor itself. It is meant to measure the
// balance of dungeons (cmd/analyze), not to play well.
type Bot struct {
	Style BotStyle
	rng   *rand.Rand
	// plan is what to answer to the prompts it opened: an item of the
	// inventory, then a command.
	plan []botStep
	// stuck counts the actions without moving, to get out of a loop.
	last  Point
	stuck int
	// risks remembers whether fights one on one are dangerous.
	risks map[riskKey]bool
	// lastRisk is the chance of dying of the fight with the enemies
	// next to the player at its last choice (0 with none); fightRisk at
	// the first choice of this fight (since the player last had no enemy
	// next to it).
	lastRisk, fightRisk float64
	// worst remembers worstTurn.
	worst map[worstKey]int
	// depth is the floor the bot is on, arrived the turn it arrived;
	// floors and floorsTurns count the floors it left and the turns it
	// spent on them.
	depth, arrived, floors, floorsTurns int
	// answersIn are the answers carried on arriving on this floor;
	// consumed counts the grasses and scrolls used on it, spent the
	// answers used against a danger.
	answersIn, consumed, spent int
}

// botStep is an answer to a prompt: the item it (in the inventory), or
// the choice labeled label.
type botStep struct {
	item  *Item
	label string
}

// NewBot returns a bot of style, with its randomness from seed.
func NewBot(style BotStyle, seed uint64) *Bot {
	return &Bot{Style: style, rng: rand.New(rand.NewPCG(seed, 0x60da)), risks: map[riskKey]bool{}, worst: map[worstKey]int{}}
}

// Next returns the next action of the bot in g, and its arguments.
func (b *Bot) Next(g *Game) (string, []string) {
	if g.Prompt != nil {
		return b.answer(g)
	}
	b.plan = nil
	b.track(g)
	if b.lastRisk = deathChance(g, b.adjacent(g)); b.lastRisk == 0 {
		b.fightRisk = 0
	} else if b.fightRisk == 0 {
		b.fightRisk = b.lastRisk
	}
	if g.Player == b.last {
		b.stuck++
	} else {
		b.last, b.stuck = g.Player, 0
	}
	if b.stuck > 30 {
		// Going round in circles: a random step.
		b.stuck = 0
		return "move", []string{clockwise[b.rng.IntN(len(clockwise))]}
	}
	for _, rule := range []func(*Game) (string, []string){
		b.heal, b.danger, b.shoot, b.fight, b.eat, b.boost, b.equip, b.identify, b.light,
		b.tidy, b.rest, b.leave, b.pick, b.explore,
	} {
		if action, args := rule(g); action != "" {
			return action, args
		}
	}
	if g.IsSeen(g.Stairs) {
		if action, args := b.toStairs(g); action != "" {
			return action, args
		}
	}
	return "wait", nil
}

// answer answers the prompt of g by the plan, or cancels it.
func (b *Bot) answer(g *Game) (string, []string) {
	p := g.Prompt
	if p.kind == promptIdentify && len(p.Choices) > 0 {
		return "choose", []string{"0"}
	}
	if len(b.plan) == 0 {
		return "cancel", nil
	}
	step := b.plan[0]
	b.plan = b.plan[1:]
	i := -1
	switch {
	case step.item != nil && p.kind == promptInventory:
		i = slices.Index(p.items, slices.Index(g.Inventory, step.item))
	case step.item == nil:
		i = slices.Index(p.Choices, step.label)
	}
	if i < 0 {
		b.plan = nil
		return "cancel", nil
	}
	return "choose", []string{fmt.Sprint(i)}
}

// use plans to do command with it (carried): the inventory, it, command.
func (b *Bot) use(it *Item, command string) (string, []string) {
	b.plan = []botStep{{item: it}, {label: command}}
	if c := it.Def.Category; c == CategoryGrass || c == CategoryScroll {
		b.consumed++
	}
	return "inventory", nil
}

// carried returns the first item carried that f accepts, or nil.
func carried(g *Game, f func(*Item) bool) *Item {
	for _, it := range g.Inventory {
		if !it.ForSale && f(it) {
			return it
		}
	}
	return nil
}

// known says whether the kind of it is identified, with the effect e.
func (b *Bot) known(g *Game, it *Item, e Effect) bool {
	return g.identified[it.Def.ID] && it.Def.Effect == e
}

// heal drinks the cheapest healing grass known when no enemy is next to
// the player (danger answers those) and its HP is below what the monsters
// of the floor could take in two turns at worst, unless the careful bot
// can rest instead.
func (b *Bot) heal(g *Game) (string, []string) {
	if len(b.adjacent(g)) > 0 || g.HP > 2*b.worstTurn(g) {
		return "", nil
	}
	if b.Style == BotCareful && b.canRest(g) {
		return "", nil
	}
	var best *Item
	for _, it := range g.Inventory {
		if !it.ForSale && b.known(g, it, EffectHeal) && (best == nil || it.Def.Price < best.Def.Price) {
			best = it
		}
	}
	if best == nil {
		return "", nil
	}
	return b.use(best, commandDrink)
}

// worstTurn returns the most damage a monster of the floor (by the table)
// can do to the player in a turn, its speed counted: what a player who
// knows the table fears.
func (b *Bot) worstTurn(g *Game) int {
	key := worstKey{g.Depth, g.Defense()}
	if w, ok := b.worst[key]; ok {
		return w
	}
	w := 0
	for _, r := range g.Dungeon.MonsterRows {
		def := g.Dungeon.Monster(r.ID)
		if r.First > g.Depth || g.Depth > r.Last || def == nil {
			continue
		}
		m := newMonster(def)
		n := len(hitDamages(g, m)) - 1
		if m.Speed == SpeedDouble {
			n *= 2
		}
		w = max(w, n)
	}
	b.worst[key] = w
	return w
}

// worstKey is what worstTurn depends on.
type worstKey struct{ depth, defense int }

// Death chances. A bot judges a fight as the article "シレン5で上達する
// ための思考法" (platinasun) says: by the chance of dying, not by the
// expected damage. A careful bot answers a fight it would lose at
// carefulRisk or more, a reckless one at recklessRisk.
const (
	carefulRisk  = 0.05
	recklessRisk = 0.20
	// riskTurns caps the turns of a fight reckoned.
	riskTurns = 50
)

// risk returns the chance of dying the bot accepts.
func (b *Bot) risk() float64 {
	if b.Style == BotReckless {
		return recklessRisk
	}
	return carefulRisk
}

// killTurns returns the turns the player expects to take to kill m, as a
// player who knows the kinds of monsters (not their HP) would reckon.
func killTurns(g *Game, m *Monster) float64 {
	r := g.Dungeon.Rules
	k := float64(r.DefenseK)
	deal := float64(g.Attack()) * k / (k + float64(m.Defense)) * float64(r.PlayerHitRate) / 100
	return float64(m.MaxHP) / max(deal, 0.1)
}

// hitDamages returns the chances of the damage of an attack of m on the
// player, by the damage (index 0 a miss), by the combat formula: a hit
// rate, then a damage spread evenly over +-1/8, rounded, at least 1.
func hitDamages(g *Game, m *Monster) []float64 {
	r := g.Dungeon.Rules
	hit := float64(r.MonsterHitRate) / 100
	c := float64(m.Attack) * float64(r.DefenseK) / float64(r.DefenseK+g.Defense())
	const lo, hi = 0.875, 1.125
	top := max(int(c*hi+0.5), 1)
	p := make([]float64, top+1)
	p[0] = 1 - hit
	for n := 0; n <= top; n++ {
		// c*f+0.5 in [n, n+1): f in [(n-0.5)/c, (n+0.5)/c).
		from, to := lo, hi
		if c > 0 {
			from, to = max((float64(n)-0.5)/c, lo), min((float64(n)+0.5)/c, hi)
		} else if n > 0 {
			continue
		}
		if to > from {
			p[max(n, 1)] += hit * (to - from) / (hi - lo)
		}
	}
	return p
}

// deathChance returns the chance that the player dies fighting the
// enemies ms (next to it), as a player who knows the kinds would reckon:
// it kills them one by one, the quickest first, in the turns it expects
// (rounded up, which leaves a turn for a miss), and each of them attacks
// on every turn it acts until it is killed (its speed counted).
func deathChance(g *Game, ms []*Monster) float64 {
	return fightDeath(g, ms, g.HP, 0)
}

// foe is an enemy in a reckoning of the chance of dying: the chances of
// the damage of its attacks, and the first and last turns it attacks (a
// first of 0 or 1: from the start).
type foe struct {
	hits        []float64
	first, last int
	m           *Monster
}

// fightDeath returns the chance that the player with hp dies fighting
// the enemies ms, after delay turns of doing something else (using an
// item) while they attack. An enemy not next to the player joins the
// fight when it comes (arrival); the player kills them one by one as
// they are there, the quickest first.
func fightDeath(g *Game, ms []*Monster, hp, delay int) float64 {
	if len(ms) == 0 {
		return 0
	}
	foes := make([]foe, len(ms))
	for i, m := range ms {
		a := arrival(g, m)
		foes[i] = foe{hitDamages(g, m), a + 1, 0, m}
	}
	slices.SortStableFunc(foes, func(a, c foe) int {
		if a.first != c.first {
			return a.first - c.first
		}
		return cmpFloat(killTurns(g, a.m), killTurns(g, c.m))
	})
	t := float64(delay)
	for i := range foes {
		t = max(t, float64(foes[i].first-1)) + killTurns(g, foes[i].m)
		foes[i].last = min(int(math.Ceil(t)), riskTurns)
	}
	return deathDP(g, hp, foes)
}

// arrival returns the turns m takes to come next to the player (0 next to
// it), by its distance and speed.
func arrival(g *Game, m *Monster) int {
	steps := chebyshev(m.Pos, g.Player) - 1
	switch m.Speed {
	case SpeedDouble:
		return (steps + 1) / 2
	case SpeedSlow:
		return 2 * steps
	}
	return steps
}

// comeReach is how far an enemy in sight may be to count in a fight: it
// comes within a few turns.
const comeReach = 5

// threats returns the enemies of a fight: those next to the player, and
// those awake in sight within comeReach, coming.
func (b *Bot) threats(g *Game) []*Monster {
	ms := b.adjacent(g)
	for _, m := range g.visibleEnemies() {
		if d := chebyshev(m.Pos, g.Player); d > 1 && d <= comeReach && !m.Dozing {
			ms = append(ms, m)
		}
	}
	return ms
}

// oneByOneDeath returns the chance that the player with hp dies fighting
// the enemies ms one at a time (from a corridor, where only one can stand
// next to it), after a step back while they all attack.
func oneByOneDeath(g *Game, ms []*Monster, hp int) float64 {
	ms = slices.Clone(ms)
	foes := make([]foe, 0, 2*len(ms))
	for _, m := range ms {
		if arrival(g, m) == 0 {
			foes = append(foes, foe{hitDamages(g, m), 1, 1, m})
		}
	}
	slices.SortStableFunc(ms, func(a, c *Monster) int {
		return cmpFloat(killTurns(g, a), killTurns(g, c))
	})
	sum := 1.0
	for _, m := range ms {
		first := int(math.Ceil(sum)) + 1
		sum += killTurns(g, m)
		foes = append(foes, foe{hitDamages(g, m), first, min(int(math.Ceil(sum)), riskTurns), m})
	}
	return deathDP(g, hp, foes)
}

// fleeDeath returns the chance that the player with hp dies walking
// turns steps away from the enemies ms next to it: an enemy of normal
// speed follows without attacking, a fast one attacks once a turn, a
// slow one is left behind.
func fleeDeath(g *Game, ms []*Monster, hp, turns int) float64 {
	var foes []foe
	for _, m := range ms {
		if m.Speed == SpeedDouble {
			// Its second action is the attack: as a normal one
			// standing.
			slow := *m
			slow.Speed = SpeedNormal
			foes = append(foes, foe{hitDamages(g, m), 0, min(turns, riskTurns), &slow})
		}
	}
	return deathDP(g, hp, foes)
}

// deathDP returns the chance that the player with hp dies to the attacks
// of foes, each attacking on the turns it acts up to its last.
func deathDP(g *Game, hp int, foes []foe) float64 {
	if len(foes) == 0 {
		return 0
	}
	if hp <= 0 {
		return 1
	}
	end := 0
	for _, f := range foes {
		end = max(end, f.last)
	}
	// alive[h] is the chance of having h HP left (and being alive).
	alive := make([]float64, hp+1)
	alive[hp] = 1
	next := make([]float64, hp+1)
	dead := 0.0
	for t := 1; t <= end; t++ {
		for _, f := range foes {
			if t < f.first || t > f.last {
				continue
			}
			for range f.m.Speed.actions(g.Turn + t) {
				clear(next)
				for h, p := range alive {
					if p == 0 {
						continue
					}
					for d, q := range f.hits {
						if h-d <= 0 {
							dead += p * q
						} else {
							next[h-d] += p * q
						}
					}
				}
				alive, next = next, alive
			}
		}
	}
	return dead
}

// cmpFloat compares a and c.
func cmpFloat(a, c float64) int {
	switch {
	case a < c:
		return -1
	case a > c:
		return 1
	}
	return 0
}

// dangerous says whether fighting the enemies ms next to the player
// would kill it at the chance the bot accepts or more.
func (b *Bot) dangerous(g *Game, ms []*Monster) bool {
	if len(ms) == 1 {
		// One on one is asked of every monster in sight and of the
		// monster table: remembered.
		m := ms[0]
		key := riskKey{m.Def, m.MaxHP, m.Attack, m.Defense, g.HP, g.Attack(), g.Defense(), g.Turn % 2}
		if d, ok := b.risks[key]; ok {
			return d
		}
		if len(b.risks) > 4096 {
			clear(b.risks)
		}
		d := deathChance(g, ms) >= b.risk()
		b.risks[key] = d
		return d
	}
	return deathChance(g, ms) >= b.risk()
}

// riskKey is what the chance of dying one on one depends on.
type riskKey struct {
	def                    *MonsterDef
	maxHP, attack, defense int
	hp, pattack, pdefense  int
	parity                 int
}

// danger answers enemies next to the player that would kill it at the
// chance the bot accepts or more, by the cheapest answer that is enough
// (the article "シレン5で上達するための思考法": the cheapest move that is
// enough, not always the best one): the stairs under it, a healing grass,
// a staff of blowing away (or an unknown grass or staff, for what it may
// be), walking away to the stairs. The cost of an item is its price (the
// mean of the kinds it may be); a wave, a quarter of a staff's price;
// leaving the floor, flatly costly for the careful bot. With no answer
// enough, the one that leaves the least chance of dying, if it beats
// fighting on.
func (b *Bot) danger(g *Game) (string, []string) {
	near := b.adjacent(g)
	ms := b.threats(g)
	if len(near) == 0 && len(ms) < swarm {
		return "", nil
	}
	fight := fightDeath(g, ms, g.HP, 0)
	if fight < b.risk() {
		return "", nil
	}
	if g.Player == g.Stairs {
		return "stairs", nil
	}
	var answers []botAnswer
	seen := map[*ItemDef]bool{}
	for _, it := range g.Inventory {
		if it.ForSale || seen[it.Def] {
			continue
		}
		seen[it.Def] = true
		switch it.Def.Category {
		case CategoryGrass:
			kinds := b.mayBe(g, it)
			var chance float64
			for _, def := range kinds {
				if def.Effect == EffectHeal {
					chance += fightDeath(g, ms, min(g.HP+def.Strength, g.MaxHP), 1)
				} else {
					chance += fightDeath(g, ms, g.HP, 1)
				}
			}
			if len(kinds) > 0 {
				answers = append(answers, botAnswer{b.itemCost(g, it, kinds), chance / float64(len(kinds)), it, commandDrink, nil})
			}
		case CategoryStaff:
			if it.Known && it.Charges == 0 {
				continue
			}
			kinds := b.mayBe(g, it)
			// Blow away the enemy next to it whose leaving helps
			// most.
			if len(near) == 0 {
				continue
			}
			var target *Monster
			away := fight
			for _, m := range near {
				rest := slices.DeleteFunc(slices.Clone(ms), func(o *Monster) bool { return o == m })
				if c := fightDeath(g, rest, g.HP, 1); target == nil || c < away {
					target, away = m, c
				}
			}
			var chance float64
			for _, def := range kinds {
				if def.Effect == EffectBlowAway {
					chance += away
				} else {
					chance += fightDeath(g, ms, g.HP, 1)
				}
			}
			if len(kinds) > 0 {
				answers = append(answers, botAnswer{b.itemCost(g, it, kinds), chance / float64(len(kinds)), it, commandWave, target})
			}
		}
	}
	if len(ms) > 1 {
		if d, ok := b.stepBack(g, ms); ok {
			answers = append(answers, botAnswer{0, oneByOneDeath(g, ms, g.HP), nil, "move " + d, nil})
		}
	}
	if g.IsSeen(g.Stairs) {
		if step, dist, ok := b.route(g, func(p Point) bool { return p == g.Stairs }); ok && g.MonsterAt(step) == nil {
			cost := 0.0
			if b.Style == BotCareful {
				cost = leaveCost
			}
			answers = append(answers, botAnswer{cost, fleeDeath(g, near, g.HP, dist), nil, "", nil})
		}
	}
	var pick *botAnswer
	for i, a := range answers {
		if a.chance < b.risk() && (pick == nil || a.cost < pick.cost) {
			pick = &answers[i]
		}
	}
	if pick == nil {
		for i, a := range answers {
			if a.chance < fight && (pick == nil || a.chance < pick.chance) {
				pick = &answers[i]
			}
		}
	}
	switch {
	case pick == nil:
		return "", nil
	case strings.HasPrefix(pick.command, "move "):
		return "move", []string{strings.TrimPrefix(pick.command, "move ")}
	case pick.item == nil:
		return b.toStairs(g)
	case pick.target != nil:
		if name := directionName(pick.target.Pos.Sub(g.Player)); g.Facing != name {
			return "turn", []string{name}
		}
	}
	b.spent++
	return b.use(pick.item, pick.command)
}

// stepBack returns the direction of a step back into a corridor, where
// the enemies ms next to the player can come at it only one at a time
// (the articles on 浜辺の魔洞, shiren2424: two enemies next to you in a
// room, step back to make it one on one): a corridor tile it can step
// to, with at most two ways on, next to at most one of ms.
func (b *Bot) stepBack(g *Game, ms []*Monster) (string, bool) {
	for _, name := range clockwise {
		d := Directions[name]
		p := g.Player.Add(d)
		if !g.canStep(d) || g.MonsterAt(p) != nil || g.Level.RoomAt(p) >= 0 {
			continue
		}
		ways := 0
		for _, e := range Directions {
			if q := p.Add(e); g.Level.At(q).Walkable() && g.clearFrom(p, e) {
				ways++
			}
		}
		near := 0
		for _, m := range ms {
			if chebyshev(m.Pos, p) == 1 {
				near++
			}
		}
		if ways <= 2 && near <= 1 {
			return name, true
		}
	}
	return "", false
}

// unknownCharges are the charges the bot counts on in a staff whose
// charges it does not know.
const unknownCharges = 3

// itemCost returns what using it (one of kinds, as the player knows)
// costs the bot: the mean price of the kinds, for a staff a wave of it
// (the last one costs the whole staff), lower the more of the kind it
// carries, and lower the fewer floors are left to need it (the article
// "原始に続く穴 安定攻略への道": keep what the floors ahead need; an item
// left at the bottom is worth nothing).
func (b *Bot) itemCost(g *Game, it *Item, kinds []*ItemDef) float64 {
	cost := meanPrice(kinds)
	if it.Def.Category == CategoryStaff {
		charges := unknownCharges
		if it.Known {
			charges = it.Charges
		}
		cost /= float64(max(charges, 1))
	} else {
		n := 0
		for _, o := range g.Inventory {
			if !o.ForSale && o.Def == it.Def {
				n++
			}
		}
		cost *= 2 / float64(n+1)
	}
	d := g.Dungeon.Floors
	return cost * (0.25 + 0.75*float64(d-g.Depth)/float64(max(d-1, 1)))
}

// swarm is how many enemies coming at the player make a fight to answer
// before they are next to it (a monster house awake).
const swarm = 3

// leaveCost is what leaving a floor at once costs the careful bot, as the
// price of an item: what it would have found there.
const leaveCost = 100

// botAnswer is an answer to a danger: its cost, the chance of dying after
// it, and how: the item and its command (none: walking to the stairs, or
// a command "move <direction>": a step), at the target.
type botAnswer struct {
	cost, chance float64
	item         *Item
	command      string
	target       *Monster
}

// mayBe returns the kinds it may be, as the player knows: its kind when
// identified, else the kinds of its category not identified yet.
func (b *Bot) mayBe(g *Game, it *Item) []*ItemDef {
	if g.identified[it.Def.ID] {
		return []*ItemDef{it.Def}
	}
	var kinds []*ItemDef
	for _, def := range g.Dungeon.Items {
		if def.Category == it.Def.Category && !g.identified[def.ID] {
			kinds = append(kinds, def)
		}
	}
	return kinds
}

// meanPrice returns the mean price of kinds.
func meanPrice(kinds []*ItemDef) float64 {
	t := 0
	for _, def := range kinds {
		t += def.Price
	}
	return float64(t) / float64(max(len(kinds), 1))
}

// shoot shoots arrows at an enemy in a line from the player, not next to
// it, within an arrow's range, nothing in between.
func (b *Bot) shoot(g *Game) (string, []string) {
	arrows := g.equipped(CategoryArrow)
	if arrows == nil {
		return "", nil
	}
	for _, m := range g.visibleEnemies() {
		if arrows.Count < plentyArrows && !b.worthArrows(g, m) {
			continue
		}
		d := m.Pos.Sub(g.Player)
		n := max(abs(d.X), abs(d.Y))
		if n < 2 || n > throwRange || (d.X != 0 && d.Y != 0 && abs(d.X) != abs(d.Y)) {
			continue
		}
		step := Point{sign(d.X), sign(d.Y)}
		clear := true
		for p := g.Player; p.Add(step) != m.Pos; p = p.Add(step) {
			q := p.Add(step)
			if !g.clearFrom(p, step) || !g.Level.At(q).Walkable() || g.MonsterAt(q) != nil {
				clear = false
				break
			}
		}
		if !clear || !g.clearFrom(m.Pos.Sub(step), step) {
			continue
		}
		if name := directionName(step); g.Facing != name {
			return "turn", []string{name}
		}
		return "shoot", nil
	}
	return "", nil
}

// boost drinks the known grasses that only do good (strength).
func (b *Bot) boost(g *Game) (string, []string) {
	if it := carried(g, func(it *Item) bool { return b.known(g, it, EffectStrength) && it.Def.Category == CategoryGrass }); it != nil {
		return b.use(it, commandDrink)
	}
	return "", nil
}

// light reads a known scroll of light when the stairs are not found and
// the floor is explored as far as it can.
func (b *Bot) light(g *Game) (string, []string) {
	if g.IsSeen(g.Stairs) || g.Lit {
		return "", nil
	}
	if action, _ := b.explore(g); action != "" {
		return "", nil
	}
	if it := carried(g, func(it *Item) bool { return b.known(g, it, EffectLight) }); it != nil {
		return b.use(it, commandRead)
	}
	return "", nil
}

// tidy drops an item it has no use for, when the inventory is full and
// something to pick up is in sight: a weapon or a shield not stronger
// than the one equipped, a bracelet not worn, a pot (the bots do not use
// pots). Not in a shop (putting down is selling), nor on an item or the
// stairs.
func (b *Bot) tidy(g *Game) (string, []string) {
	if len(g.Inventory) < g.Dungeon.Rules.Capacity || g.inShop(g.Player) || g.Player == g.Stairs {
		return "", nil
	}
	if _, ok := g.ItemAt(g.Player); ok {
		return "", nil
	}
	wanted := false
	for _, it := range g.Items {
		if g.IsSeen(it.Pos) && !it.ForSale && it.Def.Category != CategoryMoney && !useless(g, it) {
			wanted = true
			break
		}
	}
	if !wanted {
		return "", nil
	}
	junk := carried(g, func(it *Item) bool { return !it.Equipped && useless(g, it) })
	if junk == nil {
		return "", nil
	}
	return b.use(junk, commandDrop)
}

// useless says whether the bots have no use for it (not equipped): a
// weapon or a shield not stronger than the one equipped, a bracelet when
// one is worn, an empty pot (they do not use pots).
func useless(g *Game, it *Item) bool {
	switch c := it.Def.Category; c {
	case CategoryWeapon, CategoryShield:
		worn := g.equipped(c)
		return worn != nil && worn != it && strength(it) <= strength(worn)
	case CategoryBracelet:
		worn := g.equipped(c)
		return worn != nil && worn != it
	case CategoryPot:
		return len(it.Contents) == 0
	}
	return false
}

// dangerousFloorShare is the share of the monsters of a floor (by the
// weights of its table) that the player would likely lose to, from which
// the floor is dangerous: a careful player goes on at once.
const dangerousFloorShare = 0.25

// dangerousFloor says whether the floor is dangerous, by its monster
// table, as a player who knows the table of the dungeon would judge: the
// share of the monsters (by weight) that would likely beat the player one
// on one, at its HP now.
func (b *Bot) dangerousFloor(g *Game) bool {
	total, bad := 0, 0
	for _, r := range g.Dungeon.MonsterRows {
		def := g.Dungeon.Monster(r.ID)
		if r.First > g.Depth || g.Depth > r.Last || def == nil {
			continue
		}
		total += r.Weight
		if b.dangerous(g, []*Monster{newMonster(def)}) {
			bad += r.Weight
		}
	}
	return total > 0 && float64(bad) >= dangerousFloorShare*float64(total)
}

// threatened says whether an enemy in sight would likely beat the player
// one on one.
func (b *Bot) threatened(g *Game) bool {
	for _, m := range g.visibleEnemies() {
		if b.dangerous(g, []*Monster{m}) {
			return true
		}
	}
	return false
}

// hungry says whether the player is hungry with nothing to eat: time to
// go down and find food, not to rest.
func (b *Bot) hungry(g *Game) bool {
	r := g.Dungeon.Rules
	return g.Fullness*r.HungerTurns < 2*b.floorTurns() &&
		carried(g, func(it *Item) bool { return it.Def.Category == CategoryFood }) == nil
}

// defaultFloorTurns are the turns the bot expects to spend on a floor
// before it has left one.
const defaultFloorTurns = 200

// floorTurns returns the mean turns the bot has spent on the floors it
// left.
func (b *Bot) floorTurns() int {
	if b.floors == 0 {
		return defaultFloorTurns
	}
	return b.floorsTurns / b.floors
}

// track keeps the turns spent on the floors the bot left.
func (b *Bot) track(g *Game) {
	if g.Depth != b.depth {
		if b.depth != 0 {
			b.floors++
			b.floorsTurns += g.Turn - b.arrived
		}
		b.depth, b.arrived = g.Depth, g.Turn
		b.answersIn, b.consumed, b.spent = answersCarried(g), 0, 0
	}
}

// answersCarried counts the answers carried: grasses, scrolls and staffs.
func answersCarried(g *Game) int {
	n := 0
	for _, it := range g.Inventory {
		if c := it.Def.Category; !it.ForSale && (c == CategoryGrass || c == CategoryScroll || c == CategoryStaff) {
			n++
		}
	}
	return n
}

// costlyFloor says whether the floor has cost the bot more answers in
// fights than it has given (the article "原始に続く穴 安定攻略への道",
// platinasun: go down at once when the answers a floor costs outweigh
// what it gives).
func (b *Bot) costlyFloor(g *Game) bool {
	found := answersCarried(g) - b.answersIn + b.consumed
	return b.spent > 0 && b.spent > found
}

// adjacent returns the enemies next to the player that it can attack.
func (b *Bot) adjacent(g *Game) []*Monster {
	var ms []*Monster
	for _, m := range g.visibleEnemies() {
		if chebyshev(m.Pos, g.Player) == 1 && g.clearFrom(g.Player, m.Pos.Sub(g.Player)) {
			ms = append(ms, m)
		}
	}
	return ms
}

// plentyArrows is the count of arrows from which the bot shoots any
// enemy; with fewer, only those worth them.
const plentyArrows = 10

// worthArrows says whether an enemy is worth arrows (the article "原始に
// 続く穴 安定攻略への道", platinasun: do not spend arrows on enemies the
// player can outheal; shoot the costly ones while they are far): fighting
// it from full HP would kill the player at a chance of a tenth of what the
// bot accepts or more.
func (b *Bot) worthArrows(g *Game, m *Monster) bool {
	return fightDeath(g, []*Monster{m}, g.MaxHP, 0) >= b.risk()/10
}

// fight attacks an enemy next to the player, the weakest first.
func (b *Bot) fight(g *Game) (string, []string) {
	ms := b.adjacent(g)
	if len(ms) == 0 {
		return "", nil
	}
	m := slices.MinFunc(ms, func(a, c *Monster) int { return a.HP - c.HP })
	name := directionName(m.Pos.Sub(g.Player))
	if g.Facing != name {
		return "turn", []string{name}
	}
	return "attack", nil
}

// directionName returns the name of the direction d.
func directionName(d Point) string {
	for _, name := range clockwise {
		if Directions[name] == d {
			return name
		}
	}
	return ""
}

// eat eats when hungry.
func (b *Bot) eat(g *Game) (string, []string) {
	if g.Fullness*100 > g.Dungeon.Rules.Fullness*15 {
		return "", nil
	}
	if it := carried(g, func(it *Item) bool { return it.Def.Category == CategoryFood }); it != nil {
		return b.use(it, commandEat)
	}
	return "", nil
}

// equip equips a weapon or a shield stronger than the one equipped (by
// what is known of it), a bracelet when none is worn, arrows when none
// are equipped.
func (b *Bot) equip(g *Game) (string, []string) {
	for _, c := range []Category{CategoryWeapon, CategoryShield, CategoryBracelet, CategoryArrow} {
		worn := g.equipped(c)
		it := carried(g, func(it *Item) bool {
			if it.Def.Category != c || it.Equipped {
				return false
			}
			switch {
			case worn == nil:
				return true
			case c == CategoryWeapon || c == CategoryShield:
				return strength(it) > strength(worn)
			}
			return false
		})
		if it != nil {
			return b.use(it, commandEquip)
		}
	}
	return "", nil
}

// strength is the strength of a weapon or a shield as far as known.
func strength(it *Item) int {
	if it.Known {
		return it.Strength()
	}
	return it.Def.Strength
}

// identify reads a scroll of identification on an unknown item, and with
// no enemy in sight, reads unknown scrolls and drinks unknown grasses at
// full HP, to learn them.
func (b *Bot) identify(g *Game) (string, []string) {
	if len(g.visibleEnemies()) > 0 {
		return "", nil
	}
	unknown := func(c Category) *Item {
		return carried(g, func(it *Item) bool { return it.Def.Category == c && !g.identified[it.Def.ID] })
	}
	if s := carried(g, func(it *Item) bool { return b.known(g, it, EffectIdentify) }); s != nil {
		if carried(g, func(it *Item) bool { return !g.identified[it.Def.ID] }) != nil {
			return b.use(s, commandRead)
		}
	}
	if s := unknown(CategoryScroll); s != nil && g.Player == g.Stairs {
		// Read near the stairs once the floor is done: a scroll may
		// not be undone (the article "原始に続く穴 安定攻略への道").
		return b.use(s, commandRead)
	}
	if s := unknown(CategoryGrass); s != nil && g.HP == g.MaxHP {
		return b.use(s, commandDrink)
	}
	return "", nil
}

// rest waits to heal (the careful bot), with no enemy in sight and the
// wind still far.
func (b *Bot) rest(g *Game) (string, []string) {
	if b.Style != BotCareful || g.HP*10 >= g.MaxHP*7 || !b.canRest(g) {
		return "", nil
	}
	if g.Level.RoomAt(g.Player) < 0 {
		// Rest in a room, not in a corridor, where whatever comes
		// is seen coming (the articles on 浜辺の魔洞, shiren2424).
		if step, n, ok := b.route(g, func(p Point) bool {
			return g.Level.RoomAt(p) >= 0 && !g.entrance(p)
		}); ok && n <= restWalk && g.MonsterAt(step) == nil {
			return "move", []string{directionName(step.Sub(g.Player))}
		}
	}
	return "wait", nil
}

// restWalk is how far the bot walks to rest in a room.
const restWalk = 10

// canRest says whether the player can rest: no enemy in sight, the wind
// far, and food enough (resting costs food).
func (b *Bot) canRest(g *Game) bool {
	return len(g.visibleEnemies()) == 0 && !b.windNear(g) &&
		g.Fullness*100 > g.Dungeon.Rules.Fullness*40
}

// windNear says whether the first wind is near: time to leave.
func (b *Bot) windNear(g *Game) bool {
	return g.FloorTurns >= g.Dungeon.Rules.WindTurns[0]-100
}

// leave goes to the stairs and down: the reckless bot as soon as it knows
// where they are, the careful one when the wind is near, it is hungry with
// nothing to eat, it sees an enemy it would likely lose to, or the floor is
// a dangerous one.
func (b *Bot) leave(g *Game) (string, []string) {
	if !g.IsSeen(g.Stairs) || (b.Style == BotCareful && !b.windNear(g) && !b.hungry(g) &&
		!b.threatened(g) && !b.dangerousFloor(g) && !b.costlyFloor(g) && !b.exploredEnough(g)) {
		return "", nil
	}
	return b.toStairs(g)
}

// toStairs goes to the stairs, and down them.
func (b *Bot) toStairs(g *Game) (string, []string) {
	if g.Player == g.Stairs {
		return "stairs", nil
	}
	return b.walk(g, func(p Point) bool { return p == g.Stairs })
}

// pick walks to the nearest item seen that it can take: not for sale, and
// with room in the inventory (money always).
func (b *Bot) pick(g *Game) (string, []string) {
	return b.approach(g, func(p Point) bool { return wanted(g, p) })
}

// wanted says whether an item the bots would pick up lies on p (seen, not
// under the player).
func wanted(g *Game, p Point) bool {
	full := len(g.Inventory) >= g.Dungeon.Rules.Capacity
	it, ok := g.ItemAt(p)
	return ok && g.IsSeen(p) && p != g.Player && !it.ForSale && !useless(g, it) &&
		(!full || it.Def.Category == CategoryMoney)
}

// explore walks to the nearest tile seen next to one not seen yet.
func (b *Bot) explore(g *Game) (string, []string) {
	return b.approach(g, func(p Point) bool { return frontier(g, p) })
}

// approach walks toward goal as walk does, but waits instead of a step
// next to an enemy coming at the player, so that the enemy steps next to
// it and the player strikes first (the articles on 浜辺の魔洞, shiren2424:
// lead the enemy, take the first move). A fast enemy acts twice whatever
// the player does: no waiting for it.
func (b *Bot) approach(g *Game, goal func(Point) bool) (string, []string) {
	action, args := b.walk(g, goal)
	if action != "move" && action != "dash" || len(args) != 1 {
		return action, args
	}
	p := g.Player.Add(Directions[args[0]])
	for _, m := range g.visibleEnemies() {
		if !m.Dozing && m.Speed != SpeedDouble && chebyshev(m.Pos, g.Player) == 2 && chebyshev(m.Pos, p) == 1 {
			return "wait", nil
		}
	}
	if b.checkFirst(g, p) {
		// A swing at nothing checks the tile for a trap.
		if g.Facing != args[0] {
			return "turn", args
		}
		return "attack", nil
	}
	return action, args
}

// checkFirst says whether the bot checks the tile p for a trap before
// stepping on it: a tile not known safe in a monster house whose monsters
// sleep (a monster house has more traps; a sleeping trap or a mine among
// them can end the run).
func (b *Bot) checkFirst(g *Game, p Point) bool {
	if g.House < 0 || g.Level.RoomAt(p) != g.House || g.IsSafe(p) || g.MonsterAt(p) != nil {
		return false
	}
	for _, m := range g.visibleEnemies() {
		if m.House && m.Dozing {
			return true
		}
	}
	return false
}

// frontier says whether p, seen, is next to a tile not seen yet.
func frontier(g *Game, p Point) bool {
	if p == g.Player {
		return false
	}
	for _, d := range Directions {
		q := p.Add(d)
		if _, ok := g.index(q); ok && !g.IsSeen(q) {
			return true
		}
	}
	return false
}

// exploreReach is how far the nearest tile to explore can be, in steps,
// for a careful bot that knows the stairs to go on exploring, with food
// for a few floors: beyond, the rest of the floor is not worth the walk
// (and the food, and the wind). An item it wants is worth twice the walk.
const exploreReach = 12

// exploredEnough says whether the floor is explored enough to leave: the
// stairs known, and no item it wants nor tile to explore worth the steps
// (the article "シレン5で上達するための思考法": no step without a reason).
func (b *Bot) exploredEnough(g *Game) bool {
	if !g.IsSeen(g.Stairs) {
		return false
	}
	reach := b.reach(g)
	if _, n, ok := b.route(g, func(p Point) bool { return wanted(g, p) }); ok && n <= 2*reach {
		return false
	}
	_, n, ok := b.route(g, func(p Point) bool { return frontier(g, p) })
	return !ok || n > reach
}

// reach returns how far a tile to explore is worth walking to, by the
// food: half as far with food for fewer than three floors, half again as
// far with food for five or more.
func (b *Bot) reach(g *Game) int {
	r := g.Dungeon.Rules
	food := g.Fullness
	for _, it := range g.Inventory {
		if !it.ForSale && it.Def.Category == CategoryFood {
			food += it.Def.Strength
		}
	}
	switch floors := food * r.HungerTurns / max(b.floorTurns(), 1); {
	case floors < 3:
		return exploreReach / 2
	case floors >= 5:
		return exploreReach * 3 / 2
	}
	return exploreReach
}

// walk steps toward the nearest tile that goal accepts, over the tiles
// seen, around the traps found and the monsters; "" when none is
// reachable.
func (b *Bot) walk(g *Game, goal func(Point) bool) (string, []string) {
	p, _, ok := b.route(g, goal)
	if !ok {
		return "", nil
	}
	// Onto the goods of a shop or an item of no use, a dash: it stops on
	// them without picking them up (leaving with goods is stealing).
	if it, ok := g.ItemAt(p); ok && (it.ForSale || useless(g, it)) {
		return "dash", []string{directionName(p.Sub(g.Player))}
	}
	return "move", []string{directionName(p.Sub(g.Player))}
}

// route finds the nearest tile that goal accepts, as walk does, and
// returns the first step toward it and how many steps it is away.
func (b *Bot) route(g *Game, goal func(Point) bool) (Point, int, bool) {
	from := map[Point]Point{g.Player: g.Player}
	queue := []Point{g.Player}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if goal(p) {
			n := 1
			for from[p] != g.Player {
				p = from[p]
				n++
			}
			return p, n, true
		}
		for _, name := range clockwise {
			d := Directions[name]
			q := p.Add(d)
			if _, ok := from[q]; ok || !b.passable(g, q) || !g.clearFrom(p, d) {
				continue
			}
			from[q] = p
			queue = append(queue, q)
		}
	}
	return Point{}, 0, false
}

// passable says whether the bot walks through p: not onto a trap found
// (unless found traps are safe), nor into a calm shopkeeper.
func (b *Bot) passable(g *Game, p Point) bool {
	if !g.IsSeen(p) || !g.Level.At(p).Walkable() {
		return false
	}
	if g.foundTrapAt(p) != nil && !g.Dungeon.Rules.FoundTrapsSafe {
		return false
	}
	// An enemy is no wall: next to it, the bot fights it (a monster asleep
	// in a corridor would cut the floor off). A calm shopkeeper is one
	// (walking into it asks to pay).
	m := g.MonsterAt(p)
	return m == nil || !g.Visible(p) || m.Ally || m.hostile()
}

// BotRun is what a bot did in a run, for cmd/analyze.
type BotRun struct {
	Seed    string
	Style   BotStyle
	Cleared bool
	// Over is how the run ended ("" if the bot gave up: too many
	// actions); Cause its kind: monster, trap, hunger, wind, cleared,
	// unfinished.
	Over, Cause string
	Depth       int
	Turns       int
	Level       int
	Actions     int
	// Floors are the floors reached, in order: the turn and the HP on
	// arriving.
	Floors []BotFloor
	// Unfair says the death was unfair: from full HP in one action, or
	// within unfairTurns of arriving on the floor.
	Unfair bool
	// Picked and Used count the items picked up and used, by category;
	// Kills the monsters killed, Damage the damage taken.
	Picked, Used map[Category]int
	Kills        int
	Damage       int
	// Left counts the items carried at the end, by category.
	Left map[Category]int
	// Risk is the chance of dying the bot reckoned at its last choice,
	// of the fight with the enemies next to the player (0 with none: a
	// death by surprise); FightRisk at the first choice of that fight.
	Risk, FightRisk float64
}

// BotFloor is a floor of a bot's run: on arriving, and the items picked
// up and used on it, by category.
type BotFloor struct {
	Depth, Turn, HP, MaxHP int
	Picked, Used           map[Category]int
}

// newBotFloor returns the floor g is on, as the player arrives.
func newBotFloor(g *Game) BotFloor {
	return BotFloor{g.Depth, g.Turn, g.HP, g.MaxHP, map[Category]int{}, map[Category]int{}}
}

// unfairTurns are the turns after arriving on a floor in which a death
// is unfair.
const unfairTurns = 10

// PlayBot plays a run of d with seed by b, up to maxActions actions.
func PlayBot(d *Dungeon, seed string, b *Bot, maxActions int) (BotRun, error) {
	g := NewGame(d, seed)
	r := BotRun{Seed: seed, Style: b.Style, Picked: map[Category]int{}, Used: map[Category]int{}}
	r.Floors = append(r.Floors, newBotFloor(g))
	for r.Actions < maxActions && g.Over == "" {
		hp, depth := g.HP, g.Depth
		action, args := b.Next(g)
		if err := g.Do(action, args); err != nil {
			return r, fmt.Errorf("%s %s: action %d %s %v: %w", d.ID, seed, r.Actions, action, args, err)
		}
		r.Actions++
		r.count(d, g.takeEvents())
		if g.Depth != depth && g.Over == "" {
			r.Floors = append(r.Floors, newBotFloor(g))
		}
		if g.Over != "" && !g.Cleared && (cause(g) == "monster" || cause(g) == "trap") {
			arrived := r.Floors[len(r.Floors)-1].Turn
			r.Unfair = hp == g.MaxHP || g.Turn-arrived <= unfairTurns
		}
	}
	r.Cleared, r.Over, r.Depth, r.Turns, r.Level = g.Cleared, g.Over, g.Depth, g.Turn, g.PlayerLevel
	r.Cause = cause(g)
	r.Risk, r.FightRisk = b.lastRisk, b.fightRisk
	r.Left = map[Category]int{}
	for _, it := range g.Inventory {
		if !it.ForSale {
			r.Left[it.Def.Category]++
		}
	}
	return r, nil
}

// count counts the events of an action in r.
func (r *BotRun) count(d *Dungeon, events []string) {
	for _, e := range events {
		// "turn event args...": "12 pickup yakusou".
		f := strings.Fields(e)
		if len(f) > 0 {
			f = f[1:]
		}
		switch {
		case len(f) == 2 && (f[0] == "pickup" || f[0] == "use"):
			if it := d.Item(f[1]); it != nil {
				counts := r.Used
				if f[0] == "pickup" {
					counts = r.Picked
				}
				counts[it.Category]++
				if len(r.Floors) > 0 {
					fl := r.Floors[len(r.Floors)-1]
					if f[0] == "pickup" {
						fl.Picked[it.Category]++
					} else {
						fl.Used[it.Category]++
					}
				}
			}
		case len(f) == 2 && f[0] == "kill":
			r.Kills++
		case len(f) >= 3 && f[0] == "hurt":
			// The damage the player took ("hurt 3 nezumi1").
			var n int
			if _, err := fmt.Sscan(f[1], &n); err == nil {
				r.Damage += n
			}
		}
	}
}

// cause is the kind of the end of g.
func cause(g *Game) string {
	switch o := g.Over; {
	case g.Cleared:
		return "cleared"
	case o == "":
		return "unfinished"
	case strings.Contains(o, "クロンの風"):
		return "wind"
	case strings.Contains(o, "飢え死に"):
		return "hunger"
	case strings.HasSuffix(o, "で倒れた"):
		return "trap"
	}
	return "monster"
}
