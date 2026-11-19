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
	// (Its first actions: whole runs are long now that the bots go deep.)
	a, err := PlayBot(d, "same", NewBot(BotReckless, 7), 1500)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlayBot(d, "same", NewBot(BotReckless, 7), 1500)
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

// TestBotShoots: arrows at an enemy in a line, facing it first.
func TestBotShoots(t *testing.T) {
	g, r := roomGame(t, 8, 5)
	b := NewBot(BotCareful, 1)
	arrows(g, "kiya", 5)
	g.Inventory[0].Equipped = true
	place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 3})
	g.Facing = "up"
	if a, args := b.Next(g); a != "turn" || !slices.Equal(args, []string{"downright"}) {
		t.Fatalf("%s %v", a, args)
	}
	g.Facing = "downright"
	if a, _ := b.Next(g); a != "shoot" {
		t.Fatalf("%s", a)
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

	// No staff: to the stairs, away from it.
	b.plan = nil
	g.Inventory = nil
	g.Stairs = Point{r.X, r.Y + 1}
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
