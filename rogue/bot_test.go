package rogue

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestBotsPlay: the bots play runs to their end through Do only, without
// an error, without stealing, the game sound after every action
// (checkGame): the bots are random play too.
func TestBotsPlay(t *testing.T) {
	d := cave()
	// With the race detector, one run of each bot, its first actions: the
	// bots go deep, and it is several times slower.
	runs, actions := 3, 30000
	if raceDetector {
		runs, actions = 1, 1500
	}
	for style := range []BotStyle{BotCareful, BotReckless} {
		for i := range runs {
			t.Run(fmt.Sprint(BotStyle(style), i), func(t *testing.T) {
				t.Parallel()
				g := NewGame(d, fmt.Sprint("bot", i))
				b := NewBot(BotStyle(style), uint64(i))
				for n := 0; g.Over == ""; n++ {
					if n == actions {
						if !raceDetector {
							t.Fatalf("not over after %d actions, %dF", n, g.Depth)
						}
						break
					}
					action, args := b.Next(g)
					if err := g.Do(action, args); err != nil {
						t.Fatalf("%s %v: %v", action, args, err)
					}
					if strings.Contains(g.Message, sayThief) {
						t.Fatalf("stole, %dF turn %d", g.Depth, g.Turn)
					}
					if err := checkGame(g); err != nil {
						t.Fatalf("%dF turn %d, after %s %v: %v", g.Depth, g.Turn, action, args, err)
					}
				}
			})
		}
	}
}

// TestPlayBot: a run is the same for the same seed and bot, and is
// counted.
func TestPlayBot(t *testing.T) {
	d := cave()
	// (Its first actions: whole runs are long now that the bots go deep.
	// A seed whose run gets past 1F, whatever the floors are now.)
	var a BotRun
	var err error
	seed := ""
	for i := 0; i < 10 && a.Depth < 2; i++ {
		seed = fmt.Sprint("same", i)
		if a, err = PlayBot(d, seed, NewBot(BotReckless, 7), 1500); err != nil {
			t.Fatal(err)
		}
	}
	b, err := PlayBot(d, seed, NewBot(BotReckless, 7), 1500)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v\n%+v", a, b)
	}
	if a.Kills == 0 || a.Damage == 0 || len(a.Floors) != a.Depth || a.Depth < 2 ||
		a.Floors[0].Depth != 1 || a.Floors[0].Turn != 0 {
		t.Errorf("%+v", a)
	}
	picked := 0
	for _, f := range a.Floors {
		for _, n := range f.Picked {
			picked += n
		}
	}
	if total := sumCounts(a.Picked); picked != total || total == 0 {
		t.Errorf("picked %d on the floors, %d in all", picked, total)
	}
	// Too few actions: unfinished.
	c, err := PlayBot(d, "same", NewBot(BotReckless, 7), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cause != "unfinished" || c.Actions != 10 {
		t.Errorf("%+v", c)
	}
}

func TestBotRunCount(t *testing.T) {
	d := cave()
	r := BotRun{Picked: map[Category]int{}, Used: map[Category]int{}}
	r.count(d, []string{"3 pickup yakusou", "4 use yakusou", "5 kill nezumi1", "6 hurt 4 nezumi1", "7 hurt 2 落とし穴", "8 floor 2"})
	if r.Picked[CategoryGrass] != 1 || r.Used[CategoryGrass] != 1 || r.Kills != 1 || r.Damage != 6 {
		t.Errorf("%+v", r)
	}
}

// TestBotShoots: arrows at an enemy in a line worth them, facing it
// first; at any enemy with plenty of arrows; not at a weak one with few.
func TestBotShoots(t *testing.T) {
	g, r := roomGame(t, 8, 5)
	b := NewBot(BotCareful, 1)
	arrows(g, "kiya", 5)
	g.Inventory[0].Equipped = true
	m := place(g, kind("kuma1"), Point{r.X + 3, r.Y + 3})
	g.Facing = "up"
	if a, args := b.Next(g); a != "turn" || !slices.Equal(args, []string{"downright"}) {
		t.Fatalf("%s %v", a, args)
	}
	g.Facing = "downright"
	if a, _ := b.Next(g); a != "shoot" {
		t.Fatalf("%s", a)
	}

	m.Def = kind("nezumi1")
	m.HP, m.MaxHP, m.Attack, m.Defense = m.Def.HP, m.Def.HP, m.Def.Attack, m.Def.Defense
	if a, _ := b.Next(g); a == "shoot" {
		t.Fatal("a weak one shot")
	}
	g.Inventory[0].Count = plentyArrows
	if a, _ := b.Next(g); a != "shoot" {
		t.Fatalf("plenty: %s", a)
	}
}

// TestBotDanger: an enemy next to the player it would lose to is blown
// away with a staff, or left by the stairs.
func TestBotDanger(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("ryu4"), Point{r.X + 2, r.Y + 1})
	if !b.dangerous(g, []*Monster{m}) {
		t.Fatal("not dangerous")
	}
	staff := carry(g, "fukitobashi", 0)
	staff.Charges = 3
	g.Facing = "right"
	if a, _ := b.Next(g); a != "inventory" || len(b.plan) != 2 || b.plan[1].label != commandWave {
		t.Fatalf("%s %+v", a, b.plan)
	}

	// Against the wall, the dragon would not fly: no staff.
	b.plan = nil
	// A row with rock on the right of the room (not a doorway).
	y := r.Y + 1
	for g.Level.At(Point{r.X + r.W, y}).Walkable() {
		y++
	}
	g.Player = Point{r.X + r.W - 2, y}
	m.Pos = Point{r.X + r.W - 1, y}
	if n := blowAwayFlight(g, m); n != 0 {
		t.Fatalf("flight against the wall: %d", n)
	}
	if a, _ := b.Next(g); a == "inventory" && len(b.plan) == 2 && b.plan[1].label == commandWave {
		t.Fatalf("waved at a dragon against the wall: %+v", b.plan)
	}
	g.Player = Point{r.X + 1, y}
	m.Pos = Point{r.X + 2, y}
	// (The room is the first big enough on the floors of the cave: its
	// width and doorways change with the data.)
	if n, want := blowAwayFlight(g, m), min(r.W-3, blowAwayTiles); n != want {
		t.Fatalf("flight across the room: %d, want %d", n, want)
	}

	// No staff: to the stairs, away from it.
	b.plan = nil
	g.Inventory = nil
	g.Stairs = Point{r.X, y}
	for i := range g.Seen {
		g.Seen[i] = true
	}
	if a, args := b.Next(g); a != "move" || !slices.Equal(args, []string{"left"}) {
		t.Fatalf("%s %v", a, args)
	}
}

// TestDangerousFloor: a floor is dangerous when enough of its monsters
// (by the table) would beat the player.
func TestDangerousFloor(t *testing.T) {
	g := NewGame(cave(), "floor")
	b := NewBot(BotCareful, 1)
	if b.dangerousFloor(g) {
		t.Error("1F dangerous")
	}
	g.Depth = 30
	if !b.dangerousFloor(g) {
		t.Error("30F at level 1 not dangerous")
	}
}

// TestDangerousFar: one on one is reckoned next to the player, wherever
// the monster is: far away or of the table, it is as dangerous.
func TestDangerousFar(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X, r.Y}
	near := place(g, kind("kuma2"), Point{r.X + 1, r.Y})
	far := place(g, kind("kuma2"), Point{r.X + 7, r.Y + 2})
	// Across the floor: it would come after the turns reckoned.
	far.Pos = Point{g.Level.W - 1 - r.X, g.Level.H - 1 - r.Y}
	if chebyshev(far.Pos, g.Player) <= riskTurns {
		far.Pos = Point{r.X + riskTurns + 2, r.Y}
	}
	if !NewBot(BotCareful, 1).dangerous(g, []*Monster{far}) {
		t.Error("far: not dangerous")
	}
	if !NewBot(BotCareful, 1).dangerous(g, []*Monster{near}) {
		t.Error("near: not dangerous")
	}
}

// TestUseless: the bots keep only what they use, and drop the rest when
// the inventory is full and something is in sight.
func TestUseless(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	b := NewBot(BotCareful, 1)
	sword := carry(g, "katana", 0)
	sword.Equipped = true
	weak := carry(g, "bokutou", 0)
	strong := carry(g, "katana", 2)
	if !useless(g, weak) || useless(g, strong) || useless(g, sword) {
		t.Errorf("useless: weak %v, strong %v, equipped %v", useless(g, weak), useless(g, strong), useless(g, sword))
	}
	for len(g.Inventory) < g.Dungeon.Rules.Capacity {
		carry(g, "yakusou", 0)
	}
	putAtFoot(g, "onigiri").Pos = Point{r.X + 3, r.Y}
	strong.Equipped, sword.Equipped = true, false
	if a, _ := b.tidy(g); a != "inventory" || b.plan[1].label != commandDrop || (b.plan[0].item != weak && b.plan[0].item != sword) {
		t.Errorf("%s %+v", a, b.plan)
	}
}

// TestDeathChance: the chances of an attack add up to 1, and the chance of
// dying grows with the enemies and falls with the HP.
func TestDeathChance(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("kuma1"), Point{r.X + 2, r.Y + 1})
	var sum float64
	for _, p := range hitDamages(g, m) {
		sum += p
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("sum %v", sum)
	}
	g.HP = 1
	if d := deathChance(g, []*Monster{m}); d < 0.5 {
		t.Errorf("1 HP: %v", d)
	}
	g.HP = 1000
	if d := deathChance(g, []*Monster{m}); d != 0 {
		t.Errorf("1000 HP: %v", d)
	}
	g.HP = 30
	one := deathChance(g, []*Monster{m})
	n := place(g, kind("kuma1"), Point{r.X + 2, r.Y + 2})
	two := deathChance(g, []*Monster{m, n})
	if two <= one {
		t.Errorf("one %v, two %v", one, two)
	}
}

// sumCounts returns the sum of the counts of m.
func sumCounts(m map[Category]int) int {
	n := 0
	for _, k := range m {
		n += k
	}
	return n
}

// TestBotCheapestAnswer: in danger, the bot uses the cheapest answer that
// is enough: a healing grass before a staff, the staff without the grass;
// out of danger, neither.
func TestBotCheapestAnswer(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X + 1, r.Y + 1}
	place(g, kind("nezumi1"), Point{r.X + 2, r.Y + 1})
	g.Inventory = nil
	staff := carry(g, "fukitobashi", 0)
	staff.Charges = 3
	grass := carry(g, "yakusou", 0)
	// (A player strong enough that the grass heals a third of its HP:
	// worth less than a wave of the staff.)
	g.MaxHP = 150

	g.HP = g.MaxHP
	if a, _ := b.Next(g); a == "inventory" {
		t.Fatalf("full HP: %s %+v", a, b.plan)
	}
	g.HP = 2
	if a, _ := b.Next(g); a != "inventory" || b.plan[0].item != grass {
		t.Fatalf("with a grass: %s %+v", a, b.plan)
	}
	g.Inventory = []*Item{staff}
	g.Facing = "right"
	if a, _ := b.Next(g); a != "inventory" || b.plan[0].item != staff || b.plan[1].label != commandWave {
		t.Fatalf("with a staff: %s %+v", a, b.plan)
	}
}

// TestBotFirstStrike: the bot waits for an enemy two steps away to come
// next to it rather than step next to it; not for a dozing one.
func TestBotFirstStrike(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X, r.Y + 1}
	m := place(g, kind("nezumi1"), Point{r.X + 2, r.Y + 1})
	goal := func(p Point) bool { return p == Point{r.X + 4, r.Y + 1} }
	if a, _ := b.approach(g, goal); a != "wait" {
		t.Errorf("awake: %s", a)
	}
	m.Dozing = true
	if a, _ := b.approach(g, goal); a != "move" {
		t.Errorf("dozing: %s", a)
	}
}

// TestBotStepBack: two enemies next to the player in a room, a corridor
// behind it: the bot steps back into the corridor to fight them one by
// one, if that is enough.
func TestBotStepBack(t *testing.T) {
	g := find(t, func(g *Game) bool { return len(g.Level.Rooms) > 1 })
	b := NewBot(BotCareful, 1)
	g.Monsters, g.Items, g.Traps = nil, nil, nil
	g.Stairs = Point{-1, -1}
	// A corridor tile next to a room tile: the player on the room
	// tile, two enemies on the other side.
	var at, back Point
	found := false
	for y := 1; y < g.Level.H-1 && !found; y++ {
		for x := 1; x < g.Level.W-1 && !found; x++ {
			p := Point{x, y}
			if !g.Level.At(p).Walkable() || g.Level.RoomAt(p) < 0 {
				continue
			}
			for _, d := range []Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				q := p.Add(d)
				o := p.Sub(d)
				if g.Level.At(q).Walkable() && g.Level.RoomAt(q) < 0 && g.Level.RoomAt(o) >= 0 &&
					g.Level.RoomAt(o.Add(Point{d.Y, d.X})) >= 0 {
					at, back, found = p, q, true
					place(g, kind("kuma1"), o)
					place(g, kind("kuma1"), o.Add(Point{d.Y, d.X}))
					break
				}
			}
		}
	}
	if !found {
		t.Skip("no room door")
	}
	g.Player = at
	ms := b.adjacent(g)
	if len(ms) != 2 {
		t.Fatalf("%d next", len(ms))
	}
	name, ok := b.stepBack(g, ms)
	if !ok || at.Add(Directions[name]) != back {
		t.Fatalf("%v %v", name, ok)
	}
	if oneByOneDeath(g, ms, g.HP) >= deathChance(g, ms) {
		t.Error("one by one is not safer")
	}
}

// TestBotChecksHouse: in a monster house whose monsters sleep, the bot
// swings at a tile not known safe before stepping on it.
func TestBotChecksHouse(t *testing.T) {
	g := specialGame(t, "bighouse", func(c *FloorCounts) { c.BigRoom, c.House = 100, 100 })
	b := NewBot(BotCareful, 1)
	var p Point
	found := false
	for _, name := range clockwise {
		d := Directions[name]
		if q := g.Player.Add(d); g.canStep(d) && g.MonsterAt(q) == nil && !g.IsSafe(q) {
			p, found = q, true
			break
		}
	}
	if !found {
		t.Skip("no tile to check")
	}
	if !b.checkFirst(g, p) {
		t.Fatal("no check")
	}
	g.markSafe(p)
	if b.checkFirst(g, p) {
		t.Error("checks a safe tile")
	}
	for _, m := range g.Monsters {
		m.Dozing = false
	}
	i, _ := g.index(p)
	g.Safe[i] = false
	if b.checkFirst(g, p) {
		t.Error("checks with the house awake")
	}
}

// TestBotItemCost: an item costs less the more of its kind the bot
// carries and the deeper it is; a staff's last wave costs the most.
func TestBotItemCost(t *testing.T) {
	g, _ := roomGame(t, 3, 3)
	b := NewBot(BotCareful, 1)
	g.Inventory = nil
	grass := carry(g, "yakusou", 0)
	kinds := []*ItemDef{grass.Def}
	one := b.itemCost(g, grass, kinds)
	carry(g, "yakusou", 0)
	if two := b.itemCost(g, grass, kinds); two >= one {
		t.Errorf("two %v, one %v", two, one)
	}
	g.Depth = g.Dungeon.Floors
	if deep := b.itemCost(g, grass, kinds); deep >= one/2 {
		t.Errorf("deep %v, one %v", deep, one)
	}
	staff := carry(g, "fukitobashi", 0)
	staff.Charges = 4
	many := b.itemCost(g, staff, []*ItemDef{staff.Def})
	staff.Charges = 1
	if last := b.itemCost(g, staff, []*ItemDef{staff.Def}); last <= many {
		t.Errorf("last %v, many %v", last, many)
	}
}

// TestBotRestsInRoom: with low HP in a corridor, the careful bot walks
// into a room to rest.
func TestBotRestsInRoom(t *testing.T) {
	g := find(t, func(g *Game) bool { return len(g.Level.Rooms) > 1 })
	b := NewBot(BotCareful, 1)
	g.Monsters, g.Items, g.Traps = nil, nil, nil
	var corridor Point
	found := false
	for y := 1; y < g.Level.H-1 && !found; y++ {
		for x := 1; x < g.Level.W-1 && !found; x++ {
			p := Point{x, y}
			if g.Level.At(p) == TileCorridor {
				for _, d := range Directions {
					if g.entrance(p.Add(d)) {
						corridor, found = p, true
					}
				}
			}
		}
	}
	if !found {
		t.Skip("no corridor by a room")
	}
	g.Player = corridor
	for i := range g.Seen {
		g.Seen[i] = true
	}
	g.HP = g.MaxHP / 4
	if a, _ := b.rest(g); a != "move" {
		t.Fatalf("in a corridor: %s", a)
	}
	g.Player = Point{g.Level.Rooms[0].X + 1, g.Level.Rooms[0].Y + 1}
	if a, _ := b.rest(g); a != "wait" {
		t.Fatalf("in a room: %s", a)
	}
}

// TestBotThreats: enemies coming at the player count in a fight from
// when they come; a monster house awake is answered before contact.
func TestBotThreats(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X, r.Y + 1}
	g.HP = 12
	near := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	alone := fightDeath(g, []*Monster{near}, g.HP, 0)
	far := place(g, kind("kuma1"), Point{r.X + 4, r.Y + 1})
	if arrival(g, far) != 3 {
		t.Errorf("arrival %d", arrival(g, far))
	}
	if ms := b.threats(g); len(ms) != 2 {
		t.Fatalf("%d threats", len(ms))
	}
	if both := fightDeath(g, b.threats(g), g.HP, 0); both <= alone {
		t.Errorf("both %v, alone %v", both, alone)
	}
	far.Dozing = true
	if ms := b.threats(g); len(ms) != 1 {
		t.Errorf("a dozing one counts: %d", len(ms))
	}
}

// TestBotNoPacing: the careful bot does not walk back and forth between
// two tiles, as it did between an item far away and the stairs (some 8
// times a run).
func TestBotNoPacing(t *testing.T) {
	d := cave()
	paced := 0
	for i := range 4 {
		g := NewGame(d, fmt.Sprint("pace", i))
		b := NewBot(BotCareful, uint64(i))
		var pos []Point
		for n := 0; n < 60000 && g.Over == ""; n++ {
			a, args := b.Next(g)
			if err := g.Do(a, args); err != nil {
				t.Fatal(err)
			}
			if a != "move" && a != "dash" {
				pos = nil
				continue
			}
			pos = append(pos, g.Player)
			// Seven steps on two tiles.
			if k := len(pos); k >= 7 && pos[k-1] != pos[k-2] && pos[k-1] == pos[k-3] && pos[k-1] == pos[k-5] &&
				pos[k-1] == pos[k-7] && pos[k-2] == pos[k-4] && pos[k-2] == pos[k-6] {
				paced++
				pos = nil
			}
		}
	}
	if paced > 6 {
		t.Errorf("paced %d times in 4 runs", paced)
	}
}

// TestBotLastFloor: on the last floor, the careful bot goes to the stairs
// it sees, whatever lies around; on another, it does not yet.
func TestBotLastFloor(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Stairs = Point{r.X + 6, r.Y + 1}
	g.Player = Point{r.X + 2, r.Y + 1}
	g.Items = append(g.Items, &Item{Def: g.Dungeon.Item("yakusou"), Glyph: "草", Pos: Point{r.X, r.Y + 1}})
	g.look()
	b := NewBot(BotCareful, 1)
	if a, args := b.leave(g); a != "" {
		t.Fatalf("on %dF it leaves already: %s %v", g.Depth, a, args)
	}
	g.Depth = g.Dungeon.Floors
	for range 4 {
		a, args := b.Next(g)
		if err := g.Do(a, args); err != nil {
			t.Fatal(err)
		}
	}
	if g.Player != g.Stairs {
		t.Errorf("on the last floor: at %v after 4 actions, the stairs at %v", g.Player, g.Stairs)
	}
}

// full fills the inventory of g with grasses of confusion, and returns
// one of them.
func full(g *Game) *Item {
	var it *Item
	for len(g.Inventory) < g.Dungeon.Rules.Capacity {
		it = carry(g, "konran", 0)
	}
	return it
}

// TestBotTrade: with no room left, the bot does not walk past food: it
// walks to it, and on it eats it if none is wasted, else swaps what it
// values least for it; it gives an empty staff up first, and nothing for
// what is worth no more than what it carries.
func TestBotTrade(t *testing.T) {
	setup := func() (*Game, *Bot, *Item) {
		g, r := roomGame(t, 8, 3)
		g.Player = Point{r.X + 1, r.Y + 1}
		food := &Item{Def: g.Dungeon.Item("ookiionigiri"), Glyph: "食", Pos: Point{r.X + 3, r.Y + 1}}
		g.Items = append(g.Items, food)
		full(g)
		g.look()
		return g, NewBot(BotCareful, 1), food
	}
	step := func(g *Game, b *Bot) string {
		t.Helper()
		a, args := b.Next(g)
		if err := g.Do(a, args); err != nil {
			t.Fatal(err)
		}
		for g.Prompt != nil {
			pa, pargs := b.Next(g)
			if err := g.Do(pa, pargs); err != nil {
				t.Fatal(err)
			}
		}
		return a
	}

	// Hungry enough: it walks to the food and eats it where it lies.
	g, b, food := setup()
	g.Fullness = g.Dungeon.Rules.Fullness - food.Def.Strength
	for range 3 {
		step(g, b)
	}
	if slices.Contains(g.Items, food) || g.Fullness < g.Dungeon.Rules.Fullness-1 || len(g.Inventory) != g.Dungeon.Rules.Capacity {
		t.Fatalf("food on the floor %v, fullness %d, %d carried", slices.Contains(g.Items, food), g.Fullness, len(g.Inventory))
	}

	// Full: it swaps an empty staff for it, before any grass.
	g, b, food = setup()
	staff := g.Inventory[len(g.Inventory)-1]
	*staff = Item{Def: g.Dungeon.Item("fukitobashi"), Glyph: "杖", Known: true}
	g.sortInventory()
	for range 3 {
		step(g, b)
	}
	if !slices.Contains(g.Inventory, food) || !slices.Contains(g.Items, staff) || len(g.Inventory) != g.Dungeon.Rules.Capacity {
		t.Fatalf("carried food %v, staff on the floor %v", slices.Contains(g.Inventory, food), slices.Contains(g.Items, staff))
	}
	// Nothing more: the staff it stands on is worth nothing.
	if a, _ := b.trade(g); a != "" {
		t.Errorf("it trades again: %s", a)
	}

	// With grasses only, one of them goes.
	g, b, food = setup()
	for range 3 {
		step(g, b)
	}
	if !slices.Contains(g.Inventory, food) {
		t.Error("no grass given up for the food")
	}

	// Another grass of the kind it is full of is not worth a trade.
	g, b, food = setup()
	*food = Item{Def: g.Dungeon.Item("konran"), Glyph: "草", Pos: food.Pos}
	if b.wants(g, food.Pos) || b.least(g, food) != nil {
		t.Error("it wants one more grass of confusion for one of them")
	}
}

// TestFoodWorth: food is worth more to the bot the less fullness it
// holds, in its belly, its bag and its pots: a lot below 200, little
// above 300.
func TestFoodWorth(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	big := &Item{Def: g.Dungeon.Item("ookiionigiri")}
	g.Fullness = 50
	hungry := foodWorth(g, big)
	carry(g, "ookiionigiri", 0)
	some := foodWorth(g, big) // 150
	p := pot(g, "hozon", 3)
	p.Contents = []*Item{{Def: g.Dungeon.Item("ookiionigiri")}}
	base := foodWorth(g, big) // 250
	carry(g, "ookiionigiri", 0)
	plenty := foodWorth(g, big) // 350
	if !(hungry > some && some > 5 && base < 5 && base > 1 && plenty == foodSpare) {
		t.Errorf("worth %.2f at 50, %.2f at 150, %.2f at 250, %.2f at 350", hungry, some, base, plenty)
	}
	// What it carries is worth what it would hold without it.
	own := g.Inventory[slices.IndexFunc(g.Inventory, func(it *Item) bool { return it.Def.Category == CategoryFood })]
	if w := foodWorth(g, own); w != base {
		t.Errorf("an onigiri carried at 350: %.2f, want %.2f (250 without it)", w, base)
	}
	// With plenty, it does not give a grass up for one more.
	g.Fullness = g.Dungeon.Rules.Fullness
	full(g)
	b := NewBot(BotCareful, 1)
	if b.least(g, big) != nil {
		t.Error("with 400 of fullness it trades for one more onigiri")
	}
}

// TestBotStraight: of the ways of a same length, the bot takes a
// straight one.
func TestBotStraight(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X + 1, r.Y + 1}
	g.Stairs = Point{r.X + 5, r.Y + 1}
	g.look()
	g.Depth = g.Dungeon.Floors
	b := NewBot(BotCareful, 1)
	for range 4 {
		a, args := b.Next(g)
		if a != "move" || args[0] != "right" {
			t.Fatalf("%s %v, want four steps to the right", a, args)
		}
		if err := g.Do(a, args); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBotPromising: a corridor is worth exploring while a room of its
// section or of one next to it is not seen; in a room, always.
func TestBotPromising(t *testing.T) {
	g := find(t, func(g *Game) bool { return !g.Level.Maze && len(g.Level.Rooms) >= 8 })
	b := NewBot(BotCareful, 1)
	var corridor Point
	for i, tile := range g.Level.Tiles {
		if tile == TileCorridor {
			corridor = Point{i % g.Level.W, i / g.Level.W}
			break
		}
	}
	if !b.promising(g, corridor) {
		t.Fatal("a corridor of a floor just arrived on is not promising")
	}
	for i := range g.Level.Tiles {
		g.markSeen(Point{i % g.Level.W, i / g.Level.W})
	}
	// Every room seen: with no section without a room around, nothing.
	dummy := map[int]bool{}
	for s := range gridCols * gridRows {
		dummy[s] = true
	}
	for _, r := range g.Level.Rooms {
		delete(dummy, section(Point{r.X, r.Y}))
	}
	near := false
	s := section(corridor)
	for _, d := range []int{0, 1, -1, gridCols, -gridCols} {
		near = near || dummy[s+d]
	}
	if got := b.promising(g, corridor); got != near {
		t.Errorf("all seen: promising %v, a section without a room around %v", got, near)
	}
	// (Not a dummy room: one tile of a corridor.)
	i := slices.IndexFunc(g.Level.Rooms, func(r Room) bool { return !r.Dummy })
	r := g.Level.Rooms[i]
	if !b.promising(g, Point{r.X, r.Y}) {
		t.Error("a tile of a room is not promising")
	}
}

// TestBotPacingShut: after walking back and forth between two tiles, the
// bot keeps off both for a while.
func TestBotPacingShut(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.look()
	b := NewBot(BotCareful, 1)
	a, c := Point{r.X + 1, r.Y + 1}, Point{r.X + 2, r.Y + 1}
	for i := range 7 {
		g.Player = []Point{a, c}[i%2]
		b.pacing(g)
	}
	if b.passable(g, a) || b.passable(g, c) {
		t.Fatalf("shut %v: the tiles are still open", b.shut)
	}
	g.Turn += shutTurns
	if !b.passable(g, a) {
		t.Error("the tiles stay shut")
	}
}

// TestBotOnDecoy: standing on what looks like stairs (blown there), the
// bot goes down them; it does not ask for a step to where it stands.
func TestBotOnDecoy(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Stairs = Point{r.X + 4, r.Y}
	g.look()
	g.Decoys = []Decoy{{Pos: g.Player, Def: kind("nezumi1")}}
	b := NewBot(BotReckless, 0)
	if a, args := b.toStairs(g); a != "stairs" {
		t.Errorf("%s %v", a, args)
	}
}

// TestBotAwait: the bot waits for an enemy two tiles away, coming, to
// strike first; not twice when it did not come.
func TestBotAwait(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 1, r.Y + 1}
	place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	b := NewBot(BotCareful, 1)
	if a, _ := b.Next(g); a != "wait" || b.rule != "敵を待つ" {
		t.Fatalf("%s [%s]", a, b.rule)
	}
	if a, _ := b.Next(g); a == "wait" && b.rule == "敵を待つ" {
		t.Errorf("waited again for one that did not come")
	}
}

// TestBotChasesEagle: the bot does not wait for an eagle a knight's move
// away, which swoops from there and never comes: it goes next to it.
func TestBotChasesEagle(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 1, r.Y + 1}
	place(g, kind("washi1"), Point{r.X + 3, r.Y + 2})
	b := NewBot(BotCareful, 1)
	a, args := b.Next(g)
	if a != "move" || b.rule != "離れて打つ敵を追う" {
		t.Fatalf("%s %v [%s]", a, args, b.rule)
	}
}

// TestBotSealedMouth: with its mouth sealed, the hungry bot does not try
// to eat, again and again.
func TestBotSealedMouth(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	carry(g, "ookiionigiri", 0)
	g.Fullness = 0
	b := NewBot(BotCareful, 1)
	if a, _ := b.Next(g); a != "inventory" || b.rule != "食べる" {
		t.Fatalf("not sealed: %s [%s]", a, b.rule)
	}
	g.Sealed = true
	if a, _ := b.Next(g); a == "inventory" {
		t.Errorf("sealed: %s [%s] %+v", a, b.rule, b.plan)
	}
}

// TestBotRegenBracelet: the bot wears a bracelet of regeneration only to
// heal, and wears another one the rest of the time.
func TestBotRegenBracelet(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	b := NewBot(BotCareful, 1)
	regen := carry(g, "kaifuku", 0)
	regen.Known = true
	other := carry(g, "kehai", 0)
	other.Equipped = true
	g.Fullness = g.Dungeon.Rules.Fullness
	carry(g, "ookiionigiri", 0)
	if a, _ := b.equip(g); a != "" {
		t.Errorf("at full HP: %s", a)
	}
	g.HP = g.MaxHP / 2
	if a, _ := b.equip(g); a != "inventory" || b.plan[0].item != regen || b.plan[1].label != commandEquip {
		t.Errorf("hurt: %s %+v", a, b.plan)
	}
	b.plan = nil
	g.equip(regen)
	g.HP = g.MaxHP
	if a, _ := b.equip(g); a != "inventory" || b.plan[0].item != other || b.plan[1].label != commandEquip {
		t.Errorf("healed: %s %+v", a, b.plan)
	}
	// Neither is useless: one of each use.
	if useless(g, regen) || useless(g, other) {
		t.Errorf("useless: regen %v, other %v", useless(g, regen), useless(g, other))
	}
	// Hurt with no food to spare: it does not put it on.
	b.plan = nil
	g.equip(other)
	g.HP, g.Fullness = g.MaxHP/2, 10
	for i := len(g.Inventory) - 1; i >= 0; i-- {
		if g.Inventory[i].Def.Category == CategoryFood {
			g.Inventory = slices.Delete(g.Inventory, i, i+1)
		}
	}
	if a, _ := b.equip(g); a != "" {
		t.Errorf("hungry: %s", a)
	}
}

// TestGearWorth: the bots count the ability born with and the seals of a
// weapon, not its strength only.
func TestGearWorth(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	plain := carry(g, "tetsuken", 0) // 7
	sure := carry(g, "hicchuu", 0)   // 4, and 必中: 4
	sealed := carry(g, "tetsuken", 0)
	sealed.Seals = []string{"会", "会"}
	if !(gearWorth(g, sure) > gearWorth(g, plain) && gearWorth(g, sealed) > gearWorth(g, plain)) {
		t.Errorf("worth: plain %.1f, 必中の剣 %.1f, with 2 seals 会 %.1f", gearWorth(g, plain), gearWorth(g, sure), gearWorth(g, sealed))
	}
	plain.Equipped = true
	if useless(g, sure) || !useless(g, carry(g, "bokutou", 0)) {
		t.Error("useless: wrong weapon")
	}
}

// TestTraceBot: the trace of a run tells each choice and its rule.
func TestTraceBot(t *testing.T) {
	var lines []string
	over := TraceBot(cave(), "trace", NewBot(BotCareful, 1), 30, func(l string) { lines = append(lines, l) })
	if over != "" || len(lines) == 0 || !strings.Contains(lines[0], "1F T0") || !strings.Contains(lines[0], "[") {
		t.Errorf("over %q, lines %q", over, lines)
	}
}
