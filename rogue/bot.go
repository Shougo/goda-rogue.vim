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
	// LookAhead turns the look-ahead on (lookahead.go; off by default:
	// a first step, to measure); rollout says it plays one: it does not
	// look ahead itself.
	LookAhead, rollout bool
	// Trace, if set, gets a line on each of its choices: the rule that
	// gave the action, and the answers to a danger it weighed (analyze
	// -trace).
	Trace func(string)
	rng   *rand.Rand
	// plan is what to answer to the prompts it opened: an item of the
	// inventory, then a command.
	plan []botStep
	// lastHP is the HP at its last choice (afflicted: hit unseen).
	lastHP int
	// rule is the name of the rule that gave the last action (botRules).
	rule string
	// leaving is the floor it turned to the stairs on, and until which
	// turn it keeps going to them (leave).
	leaving struct{ depth, until int }
	// seenRooms are the sections whose room it has seen, worked out once
	// an action (promising).
	seenRooms [gridCols * gridRows]bool
	hasRooms  bool
	// trail are the last tiles it stood on, and shut the tiles it walked
	// back and forth between, with the turn until which it keeps off
	// them (pacing).
	trail []Point
	shut  map[Point]int
	// deciding says whether the bot is deciding on an action (Next): the
	// game does not change meanwhile, so the ways from the player
	// (searched) and the lines of the enemies (enemyLines) are worked out
	// once for all its rules.
	deciding   bool
	searched   [2]botWays
	enemyLines map[Point]bool
	hasLines   bool
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
	// awaited is the enemy the bot waited for last (await), and where
	// it was then.
	awaited   *Monster
	awaitedAt Point

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
	b.deciding, b.hasLines, b.hasRooms = true, false, false
	b.searched[0].done, b.searched[1].done = false, false
	defer func() { b.deciding = false }()
	b.track(g)
	b.pacing(g)
	g.meet()
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
		b.stuck, b.rule = 0, "迷った"
		return "move", []string{clockwise[b.rng.IntN(len(clockwise))]}
	}
	for i, rule := range []func(*Game) (string, []string){
		b.heal, b.afflicted, b.danger, b.shoot, b.fight, b.chase, b.await, b.eat, b.boost, b.equip, b.synthesize, b.identify, b.toShop, b.light,
		b.tidy, b.trade, b.rest, b.leave, b.pick, b.explore,
	} {
		if action, args := rule(g); action != "" {
			if g.Sealed && b.needsMouth() {
				// A sealed mouth eats, drinks and reads nothing: the
				// bots tried again and again, and starved on the
				// floor.
				if it := b.plan[0].item; it != nil && (it.Def.Category == CategoryGrass || it.Def.Category == CategoryScroll) {
					b.consumed-- // (counted by use)
				}
				b.plan = nil
				continue
			}
			b.rule = botRules[i]
			if i == ruleDanger && action == "move" {
				// It steps away from a danger: it does not walk back
				// into it the next turn, as the rules below would,
				// to what lies beyond (and then away again).
				if b.shut == nil {
					b.shut = map[Point]int{}
				}
				b.shut[g.Player] = g.Turn + shutTurns
			}
			return action, args
		}
	}
	if b.seesStairs(g) {
		if action, args := b.toStairs(g); action != "" {
			b.rule = "階段へ向かう"
			return action, args
		}
	}
	b.rule = "することがない"
	return "wait", nil
}

// needsMouth says whether the plan of the bot eats, drinks or reads.
func (b *Bot) needsMouth() bool {
	for _, s := range b.plan {
		switch s.label {
		case commandEat, commandDrink, commandRead:
			return true
		}
	}
	return false
}

// ruleDanger is the place of the rule danger in the rules of Next.
const ruleDanger = 1

// botRules are the names of the rules of Next, in their order: what the
// bot is doing, for a reader (compare.go).
var botRules = []string{
	"回復する", "状態異常の敵や状態に答える", "危険に答える", "矢を撃つ", "戦う", "離れて打つ敵を追う", "敵を待つ", "食べる", "強化する", "装備する",
	"合成する", "識別する", "店で値段を見る", "あかりを使う", "不要品を置く", "入れ替える", "休む", "階段へ向かう", "拾いに行く", "探索する",
}

// answer answers the prompt of g by the plan, or cancels it.
func (b *Bot) answer(g *Game) (string, []string) {
	p := g.Prompt
	if p.kind == promptIdentify && len(p.Choices) > 0 {
		// A kind it does not know first; else whether one is cursed.
		i := max(slices.IndexFunc(p.targets, func(it *Item) bool { return !g.identified[it.Def.ID] }), 0)
		return "choose", []string{fmt.Sprint(i)}
	}
	if p.kind == promptUncurse && len(p.Choices) > 0 {
		// What it knows to be cursed.
		i := max(slices.IndexFunc(p.targets, func(it *Item) bool { return it.curseKnown() }), 0)
		return "choose", []string{fmt.Sprint(i)}
	}
	if p.kind == promptSlot && len(p.Choices) > 0 {
		// The weapon or the shield equipped first.
		i := max(slices.IndexFunc(p.targets, func(it *Item) bool { return it.Equipped }), 0)
		return "choose", []string{fmt.Sprint(i)}
	}
	if len(b.plan) == 0 {
		return "cancel", nil
	}
	step := b.plan[0]
	b.plan = b.plan[1:]
	i := -1
	switch {
	case step.item != nil && (p.kind == promptInventory || p.kind == promptPutIn):
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

// known says whether the bot knows the kind of it, with the effect e:
// identified, or the only kind it may be (mayBe).
func (b *Bot) known(g *Game, it *Item, e Effect) bool {
	if spoiled(it) {
		return false
	}
	kinds := b.mayBe(g, it)
	return len(kinds) == 1 && kinds[0].Effect == e
}

// unsure says whether the bot does not know the kind of it: it may be
// several.
func (b *Bot) unsure(g *Game, it *Item) bool {
	return len(b.mayBe(g, it)) > 1
}

// spoiled says whether it is an item the player knows it cannot use: a
// curse is on it.
func spoiled(it *Item) bool {
	return it.curseKnown() && !it.worn()
}

// heal heals when no enemy is next to the player (danger answers those)
// and its HP is below what the monsters of the floor could take in a few
// turns at worst (healTurns: fewer the stronger the player is on the
// floor), unless the careful bot can rest instead: with the healing that
// fits what is missing best (bestHeal).
func (b *Bot) heal(g *Game) (string, []string) {
	if len(b.adjacent(g)) > 0 || float64(g.HP) > b.healTurns(g)*float64(b.worstTurn(g)) {
		return "", nil
	}
	if b.Style == BotCareful && b.canRest(g) {
		return "", nil
	}
	h, ok := b.bestHeal(g)
	if !ok {
		// None known: an unknown grass may be one (healUnknown).
		if it := b.unknownHeal(g); it != nil {
			return b.use(it, commandDrink)
		}
		return "", nil
	}
	return b.useHeal(h)
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
	if _, ok := m.ability(AbilityRust); ok {
		// A rust bug never attacks.
		return []float64{1}
	}
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
	if a, ok := m.ability(AbilityCritical); ok {
		// A share of the hits do more.
		c := float64(a.Values[0]) / 100
		q := make([]float64, top*a.Values[1]/100+1)
		q[0] = p[0]
		for n := 1; n <= top; n++ {
			q[n] += p[n] * (1 - c)
			q[n*a.Values[1]/100] += p[n] * c
		}
		p = q
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
	// An enemy is there to be killed from its arrival, and attacks from
	// then, or, asleep or confused, once that is over (the player may
	// have killed it by then).
	ms = slices.Clone(ms)
	slices.SortStableFunc(ms, func(a, c *Monster) int {
		if x, y := arrival(g, a), arrival(g, c); x != y {
			return x - y
		}
		return cmpFloat(killTurns(g, a), killTurns(g, c))
	})
	foes := make([]foe, len(ms))
	t := float64(delay)
	for i, m := range ms {
		a := arrival(g, m)
		first := a + 1
		if held := max(m.Asleep, m.Confused); held > 0 {
			first = max(a, delay+held) + 1
		}
		t = max(t, float64(a)) + killTurns(g, m)
		foes[i] = foe{hitDamages(g, m), first, min(int(math.Ceil(t)), riskTurns), m}
	}
	return deathDP(g, hp, foes)
}

// arrival returns the turns m takes to come next to the player (0 next to
// it), by its distance and speed.
func arrival(g *Game, m *Monster) int {
	if reachesNow(g, m) {
		return 0
	}
	steps := chebyshev(m.Pos, g.Player) - 1
	switch m.Speed {
	case SpeedDouble, SpeedMoveDouble:
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
// turns steps away from the enemies ms, its first step to step: an enemy
// of normal speed behind follows without attacking, a fast one attacks
// once a turn, a slow one is left behind; one still next to the player
// after the step (alongside it, or ahead) attacks every turn too, as it
// keeps up: the bots that counted it out walked to the stairs under the
// blows of a crowd, with a scroll that would have put it to sleep.
func fleeDeath(g *Game, ms []*Monster, hp, turns int, step Point) float64 {
	var foes []foe
	for _, m := range ms {
		if a, ok := m.ability(AbilityKnight); ok {
			// It jumps after the player and swoops on it every turn
			// (the bots walked away from an eagle under its blows,
			// back and forth).
			hits := make([]float64, a.Values[0]+1)
			hits[a.Values[0]] = 1
			foes = append(foes, foe{hits, 1, min(turns, riskTurns), m})
			continue
		}
		if !m.Speed.fast() && !m.still() && chebyshev(m.Pos, step) <= 1 && m.Speed != SpeedSlow {
			foes = append(foes, foe{hitDamages(g, m), 1, min(turns, riskTurns), m})
			continue
		}
		if m.Speed.fast() {
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
			for range f.m.Speed.attacks(g.Turn + t) {
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
		d := oneOnOneDeath(g, m) >= b.risk()
		b.risks[key] = d
		return d
	}
	return deathChance(g, ms) >= b.risk()
}

// oneOnOneDeath returns the chance that the player dies fighting m one on
// one, m next to it wherever it is: a monster of the table (with no place)
// or one in sight. (The cache of dangerous keeps no place: a monster
// reckoned far away must not count as harmless next to the player.)
func oneOnOneDeath(g *Game, m *Monster) float64 {
	f := foe{hitDamages(g, m), 1, min(int(math.Ceil(killTurns(g, m))), riskTurns), m}
	return deathDP(g, g.HP, []foe{f})
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
// enough, not always the best one): the stairs under it, a grass drunk
// or thrown, a scroll, a staff (an unknown one, for what it may be: what
// each does to the fight, botitems.go), walking away to the stairs. The cost of an item is its price (the
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
	if g.Player == g.Stairs && g.Confused == 0 {
		return "stairs", nil
	}
	answers := b.answers(g, near, ms)
	if g.Confused > 0 || g.Blind > 0 {
		// Confused, a step goes anywhere; blind, it may meet what it
		// does not see: items only.
		answers = slices.DeleteFunc(answers, func(a botAnswer) bool { return a.item == nil })
	}
	// What it knows first; an item it does not know only when nothing
	// else helps.
	pick := b.pickAnswer(answers, fight, false)
	if pick == nil {
		pick = b.pickAnswer(answers, fight, true)
	}
	if h := b.hopelessHeal(g, answers, pick); h != nil {
		pick = h
	}
	if l, ok := b.lookAhead(g, answers, pick); ok {
		pick = l
	}
	if b.Trace != nil {
		b.traceAnswers(g, fight, answers, pick)
	}
	return b.take(g, pick)
}

// answers are the answers to a danger: the enemies near next to the
// player, ms coming at it.
func (b *Bot) answers(g *Game, near, ms []*Monster) []botAnswer {
	var answers []botAnswer
	seen := map[*ItemDef]bool{}
	for _, it := range g.Inventory {
		if it.ForSale || seen[it.Def] || spoiled(it) {
			continue
		}
		seen[it.Def] = true
		kinds := b.mayBe(g, it)
		if len(kinds) == 0 {
			continue
		}
		cost := b.itemCost(g, it, kinds)
		first := len(answers)
		switch it.Def.Category {
		case CategoryGrass:
			chance := meanOf(kinds, func(def *ItemDef) float64 { return drinkDeath(g, ms, def) })
			answers = append(answers, botAnswer{cost, chance, it, commandDrink, nil, nil, false})
			// A known grass that is better thrown.
			if g.identified[it.Def.ID] {
				if target, chance := b.best(near, func(m *Monster) float64 { return throwDeath(g, ms, m, it.Def) }); target != nil {
					answers = append(answers, botAnswer{cost, chance, it, commandThrow, target, nil, false})
				}
			}
		case CategoryScroll:
			chance := meanOf(kinds, func(def *ItemDef) float64 { return readDeath(g, ms, def) })
			answers = append(answers, botAnswer{cost, chance, it, commandRead, nil, nil, false})
		case CategoryFood:
			// At a hungry ghost: it eats it and is gone.
			if target, chance := b.best(near, func(m *Monster) float64 { return feedDeath(g, ms, m) }); target != nil {
				answers = append(answers, botAnswer{b.worth(g, it, false), chance, it, commandThrow, target, nil, false})
			}
		case CategoryStaff:
			if it.Known && it.Charges == 0 {
				continue
			}
			// At the enemy next to it on which it helps most, for
			// what the staff may be.
			// (Not at a mirror: it sends the bolt back.)
			var at []*Monster
			for _, m := range near {
				if !sendsBack(m, reflectStaffs) {
					at = append(at, m)
				}
			}
			target, chance := b.best(at, func(m *Monster) float64 {
				return meanOf(kinds, func(def *ItemDef) float64 { return waveDeath(g, ms, m, def) })
			})
			if target != nil {
				answers = append(answers, botAnswer{cost, chance, it, commandWave, target, nil, false})
			}
		}
		if len(kinds) > 1 {
			for n := first; n < len(answers); n++ {
				answers[n].unsure = true
			}
		}
	}
	if pot, fuel, ok := b.healingPot(g); ok {
		answers = append(answers, botAnswer{b.healPrice(g) + b.worth(g, fuel, false), fightDeath(g, ms, g.MaxHP, 1), pot, commandPutIn, nil, fuel, false})
	}
	// (Held, it cannot step away.)
	if len(ms) > 1 && g.Stuck == 0 {
		if d, ok := b.stepBack(g, ms); ok {
			answers = append(answers, botAnswer{0, oneByOneDeath(g, ms, g.HP), nil, "move " + d, nil, nil, false})
		}
	}
	if b.seesStairs(g) && g.Stuck == 0 {
		if step, dist, ok := b.route(g, func(p Point) bool { return b.stairs(g, p) }); ok && g.MonsterAt(step) == nil {
			cost := 0.0
			if b.Style == BotCareful {
				cost = leaveCost
			}
			answers = append(answers, botAnswer{cost, fleeDeath(g, ms, g.HP, dist, step), nil, "", nil, nil, false})
		}
	}
	return answers
}

// take is the action that carries out the answer pick (nil for none: the
// rules below go on).
func (b *Bot) take(g *Game, pick *botAnswer) (string, []string) {
	switch {
	case pick == nil:
		return "", nil
	case strings.HasPrefix(pick.command, "move "):
		return "move", []string{strings.TrimPrefix(pick.command, "move ")}
	case pick.item == nil:
		return b.toStairs(g)
	case pick.target != nil:
		d := pick.target.Pos.Sub(g.Player)
		if name := directionName(Point{sign(d.X), sign(d.Y)}); g.Facing != name {
			return "turn", []string{name}
		}
	}
	b.spent++
	if pick.fuel != nil {
		return b.putIn(pick.item, pick.fuel)
	}
	return b.use(pick.item, pick.command)
}

// hopeless is the chance of dying from which the reckoning of the bots
// tells the answers no more apart: against many enemies, every answer
// looks as deadly, over the turns it counts.
const hopeless = 0.95

// hopelessHeal returns, when the answer picked (pick, nil for none)
// leaves the chance of dying hopeless and the HP is below half, the
// known healing that heals most: it buys turns, which the reckoning does
// not see (the bots that walked on under blows died with grasses of
// healing in the pack). Nil otherwise.
func (b *Bot) hopelessHeal(g *Game, answers []botAnswer, pick *botAnswer) *botAnswer {
	if pick != nil && pick.chance < hopeless || g.HP*2 >= g.MaxHP {
		return nil
	}
	var best *botAnswer
	amount := 0
	for i, a := range answers {
		n := 0
		switch {
		case a.fuel != nil:
			n = g.MaxHP
		case a.command == commandDrink && a.item != nil && !a.unsure && a.item.Def.Effect == EffectHeal:
			n = a.item.Def.Strength
		}
		if n > amount {
			best, amount = &answers[i], n
		}
	}
	return best
}

// pickAnswer picks of answers (the unsure ones too with unsure) the
// cheapest that brings the chance of dying under the risk the bot takes,
// else the one that lowers it most under fight; nil if none.
func (b *Bot) pickAnswer(answers []botAnswer, fight float64, unsure bool) *botAnswer {
	var pick *botAnswer
	for i, a := range answers {
		if (unsure || !a.unsure) && a.chance < b.risk() && (pick == nil || a.cost < pick.cost) {
			pick = &answers[i]
		}
	}
	if pick != nil {
		return pick
	}
	for i, a := range answers {
		if (unsure || !a.unsure) && a.chance < fight && (pick == nil || a.chance < pick.chance) {
			pick = &answers[i]
		}
	}
	return pick
}

// minFlight is how far a monster blown away must fly for the bot to count
// it out of the fight: nearer, it is back before the fight is over.
const minFlight = 3

// blowAwayFlight is how many tiles a staff of blowing away sends the
// monster m next to the player flying (Game.blowAway): 0 when it is
// against a wall or another monster.
func blowAwayFlight(g *Game, m *Monster) int {
	d := m.Pos.Sub(g.Player)
	p := m.Pos
	for n := range blowAwayTiles {
		next := p.Add(d)
		if !g.clearFrom(p, d) || !g.Level.At(next).Walkable() || g.occupied(next) {
			return n
		}
		p = next
	}
	return blowAwayTiles
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
	cost := b.meanPrice(g, kinds)
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
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		// With no room left, what it carries keeps the next find out:
		// using it is cheaper.
		cost /= 2
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
	// fuel is what goes in a pot of healing.
	fuel *Item
	// unsure says the bot does not know what the item is: such answers
	// are the last resort.
	unsure bool
}

// mayBe returns the kinds it may be, as the player knows: its kind when
// identified, else the kinds of its category not identified yet (of its
// price, once the player saw it in a shop: the catalog, catalog.go).
func (b *Bot) mayBe(g *Game, it *Item) []*ItemDef {
	if g.identified[it.Def.ID] {
		return []*ItemDef{it.Def}
	}
	priced := g.priced[it.Def.ID]
	var kinds []*ItemDef
	for _, def := range g.Dungeon.Items {
		if def.Category == it.Def.Category && !g.identified[def.ID] && (!priced || def.Price == it.Def.Price) {
			kinds = append(kinds, def)
		}
	}
	return kinds
}

// meanPrice returns the mean price of kinds, as the bot values them: a
// healing grass by how much of the max HP it heals (kindPrice).
func (b *Bot) meanPrice(g *Game, kinds []*ItemDef) float64 {
	t := 0.0
	for _, def := range kinds {
		t += b.kindPrice(g, def)
	}
	return t / float64(max(len(kinds), 1))
}

// shoot shoots arrows at an enemy in a line from the player, not next to
// it, within an arrow's range, nothing in between.
func (b *Bot) shoot(g *Game) (string, []string) {
	arrows := g.equipped(CategoryArrow)
	if arrows == nil {
		return "", nil
	}
	for _, m := range g.visibleEnemies() {
		if arrows.Count < plentyArrows && !b.worthArrows(g, m) || m.room() > 0 || sendsBack(m, reflectArrows) {
			// (One of synthesis swallows the arrow; a mirror sends it
			// back.)
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
		// An arrow that misses flies on: not with a shopkeeper or an
		// ally behind the enemy (a shopkeeper hit calls it a thief).
		behind := false
		for p, i := m.Pos, n; i < throwRange && g.clearFrom(p, step) && g.Level.At(p.Add(step)).Walkable(); i++ {
			p = p.Add(step)
			if o := g.MonsterAt(p); o != nil && !o.hostile() {
				behind = true
				break
			}
		}
		if behind {
			continue
		}
		if name := directionName(step); g.Facing != name {
			return "turn", []string{name}
		}
		return "shoot", nil
	}
	return "", nil
}

// boost drinks the known grasses that only do good (strength, max HP),
// and reads the known scrolls that make the equipped weapon or shield
// stronger, or that take a curse it knows of away.
func (b *Bot) boost(g *Game) (string, []string) {
	if it := carried(g, func(it *Item) bool {
		return it.Def.Category == CategoryGrass && !it.ForSale && (b.known(g, it, EffectStrength) || b.known(g, it, EffectMaxHP))
	}); it != nil {
		return b.use(it, commandDrink)
	}
	if it := carried(g, func(it *Item) bool {
		return !it.ForSale && b.known(g, it, EffectUncurse) &&
			slices.ContainsFunc(g.Inventory, func(x *Item) bool { return x.curseKnown() })
	}); it != nil {
		return b.use(it, commandRead)
	}
	if it := carried(g, func(it *Item) bool {
		return !it.ForSale && (b.known(g, it, EffectUpWeapon) && g.equipped(CategoryWeapon) != nil ||
			b.known(g, it, EffectUpShield) && g.equipped(CategoryShield) != nil)
	}); it != nil {
		return b.use(it, commandRead)
	}
	return "", nil
}

// light reads a known scroll of light when the stairs are not found and
// the floor is explored as far as it can.
func (b *Bot) light(g *Game) (string, []string) {
	if b.seesStairs(g) || g.Lit {
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

// The trades of a full inventory. A player with no room does not walk
// past an onigiri with four grasses of confusion in its bag: it eats or
// drinks on the spot what can be used at once, or swaps what it values
// least for what lies there.

// tradeMargin is how much more than the least it carries an item on the
// floor must be worth to the bot to be swapped for it.
const tradeMargin = 1.25

// worth returns what it is worth to the bot: what giving it up costs.
// Food is worth several times its price while the bot holds little of it
// (foodWorth: hunger ends runs), a rotten onigiri little; equipment in use, or better than that in use, is never
// given up; what it has no use for is worth nothing; the rest, what using
// it costs (itemCost: the less the more of the kind it carries). extra
// counts it as one more of its kind (an item on the floor).
func (b *Bot) worth(g *Game, it *Item, extra bool) float64 {
	if it.Equipped {
		return math.Inf(1)
	}
	if useless(g, it) {
		return 0
	}
	switch it.Def.Category {
	case CategoryWeapon, CategoryShield, CategoryBracelet:
		return math.Inf(1)
	case CategoryFood:
		if it.Def.Effect == EffectRotten {
			return float64(it.Def.Price) / 2
		}
		return float64(it.Def.Price) * foodWorth(g, it)
	case CategoryArrow:
		return float64(it.Def.Price * it.Count)
	case CategoryPot:
		if b.known(g, it, EffectHealPot) {
			charges := unknownCharges
			if it.Known {
				charges = it.Charges
			}
			return b.healPrice(g) * float64(charges)
		}
	}
	kinds := b.mayBe(g, it)
	w := b.itemCost(g, it, kinds)
	if extra && it.Def.Category != CategoryStaff {
		// itemCost counted those carried only.
		n := 0
		for _, o := range g.Inventory {
			if !o.ForSale && o.Def == it.Def {
				n++
			}
		}
		w *= float64(n+1) / float64(n+2)
	}
	return w
}

// The fullness the bots want to hold, in the belly and in the bag: below
// foodLow, food is worth a lot; above foodHigh, little.
const (
	foodLow   = 200
	foodHigh  = 300
	foodSpare = 0.02
)

// foodWorth returns how many times its price the food it is worth to the
// bot, by the fullness it holds without it: its own, and that of the
// food it carries, in pots too. Below foodLow it takes food along,
// whatever it gives up for it (8 times the price with none, 5 at
// foodLow); from there to foodHigh food is worth less and less (half its
// price at foodHigh); beyond, next to nothing (foodSpare): only what it
// has no use for goes for it.
func foodWorth(g *Game, it *Item) float64 {
	total := g.Fullness
	var count func(its []*Item)
	count = func(its []*Item) {
		for _, o := range its {
			if o != it && !o.ForSale && o.Def.Category == CategoryFood && o.Def.Effect != EffectRotten {
				total += o.Def.Strength
			}
			count(o.Contents)
		}
	}
	count(g.Inventory)
	switch t := float64(total); {
	case t < foodLow:
		return 8 - 3*t/foodLow
	case t < foodHigh:
		return 5 - 4.5*(t-foodLow)/(foodHigh-foodLow)
	default:
		return foodSpare
	}
}

// least returns the item carried that the bot values least, to give up
// for the item foot on the floor, if foot is worth more by tradeMargin;
// nil if none.
func (b *Bot) least(g *Game, foot *Item) *Item {
	if foot.ForSale || foot.Def.Category == CategoryMoney || useless(g, foot) {
		return nil
	}
	var low *Item
	lw := math.Inf(1)
	for _, it := range g.Inventory {
		if it.ForSale || it.Equipped {
			continue
		}
		if w := b.worth(g, it, false); w < lw {
			low, lw = it, w
		}
	}
	if low == nil || lw*tradeMargin >= b.worth(g, foot, true) {
		return nil
	}
	return low
}

// wants says whether the bot walks to the item on p: one it would pick
// up (wanted), or, with a full inventory, one worth a trade.
func (b *Bot) wants(g *Game, p Point) bool {
	if wanted(g, p) {
		return true
	}
	it, ok := g.ItemAt(p)
	return ok && g.IsSeen(p) && p != g.Player && len(g.Inventory) >= g.Dungeon.Rules.Capacity &&
		!g.inShop(p) && b.least(g, it) != nil
}

// fits says whether eating the food it now wastes none of it.
func fits(g *Game, it *Item) bool {
	return it.Def.Effect != EffectRotten && !spoiled(it) && g.Fullness+it.Def.Strength <= g.fullnessCap()
}

// trade is the bot standing on an item with a full inventory: it eats
// the food there if none of it is wasted, drinks an unknown grass there
// at full HP (as it does one it carries), eats the food it would give up,
// or swaps what it values least for the item. Not in a shop. (An enemy
// next to it is fought first, by the rules before this one; one farther
// does not keep it from a swap: walking off and back would cost more.)
func (b *Bot) trade(g *Game) (string, []string) {
	foot := g.foot()
	if foot == nil || foot.ForSale || len(g.Inventory) < g.Dungeon.Rules.Capacity || !g.canSwap() {
		return "", nil
	}
	atFoot := func(command string) (string, []string) {
		b.plan = []botStep{{label: command}}
		return "foot", nil
	}
	switch {
	case foot.Def.Category == CategoryFood && fits(g, foot):
		return atFoot(commandEat)
	case foot.Def.Category == CategoryGrass && !g.identified[foot.Def.ID] && !spoiled(foot) && g.HP == g.MaxHP &&
		len(g.visibleEnemies()) == 0:
		return atFoot(commandDrink)
	}
	low := b.least(g, foot)
	switch {
	case low == nil:
		return "", nil
	case low.Def.Category == CategoryFood && fits(g, low):
		return b.use(low, commandEat)
	}
	// (Not use: nothing is used up.)
	b.plan = []botStep{{item: low}, {label: commandSwap}}
	return "inventory", nil
}

// useless says whether the bots have no use for it (not equipped): a
// weapon or a shield not stronger than the one equipped, a bracelet when
// one is worn, an empty pot (they do not use pots), a staff with no
// charge left.
func useless(g *Game, it *Item) bool {
	if spoiled(it) {
		return true
	}
	switch c := it.Def.Category; c {
	case CategoryWeapon, CategoryShield:
		worn := g.equipped(c)
		return worn != nil && worn != it && gearWorth(g, it) <= gearWorth(g, worn)
	case CategoryBracelet:
		// One bracelet of regeneration (known) to heal, and one other
		// to wear the rest of the time.
		// The one kept of two of the same use: the one worn, else the
		// first carried.
		regen := func(o *Item) bool { return o.Known && o.Def.Effect == EffectRegen }
		if it.Equipped {
			return false
		}
		for _, o := range g.Inventory {
			if o == it {
				// Nothing kept before it: only one worn would be.
				return slices.ContainsFunc(g.Inventory, func(o *Item) bool {
					return o != it && o.Equipped && o.Def.Category == c && regen(o) == regen(it)
				})
			}
			if !o.ForSale && o.Def.Category == c && regen(o) == regen(it) && regen(it) {
				return true
			}
		}
		// On the floor: one of the same use is carried already.
		return slices.ContainsFunc(g.Inventory, func(o *Item) bool {
			return !o.ForSale && o.Def.Category == c && regen(o) == regen(it)
		})
	case CategoryPot:
		if it.Def.Effect == EffectHealPot {
			// Nothing stays in it: it is used up when it holds no more.
			return it.Known && it.Charges <= 0
		}
		return len(it.Contents) == 0
	case CategoryStaff:
		// Waved empty.
		return it.Known && it.Charges <= 0
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
	return b.floorDanger(g) >= dangerousFloorShare
}

// floorDanger is the share of the monsters of the floor (by the weights
// of its table) that would likely beat the player one on one, at its HP
// now.
func (b *Bot) floorDanger(g *Game) float64 {
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
	return float64(bad) / float64(max(total, 1))
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
	return g.Fullness*r.HungerTurns/hungerRate(g) < 2*b.floorTurns() &&
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
		b.trail, b.shut = nil, nil
	}
}

// shutTurns is for how many turns the bot keeps off the tiles it walked
// back and forth between, or stepped away from a danger on.
const shutTurns = 50

// pacing notes where the bot stands, and when it has walked back and
// forth between two tiles (seven times on them in a row: two of its
// rules undo each other there, one stepping away from a danger, say, and
// the other back to what lies beyond), shuts both for a while: its ways
// go around them, to something else.
func (b *Bot) pacing(g *Game) {
	if n := len(b.trail); n == 0 || b.trail[n-1] != g.Player {
		b.trail = append(b.trail, g.Player)
	}
	n := len(b.trail)
	if n < 7 {
		return
	}
	b.trail = b.trail[n-7:]
	t := b.trail
	if t[0] != t[1] && t[0] == t[2] && t[0] == t[4] && t[0] == t[6] && t[1] == t[3] && t[1] == t[5] {
		if b.shut == nil {
			b.shut = map[Point]int{}
		}
		b.shut[t[0]], b.shut[t[1]] = g.Turn+shutTurns, g.Turn+shutTurns
		b.trail = nil
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
	// An invisible one next to it too: its blows tell it is there (a
	// player swings at where they come from).
	for _, m := range g.Monsters {
		if !m.hostile() || !g.Visible(m.Pos) {
			continue
		}
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

// await waits for an enemy coming, two tiles away, so as to strike
// first when it steps next to the player, as players do (Shougo's replay
// of 2026-11-24: he swung at the air while one came). Not for one that
// does not come (still, fleeing, an archer or a mage: chase), nor
// twice for the same one when it did not come.
func (b *Bot) await(g *Game) (string, []string) {
	for _, m := range g.visibleEnemies() {
		if m.Dozing || chebyshev(m.Pos, g.Player) != 2 || !g.sees(m.Pos, g.Player) || m.still() {
			continue
		}
		_, flees := m.ability(AbilityFlee)
		if strikesFromAfar(m) || flees || m.Fled {
			continue
		}
		if b.awaited == m && b.awaitedAt == m.Pos {
			// It did not come: go on.
			continue
		}
		b.awaited, b.awaitedAt = m, m.Pos
		return "wait", nil
	}
	return "", nil
}

// strikesFromAfar says whether m strikes the player from where it stands
// rather than coming next to it: an archer, a mage, one that swoops from
// a knight's move away (the eagles: the bots waited for one that hit them
// to death, never coming), one that strikes two tiles away.
func strikesFromAfar(m *Monster) bool {
	for _, a := range []AbilityName{AbilityShoot, AbilityMagic, AbilityKnight, AbilityLong} {
		if _, ok := m.ability(a); ok {
			return true
		}
	}
	return false
}

// chase goes for an enemy in sight, awake, that strikes from afar
// (strikesFromAfar): it does not come. The way keeps off its lines, if it
// can.
func (b *Bot) chase(g *Game) (string, []string) {
	var archer *Monster
	for _, m := range g.visibleEnemies() {
		if strikesFromAfar(m) && !m.Dozing &&
			(archer == nil || chebyshev(m.Pos, g.Player) < chebyshev(archer.Pos, g.Player)) {
			archer = m
		}
	}
	if archer == nil || chebyshev(archer.Pos, g.Player) <= 1 {
		return "", nil
	}
	return b.walk(g, func(p Point) bool { return chebyshev(p, archer.Pos) == 1 })
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
	var foods []*Item
	for _, it := range g.Inventory {
		if it.Def.Category == CategoryFood && !it.ForSale && !spoiled(it) {
			foods = append(foods, it)
		}
	}
	if foods = keepOnigiri(foods); len(foods) > 0 {
		return b.use(foods[0], commandEat)
	}
	return "", nil
}

// equip equips a weapon or a shield stronger than the one equipped (by
// what is known of it), a bracelet when none is worn, arrows when none
// are equipped.
func (b *Bot) equip(g *Game) (string, []string) {
	for _, c := range []Category{CategoryWeapon, CategoryShield, CategoryBracelet, CategoryArrow} {
		worn := g.equipped(c)
		if worn != nil && worn.Cursed {
			// It does not come off.
			continue
		}
		if c == CategoryBracelet {
			if action, args := b.bracelet(g, worn); action != "" {
				return action, args
			}
			continue
		}
		it := carried(g, func(it *Item) bool {
			if it.Def.Category != c || it.Equipped {
				return false
			}
			switch {
			case worn == nil:
				return true
			case c == CategoryWeapon || c == CategoryShield:
				return gearWorth(g, it) > gearWorth(g, worn)
			}
			return false
		})
		if it != nil {
			return b.use(it, commandEquip)
		}
	}
	return "", nil
}

// regenOff and regenOn are the floors of food (foodFloors) below which
// the bot takes a bracelet of regeneration off, and from which it puts
// one on: it makes the player twice as hungry, and the bots that wore one
// all the time starved (a third of their deaths by hunger in the cave).
// The gap keeps it from taking it off and putting it on again and again.
const (
	regenOff = 2
	regenOn  = 3
)

// regenFrom is the share of the max HP (in tenths) below which the bot
// puts a bracelet of regeneration on.
const regenFrom = 9

// bracelet wears a bracelet of regeneration only to heal, as a player
// puts one on to rest (Shougo): with the HP below regenFrom tenths and
// food enough, and takes it off at full HP, for another bracelet if it
// carries one. Otherwise it wears a bracelet when none is worn (to learn
// it, if it does not know it). Not with an enemy in sight: a swap costs
// a turn.
func (b *Bot) bracelet(g *Game, worn *Item) (string, []string) {
	if (worn != nil && worn.Cursed) || len(g.visibleEnemies()) > 0 {
		return "", nil
	}
	regen := func(it *Item) bool { return b.known(g, it, EffectRegen) }
	// A bracelet of hunger (twice as hungry, for stronger blows) comes
	// off when the food runs short, and goes on again with food for a
	// few floors, as one of regeneration does.
	hungry := func(it *Item) bool { return b.known(g, it, EffectHungry) }
	short := b.foodFloors(g) < regenOn
	other := carried(g, func(it *Item) bool {
		return it.Def.Category == CategoryBracelet && !it.Equipped && !regen(it) && !(short && hungry(it))
	})
	if worn != nil && hungry(worn) && b.foodFloors(g) < regenOff {
		if other != nil {
			return b.use(other, commandEquip)
		}
		return b.use(worn, commandUnequip)
	}
	if worn != nil && regen(worn) {
		if g.HP < g.MaxHP && b.foodFloors(g) >= regenOff {
			return "", nil
		}
		if other != nil {
			return b.use(other, commandEquip)
		}
		return b.use(worn, commandUnequip)
	}
	if g.HP*10 < g.MaxHP*regenFrom && b.foodFloors(g) >= regenOn {
		if it := carried(g, func(it *Item) bool { return it.Def.Category == CategoryBracelet && regen(it) }); it != nil {
			return b.use(it, commandEquip)
		}
	}
	if worn == nil && other != nil {
		return b.use(other, commandEquip)
	}
	return "", nil
}

// foodFloors is for how many floors the food lasts, the fullness and that
// of the food carried (in pots too), at twice the hunger, as with a
// bracelet of regeneration.
func (b *Bot) foodFloors(g *Game) int {
	return foodTotal(g) * g.Dungeon.Rules.HungerTurns / 2 / max(b.floorTurns(), 1)
}

// foodTotal is the fullness and that of the food carried, in pots too (not
// rotten).
func foodTotal(g *Game) int {
	total := g.Fullness
	var count func(its []*Item)
	count = func(its []*Item) {
		for _, o := range its {
			if !o.ForSale && o.Def.Category == CategoryFood && o.Def.Effect != EffectRotten {
				total += o.Def.Strength
			}
			count(o.Contents)
		}
	}
	count(g.Inventory)
	return total
}

// hungerRate is how many times as fast as without a bracelet the player
// gets hungry.
func hungerRate(g *Game) int {
	if g.wears(EffectRegen) || g.wears(EffectHungry) {
		return 2
	}
	return 1
}

// gearWorth is what a weapon or a shield is worth to the bots: its
// strength as far as known, and its ability born with and its seals, as
// so much strength each (abilityWorth).
func gearWorth(g *Game, it *Item) float64 {
	w := float64(strength(it))
	if s := g.Dungeon.SealOn(it.Def.Seal, it.Def.Category); s != nil {
		w += abilityWorth(s.Effect, s.Strength)
	}
	for _, glyph := range it.Seals {
		if s := g.Dungeon.SealOn(glyph, it.Def.Category); s != nil {
			w += abilityWorth(s.Effect, s.AsSeal)
		}
	}
	return w
}

// abilityWorth is how much strength an ability of the effect e and the
// strength n is worth to the bots, roughly: 25% of critical blows as 3,
// every miss turned into a hit as 4, a guard against rust, hunger or
// theft as 2.
func abilityWorth(e SealEffect, n int) float64 {
	f := float64(n)
	switch e {
	case SealAttack, SealDefense:
		return f
	case SealCrit:
		return f / 8
	case SealSure:
		return f / 25
	case SealThree:
		return f / 20
	case SealCounter:
		return f / 10
	case SealDodge:
		return f / 5
	case SealDodgeThrown:
		return f / 20
	case SealCritGuard:
		return f / 25
	}
	return f / 50
}

// strength is the strength of a weapon or a shield as far as known.
func strength(it *Item) int {
	if it.Known {
		return it.Strength()
	}
	return it.Def.Strength
}

// toShop walks into a shop it has seen, with no enemy in sight, to learn
// the prices of the kinds it carries and does not know (meet).
func (b *Bot) toShop(g *Game) (string, []string) {
	if g.Shop == nil || !g.keepersCalm() || len(b.threats(g)) > 0 {
		return "", nil
	}
	if carried(g, func(it *Item) bool { return b.unsure(g, it) && !g.priced[it.Def.ID] && sellPrice(it) > 0 }) == nil {
		return "", nil
	}
	return b.walk(g, func(p Point) bool { return g.inShop(p) && g.IsSeen(p) })
}

// earlyFloors: the first floors are the first 1/earlyFloors of the
// dungeon (identify).
const earlyFloors = 3

// identify reads a scroll of identification on an unknown item, and with
// no enemy in sight, reads unknown scrolls (on the stairs, or anywhere on
// the first floors) and drinks unknown grasses (at full HP, or at 70% on
// the first floors), to learn them.
func (b *Bot) identify(g *Game) (string, []string) {
	if len(g.visibleEnemies()) > 0 {
		return "", nil
	}
	unknown := func(c Category) *Item {
		return carried(g, func(it *Item) bool { return it.Def.Category == c && b.unsure(g, it) && !spoiled(it) })
	}
	if s := carried(g, func(it *Item) bool { return b.known(g, it, EffectIdentify) }); s != nil {
		if carried(g, func(it *Item) bool { return b.unsure(g, it) }) != nil {
			return b.use(s, commandRead)
		}
	}
	// On the first floors, where a bad surprise costs little, it tries
	// what it does not know as soon as it is safe: a known healing grass
	// is worth more than an unknown one when it matters.
	early := g.Depth*earlyFloors <= g.Dungeon.Floors
	if s := unknown(CategoryScroll); s != nil && (g.Player == g.Stairs || early) {
		// Else near the stairs once the floor is done: a scroll may
		// not be undone (the article "原始に続く穴 安定攻略への道").
		return b.use(s, commandRead)
	}
	if s := unknown(CategoryGrass); s != nil && (g.HP == g.MaxHP || early && g.HP*10 >= g.MaxHP*7) {
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
// nothing to eat, it sees an enemy it would likely lose to, the floor is
// a dangerous one, or it is the last one (nothing found there is of any
// use: the stairs clear the dungeon).
func (b *Bot) leave(g *Game) (string, []string) {
	last := g.Depth >= g.Dungeon.Floors
	if !b.seesStairs(g) {
		return "", nil
	}
	// Once on its way, it keeps on for a few turns: a reason that holds
	// on one tile and not on the next (an enemy in sight from there)
	// would send it back and forth.
	going := b.leaving.depth == g.Depth && g.Turn < b.leaving.until
	if b.Style == BotCareful && !going && !last && !b.windNear(g) && !b.hungry(g) &&
		!b.threatened(g) && !b.dangerousFloor(g) && !b.costlyFloor(g) && !b.exploredEnough(g) {
		return "", nil
	}
	if !going {
		b.leaving.depth, b.leaving.until = g.Depth, g.Turn+leaveTurns
	}
	return b.toStairs(g)
}

// leaveTurns is for how many turns a bot that turned to the stairs keeps
// going to them.
const leaveTurns = 10

// toStairs goes to the stairs, and down them.
func (b *Bot) toStairs(g *Game) (string, []string) {
	// (Also on what looks like stairs: a mimic the player came to stand
	// on without walking there, which walking has no step to.)
	if g.stairsAt(g.Player) {
		return "stairs", nil
	}
	return b.walk(g, func(p Point) bool { return b.stairs(g, p) })
}

// stairs says whether p looks like stairs to the bot: the stairs or a
// mimic that looks like them, seen (it cannot tell them apart).
func (b *Bot) stairs(g *Game, p Point) bool {
	return g.stairsAt(p) && g.IsSeen(p)
}

// seesStairs says whether the bot has seen stairs, or what looks like
// them.
func (b *Bot) seesStairs(g *Game) bool {
	if g.IsSeen(g.Stairs) {
		return true
	}
	return slices.ContainsFunc(g.Decoys, func(d Decoy) bool { return g.IsSeen(d.Pos) })
}

// pick walks to the nearest item seen that it can take: not for sale, and
// with room in the inventory (money always). The careful bot that knows
// the stairs leaves one too far to be worth the walk, as exploredEnough
// counts it: else it would walk toward the item, find the floor explored
// enough a step farther from the rest of it, turn to the stairs, and so
// on, back and forth.
func (b *Bot) pick(g *Game) (string, []string) {
	want := func(p Point) bool { return b.wants(g, p) }
	if b.Style == BotCareful && b.seesStairs(g) {
		if _, n, ok := b.route(g, want); !ok || n > 2*b.reach(g) {
			return "", nil
		}
	}
	return b.approach(g, want)
}

// wanted says whether an item the bots would pick up lies on p (seen, not
// under the player).
func wanted(g *Game, p Point) bool {
	full := len(g.Inventory) >= g.Dungeon.Rules.Capacity
	it, ok := g.ItemAt(p)
	return ok && g.IsSeen(p) && p != g.Player && !it.ForSale && !useless(g, it) &&
		(!full || it.Def.Category == CategoryMoney)
}

// explore walks to the nearest tile seen next to one not seen yet, that
// may lead somewhere (promising); with no such tile and no way to the
// stairs yet, to any.
func (b *Bot) explore(g *Game) (string, []string) {
	if action, args := b.approach(g, func(p Point) bool { return b.unexplored(g, p) }); action != "" {
		return action, args
	}
	// (Stairs seen from afar, by a scroll of light say, may have no way
	// to them yet.)
	if _, _, ok := b.route(g, func(p Point) bool { return b.stairs(g, p) }); ok {
		return "", nil
	}
	return b.approach(g, func(p Point) bool { return frontier(g, p) })
}

// unexplored says whether p is a tile to explore from: next to one not
// seen yet, and promising.
func (b *Bot) unexplored(g *Game, p Point) bool {
	return frontier(g, p) && b.promising(g, p)
}

// promising says whether exploring on from p may find a room: a floor is
// three by three sections with a room at most each, and a corridor runs
// between two sections next to each other, so a corridor in a section
// whose room is seen, with the rooms of the sections around it seen too,
// leads to nothing new (a player knows the floor has no more rooms
// there). Any tile is promising in a room, in a maze, in a big room.
func (b *Bot) promising(g *Game, p Point) bool {
	l := g.Level
	if l.Maze || len(l.Rooms) <= 1 || l.At(p) != TileCorridor {
		return true
	}
	if !b.deciding || !b.hasRooms {
		b.seenRooms = [gridCols * gridRows]bool{}
		for _, r := range l.Rooms {
			if g.IsSeen(Point{r.X, r.Y}) || g.IsSeen(Point{r.X + r.W - 1, r.Y + r.H - 1}) || g.IsSeen(Point{r.X + r.W/2, r.Y + r.H/2}) {
				b.seenRooms[section(Point{r.X, r.Y})] = true
			}
		}
		b.hasRooms = true
	}
	s := section(p)
	for _, d := range []Point{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		x, y := s%gridCols+d.X, s/gridCols+d.Y
		if x >= 0 && y >= 0 && x < gridCols && y < gridRows && !b.seenRooms[y*gridCols+x] {
			return true
		}
	}
	return false
}

// section returns the section of the floor p is in.
func section(p Point) int {
	return min(p.Y/secHeight, gridRows-1)*gridCols + min(p.X/secWidth, gridCols-1)
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
		// (Not for one that never moves: it would wait for ever.)
		// Nor for one that strikes from afar: it does not come.
		if !m.Dozing && !m.Speed.fast() && !m.still() && !strikesFromAfar(m) && chebyshev(m.Pos, g.Player) == 2 && chebyshev(m.Pos, p) == 1 {
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
	if !b.seesStairs(g) {
		return false
	}
	reach := b.reach(g)
	if _, n, ok := b.route(g, func(p Point) bool { return b.wants(g, p) }); ok && n <= 2*reach {
		return false
	}
	_, n, ok := b.route(g, func(p Point) bool { return b.unexplored(g, p) })
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
	switch floors := food * r.HungerTurns / hungerRate(g) / max(b.floorTurns(), 1); {
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
	// Around the lines of an enemy that charges, pulls or shoots, if
	// there is a way: on one, it reaches the player from afar.
	lines := b.lines(g)
	if len(lines) > 0 {
		if p, n, ok := b.routeAvoiding(g, goal, lines); ok {
			return p, n, true
		}
	}
	return b.routeAvoiding(g, goal, nil)
}

// lines is reachLines, worked out once an action.
func (b *Bot) lines(g *Game) map[Point]bool {
	if !b.deciding {
		return reachLines(g)
	}
	if !b.hasLines {
		b.enemyLines, b.hasLines = reachLines(g), true
	}
	return b.enemyLines
}

// reachLines returns the tiles from where the enemies in sight, awake,
// would charge, pull or shoot the player.
func reachLines(g *Game) map[Point]bool {
	var lines map[Point]bool
	for _, m := range g.visibleEnemies() {
		if m.Dozing {
			continue
		}
		reach := 0
		for _, n := range lineAbilities {
			if a, ok := m.ability(n); ok {
				reach = max(reach, a.Values[0])
			}
		}
		if a, ok := m.ability(AbilityAfflict); ok && a.Values[1] == 0 {
			// Next to one that brings a state: kept off (afflict.go).
			for _, d := range around {
				if lines == nil {
					lines = map[Point]bool{}
				}
				lines[m.Pos.Add(d)] = true
			}
		}
		for _, d := range around {
			p := m.Pos
			for range reach {
				p = p.Add(d)
				if _, _, ok := g.lineTo(m.Pos, p, reach); ok {
					if lines == nil {
						lines = map[Point]bool{}
					}
					lines[p] = true
				} else if chebyshev(m.Pos, p) > 1 {
					break
				}
			}
		}
	}
	return lines
}

// botWays are the ways from the player to every tile the bot can walk
// to: the tiles in the order a search from the player reaches them
// (nearest first), and the tile each is reached from, by index (-1: not
// reached).
type botWays struct {
	order, from []int
	start       int
	done        bool
}

// ways returns the ways from the player, not through the tiles avoid.
// The search does not depend on where the bot wants to go, so while it
// decides on an action (Next) there is one search for all its rules,
// and another around the lines of the enemies (which: 1 with avoid).
func (b *Bot) ways(g *Game, avoid map[Point]bool) *botWays {
	w := &botWays{}
	if b.deciding {
		w = &b.searched[min(len(avoid), 1)]
		if w.done {
			return w
		}
	}
	l := g.Level
	if cap(w.from) < len(l.Tiles) {
		w.from = make([]int, len(l.Tiles))
	}
	w.from = w.from[:len(l.Tiles)]
	for i := range w.from {
		w.from[i] = -1
	}
	w.order, w.done = w.order[:0], true
	start, ok := g.index(g.Player)
	if !ok {
		return w
	}
	w.start = start
	w.from[start] = start
	w.order = append(w.order, start)
	for n := 0; n < len(w.order); n++ {
		pi := w.order[n]
		p := Point{pi % l.W, pi / l.W}
		for _, d := range straightFirst {
			q := p.Add(d)
			qi, ok := g.index(q)
			if !ok || w.from[qi] >= 0 || (len(avoid) > 0 && avoid[q]) || !b.passable(g, q) || !g.clearFrom(p, d) {
				continue
			}
			w.from[qi] = pi
			w.order = append(w.order, qi)
		}
	}
	return w
}

// straightFirst are the directions of the search of the ways: the four
// straight ones before the diagonals, so that of the ways of a same
// length the search finds a straight one (four steps to the right, not
// up-right, right, right, down-right, which is as short and looks lost).
var straightFirst = []Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1}}

// routeAvoiding is route, not through the tiles avoid.
func (b *Bot) routeAvoiding(g *Game, goal func(Point) bool, avoid map[Point]bool) (Point, int, bool) {
	l := g.Level
	w := b.ways(g, avoid)
	for _, i := range w.order {
		if !goal(Point{i % l.W, i / l.W}) {
			continue
		}
		n := 1
		for w.from[i] != w.start {
			i = w.from[i]
			n++
		}
		return Point{i % l.W, i / l.W}, n, true
	}
	return Point{}, 0, false
}

// passable says whether the bot walks through p: not onto a trap found
// (unless found traps are safe), nor into a calm shopkeeper.
func (b *Bot) passable(g *Game, p Point) bool {
	if !g.IsSeen(p) || !g.Level.At(p).Walkable() {
		return false
	}
	if g.foundTrapAt(p) != nil && !g.Dungeon.Rules.FoundTrapsSafe && !g.wears(EffectTrapper) {
		return false
	}
	if until, ok := b.shut[p]; ok && g.Turn < until {
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
	// Heals counts the heals used: healing grasses drunk, and items put
	// in a pot of healing; HealsLeft those left at the end (the grasses,
	// and the room left in pots of healing).
	Heals, HealsLeft int
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
		hp, depth, pots := g.HP, g.Depth, potHeals(g)
		action, args := b.Next(g)
		if err := g.Do(action, args); err != nil {
			return r, fmt.Errorf("%s %s: action %d %s %v: %w", d.ID, seed, r.Actions, action, args, err)
		}
		r.Actions++
		r.count(d, g.takeEvents())
		if potHeals(g) < pots {
			r.Heals++
		}
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
			if it.Def.Effect == EffectHeal {
				r.HealsLeft++
			}
		}
	}
	r.HealsLeft += potHeals(g)
	return r, nil
}

// potHeals is the room left in the pots of healing carried.
func potHeals(g *Game) int {
	n := 0
	for _, it := range g.Inventory {
		if !it.ForSale && it.Def.Effect == EffectHealPot {
			n += max(it.Charges-len(it.Contents), 0)
		}
	}
	return n
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
				if f[0] == "use" && it.Effect == EffectHeal {
					r.Heals++
				}
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

// reachesNow says whether m can reach the player from where it is on its
// next action: a charge, a pull or a shot along its line.
func reachesNow(g *Game, m *Monster) bool {
	for _, n := range lineAbilities {
		if a, ok := m.ability(n); ok && a.Values[0] > 0 {
			if _, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0]); ok {
				return true
			}
		}
	}
	return false
}
