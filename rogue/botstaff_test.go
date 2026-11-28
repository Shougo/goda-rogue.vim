package rogue

import "testing"

// TestBotWallOff: the bot walls off an enemy coming along a corridor with
// a staff of 壁作り, not one in a room (it walks round).
func TestBotWallOff(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X + 1, r.Y + 1}
	m := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	b := NewBot(BotCareful, 1)
	if _, ok := b.wallOff(g, []*Monster{m}); ok {
		t.Error("walled off in a room")
	}
	g.Level.set(Point{r.X + 2, r.Y + 1}, TileCorridor)
	if got, ok := b.wallOff(g, []*Monster{m}); !ok || got != m {
		t.Error("not walled off along a corridor")
	}
	staff := carry(g, "kabezue", 0)
	staff.Charges = 3
	answers := b.staffAnswers(g, []*Monster{m}, staff, staff.Def, 0)
	if len(answers) != 1 || answers[0].target != m || answers[0].chance != 0 {
		t.Errorf("answers %+v", answers)
	}
}

// TestBotLeapAway: with a staff of 飛びつき, the bot leaps the way that
// takes it to the stairs.
func TestBotLeapAway(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X, r.Y + 1}
	g.Stairs = Point{r.X + r.W - 1, r.Y + 1}
	g.markSeen(g.Stairs)
	place(g, kind("kuma1"), Point{r.X, r.Y})
	b := NewBot(BotCareful, 1)
	b.deciding = true
	dir, chance, ok := b.leapAway(g)
	if !ok || dir != "right" || chance != 0 {
		t.Errorf("leap %q %.2f %v, want right to the stairs", dir, chance, ok)
	}
	// The ways it worked out from where it would land are not kept as
	// the player's (the bots walked a step of nowhere).
	if step, _, ok := b.route(g, func(p Point) bool { return b.stairs(g, p) }); !ok || chebyshev(step, g.Player) != 1 {
		t.Errorf("then a step to %v from %v", step, g.Player)
	}
}

// TestBotGuide: with nothing left to explore and no stairs in sight, the
// bot waves a staff of 導き.
func TestBotGuide(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	for y := range g.Level.H {
		for x := range g.Level.W {
			g.markSeen(Point{x, y})
		}
	}
	g.Stairs = Point{-1, -1}
	staff := carry(g, "michibikizue", 0)
	staff.Charges = 2
	b := NewBot(BotCareful, 1)
	if a, _ := b.light(g); a != "inventory" || b.plan[0].item != staff || b.plan[1].label != commandWave {
		t.Errorf("%s %+v", a, b.plan)
	}
}
