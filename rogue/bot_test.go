package rogue

import (
	"fmt"
	"reflect"
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
