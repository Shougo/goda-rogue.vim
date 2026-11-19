package rogue

import (
	"fmt"
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
}

// botStep is an answer to a prompt: the item it (in the inventory), or
// the choice labeled label.
type botStep struct {
	item  *Item
	label string
}

// NewBot returns a bot of style, with its randomness from seed.
func NewBot(style BotStyle, seed uint64) *Bot {
	return &Bot{Style: style, rng: rand.New(rand.NewPCG(seed, 0x60da))}
}

// Next returns the next action of the bot in g, and its arguments.
func (b *Bot) Next(g *Game) (string, []string) {
	if g.Prompt != nil {
		return b.answer(g)
	}
	b.plan = nil
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
		b.rest, b.leave, b.pick, b.explore,
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

// heal drinks a healing grass when the HP is low: at a third for the
// careful bot, a fifth for the reckless one; with none, an unknown grass
// when in danger.
func (b *Bot) heal(g *Game) (string, []string) {
	low := g.MaxHP / 3
	if b.Style == BotReckless {
		low = g.MaxHP / 5
	}
	if g.HP > low {
		return "", nil
	}
	if it := carried(g, func(it *Item) bool { return b.known(g, it, EffectHeal) }); it != nil {
		return b.use(it, commandDrink)
	}
	if len(b.adjacent(g)) > 0 {
		if it := carried(g, func(it *Item) bool {
			return it.Def.Category == CategoryGrass && !g.identified[it.Def.ID]
		}); it != nil {
			return b.use(it, commandDrink)
		}
	}
	return "", nil
}

// expected returns the damage the player expects to deal to m in a turn,
// and to take from it, as a player who knows the kinds of monsters (not
// their HP) would reckon.
func expected(g *Game, m *Monster) (deal, take float64) {
	r := g.Dungeon.Rules
	k := float64(r.DefenseK)
	deal = float64(g.Attack()) * k / (k + float64(m.Defense)) * float64(r.PlayerHitRate) / 100
	take = float64(m.Attack) * k / (k + float64(g.Defense())) * float64(r.MonsterHitRate) / 100
	switch m.Speed {
	case SpeedDouble:
		take *= 2
	case SpeedSlow:
		take /= 2
	}
	return deal, take
}

// dangerous says whether fighting the enemies next to the player would
// likely cost it more HP than it has: each in turn, the others hitting
// meanwhile.
func dangerous(g *Game, ms []*Monster) bool {
	var turns, take float64
	for _, m := range ms {
		deal, t := expected(g, m)
		turns += float64(m.MaxHP) / max(deal, 0.1)
		take += t
	}
	return take*turns >= float64(g.HP)
}

// danger answers enemies next to the player it would likely lose to: a
// staff of blowing away (or an unknown staff) at the strongest, or away
// to the stairs, when they are known and free.
func (b *Bot) danger(g *Game) (string, []string) {
	ms := b.adjacent(g)
	if len(ms) == 0 || !dangerous(g, ms) {
		return "", nil
	}
	if g.Player == g.Stairs {
		return "stairs", nil
	}
	staff := carried(g, func(it *Item) bool {
		return it.Def.Category == CategoryStaff && (!it.Known || it.Charges > 0) &&
			(!g.identified[it.Def.ID] || it.Def.Effect == EffectBlowAway)
	})
	if staff != nil {
		m := slices.MaxFunc(ms, func(a, c *Monster) int { return a.Attack - c.Attack })
		if name := directionName(m.Pos.Sub(g.Player)); g.Facing != name {
			return "turn", []string{name}
		}
		return b.use(staff, commandWave)
	}
	if g.IsSeen(g.Stairs) {
		return b.toStairs(g)
	}
	return "", nil
}

// shoot shoots arrows at an enemy in a line from the player, not next to
// it, within an arrow's range, nothing in between.
func (b *Bot) shoot(g *Game) (string, []string) {
	if g.equipped(CategoryArrow) == nil {
		return "", nil
	}
	for _, m := range g.visibleEnemies() {
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

// threatened says whether an enemy in sight would likely beat the player
// one on one.
func (b *Bot) threatened(g *Game) bool {
	for _, m := range g.visibleEnemies() {
		if dangerous(g, []*Monster{m}) {
			return true
		}
	}
	return false
}

// hungry says whether the player is hungry with nothing to eat: time to
// go down and find food, not to rest.
func (b *Bot) hungry(g *Game) bool {
	return g.Fullness*100 <= g.Dungeon.Rules.Fullness*25 &&
		carried(g, func(it *Item) bool { return it.Def.Category == CategoryFood }) == nil
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
	if s := unknown(CategoryScroll); s != nil {
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
	if b.Style != BotCareful || g.HP*10 >= g.MaxHP*7 || len(g.visibleEnemies()) > 0 || b.windNear(g) {
		return "", nil
	}
	if g.Fullness*100 <= g.Dungeon.Rules.Fullness*40 {
		// Resting costs food.
		return "", nil
	}
	return "wait", nil
}

// windNear says whether the first wind is near: time to leave.
func (b *Bot) windNear(g *Game) bool {
	return g.FloorTurns >= g.Dungeon.Rules.WindTurns[0]-100
}

// leave goes to the stairs and down: the reckless bot as soon as it knows
// where they are, the careful one when the wind is near, it is hungry with
// nothing to eat, or it sees an enemy it would likely lose to.
func (b *Bot) leave(g *Game) (string, []string) {
	if !g.IsSeen(g.Stairs) || (b.Style == BotCareful && !b.windNear(g) && !b.hungry(g) && !b.threatened(g)) {
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
	full := len(g.Inventory) >= g.Dungeon.Rules.Capacity
	return b.walk(g, func(p Point) bool {
		it, ok := g.ItemAt(p)
		return ok && g.IsSeen(p) && p != g.Player && !it.ForSale &&
			(!full || it.Def.Category == CategoryMoney)
	})
}

// explore walks to the nearest tile seen next to one not seen yet.
func (b *Bot) explore(g *Game) (string, []string) {
	return b.walk(g, func(p Point) bool {
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
	})
}

// walk steps toward the nearest tile that goal accepts, over the tiles
// seen, around the traps found and the monsters; "" when none is
// reachable.
func (b *Bot) walk(g *Game, goal func(Point) bool) (string, []string) {
	from := map[Point]Point{g.Player: g.Player}
	queue := []Point{g.Player}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if goal(p) {
			for from[p] != g.Player {
				p = from[p]
			}
			// Onto the goods of a shop, a dash: it stops on them without
			// picking them up (leaving with them is stealing).
			if it, ok := g.ItemAt(p); ok && it.ForSale {
				return "dash", []string{directionName(p.Sub(g.Player))}
			}
			return "move", []string{directionName(p.Sub(g.Player))}
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
	return "", nil
}

// passable says whether the bot walks through p: not onto a trap found
// (unless found traps are safe).
func (b *Bot) passable(g *Game, p Point) bool {
	if !g.IsSeen(p) || !g.Level.At(p).Walkable() {
		return false
	}
	if g.foundTrapAt(p) != nil && !g.Dungeon.Rules.FoundTrapsSafe {
		return false
	}
	m := g.MonsterAt(p)
	return m == nil || m.Ally || !g.Visible(p)
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
}

// BotFloor is a floor of a bot's run, on arriving.
type BotFloor struct {
	Depth, Turn, HP, MaxHP int
}

// unfairTurns are the turns after arriving on a floor in which a death
// is unfair.
const unfairTurns = 10

// PlayBot plays a run of d with seed by b, up to maxActions actions.
func PlayBot(d *Dungeon, seed string, b *Bot, maxActions int) (BotRun, error) {
	g := NewGame(d, seed)
	r := BotRun{Seed: seed, Style: b.Style, Picked: map[Category]int{}, Used: map[Category]int{}}
	r.Floors = append(r.Floors, BotFloor{g.Depth, g.Turn, g.HP, g.MaxHP})
	for r.Actions < maxActions && g.Over == "" {
		hp, depth := g.HP, g.Depth
		action, args := b.Next(g)
		if err := g.Do(action, args); err != nil {
			return r, fmt.Errorf("%s %s: action %d %s %v: %w", d.ID, seed, r.Actions, action, args, err)
		}
		r.Actions++
		r.count(d, g.takeEvents())
		if g.Depth != depth && g.Over == "" {
			r.Floors = append(r.Floors, BotFloor{g.Depth, g.Turn, g.HP, g.MaxHP})
		}
		if g.Over != "" && !g.Cleared && (cause(g) == "monster" || cause(g) == "trap") {
			arrived := r.Floors[len(r.Floors)-1].Turn
			r.Unfair = hp == g.MaxHP || g.Turn-arrived <= unfairTurns
		}
	}
	r.Cleared, r.Over, r.Depth, r.Turns, r.Level = g.Cleared, g.Over, g.Depth, g.Turn, g.PlayerLevel
	r.Cause = cause(g)
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
				if f[0] == "pickup" {
					r.Picked[it.Category]++
				} else {
					r.Used[it.Category]++
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
