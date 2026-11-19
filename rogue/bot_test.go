package rogue

import (
	"fmt"
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
	runs := 3
	if raceDetector {
		runs = 1
	}
	for style := range []BotStyle{BotCareful, BotReckless} {
		for i := range runs {
			t.Run(fmt.Sprint(BotStyle(style), i), func(t *testing.T) {
				t.Parallel()
				g := NewGame(d, fmt.Sprint("bot", i))
				b := NewBot(BotStyle(style), uint64(i))
				for n := 0; g.Over == ""; n++ {
					if n == 30000 {
						t.Fatalf("not over after %d actions, %dF", n, g.Depth)
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
	a, err := PlayBot(d, "same", NewBot(BotReckless, 7), 30000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlayBot(d, "same", NewBot(BotReckless, 7), 30000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v\n%+v", a, b)
	}
	if a.Over == "" || a.Cause == "unfinished" || a.Kills == 0 || a.Damage == 0 || len(a.Floors) != a.Depth ||
		a.Floors[0] != (BotFloor{1, 0, a.Floors[0].HP, a.Floors[0].MaxHP}) {
		t.Errorf("%+v", a)
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
	if !dangerous(g, []*Monster{m}) {
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
	if dangerousFloor(g) {
		t.Error("1F dangerous")
	}
	g.Depth = 30
	if !dangerousFloor(g) {
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
