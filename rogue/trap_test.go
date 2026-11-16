package rogue

import (
	"strings"
	"testing"
)

// trap hides the trap id on p.
func trap(g *Game, id string, p Point) *Trap {
	t := &Trap{Def: g.Dungeon.Trap(id), Pos: p}
	g.Traps = append(g.Traps, t)
	return t
}

// trapGame is a game in a room, with nothing coming to bother the player.
func trapGame(t *testing.T) (*Game, Room) {
	t.Helper()
	g, r := roomGame(t, 6, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.SpawnTurns = 1 << 30 })
	g.HP, g.MaxHP = 100, 100
	return g, r
}

func move(t *testing.T, g *Game, dir string) {
	t.Helper()
	if err := g.Do("move", []string{dir}); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceTraps(t *testing.T) {
	for _, seed := range []string{"a", "b", "c", "d"} {
		g := NewGame(cave(), seed)
		c := g.Dungeon.counts(1)
		if n := len(g.Traps); n > c.MaxTrap {
			t.Errorf("%s: %d traps", seed, n)
		}
		for _, tr := range g.Traps {
			p := tr.Pos
			if !g.Level.At(p).Walkable() || p == g.Player || p == g.Stairs || tr.Found {
				t.Errorf("%s: a trap at %v", seed, p)
			}
			if _, ok := g.ItemAt(p); ok {
				t.Errorf("%s: a trap under an item at %v", seed, p)
			}
		}
	}
}

func TestSwingChecks(t *testing.T) {
	g, r := trapGame(t)
	g.Facing = "right"
	front := Point{r.X + 1, r.Y}
	tr := trap(g, "dokuya", front)
	if s, _ := g.glyph(front); s == glyphTrap {
		t.Fatal("a hidden trap shows")
	}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if !tr.Found || g.Message != "空振りした。毒矢の罠を見つけた！" || g.Turn != 1 {
		t.Errorf("found %v, message %q, turn %d", tr.Found, g.Message, g.Turn)
	}
	if s, group := g.glyph(front); s != glyphTrap || group != hlTrap {
		t.Errorf("%q %q", s, group)
	}
	if g.IsSafe(front) {
		t.Error("a trap is safe")
	}

	// Nothing there: the tile is safe.
	g.Facing = "down"
	below := Point{r.X, r.Y + 1}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if !g.IsSafe(below) {
		t.Error("a checked tile is not safe")
	}

	// A found trap does not go off (as the rule says).
	hp := g.HP
	move(t, g, "right")
	if g.Player != front || g.HP != hp || !strings.Contains(g.Message, "毒矢の罠がある。") {
		t.Errorf("at %v, HP %d, message %q", g.Player, g.HP, g.Message)
	}
	// Unless the rule says it does.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.FoundTrapsSafe = false })
	move(t, g, "left")
	move(t, g, "right")
	if g.HP >= hp {
		t.Errorf("HP %d", g.HP)
	}
}

func TestTraps(t *testing.T) {
	for _, tc := range []struct {
		id    string
		check func(*Game, Point) string
	}{
		{"otoshiana", func(g *Game, _ Point) string {
			if g.Depth != 2 || g.HP > 95 {
				return "not fallen"
			}
			return ""
		}},
		{"jirai", func(g *Game, _ Point) string {
			if g.HP != 50 {
				return "HP not halved"
			}
			return ""
		}},
		{"dokuya", func(g *Game, _ Point) string {
			if g.HP != 95 || g.Strength != 7 {
				return "not poisoned"
			}
			return ""
		}},
		{"suimin", func(g *Game, _ Point) string {
			if g.Turn != 6 || !strings.Contains(g.Message, "目が覚めた。") {
				return "not slept"
			}
			return ""
		}},
		{"warp", func(g *Game, p Point) string {
			if g.Player == p {
				return "not warped"
			}
			return ""
		}},
		{"harahe", func(g *Game, _ Point) string {
			if g.Fullness != 90 {
				return "not hungry"
			}
			return ""
		}},
	} {
		g, r := trapGame(t)
		p := Point{r.X + 1, r.Y}
		tr := trap(g, tc.id, p)
		move(t, g, "right")
		if !tr.Found || !strings.Contains(g.Message, tr.Def.Name+"だ！") {
			t.Errorf("%s: message %q", tc.id, g.Message)
		}
		if msg := tc.check(g, p); msg != "" {
			t.Errorf("%s: %s: HP %d, turn %d, message %q", tc.id, msg, g.HP, g.Turn, g.Message)
		}
	}
}

func TestSnare(t *testing.T) {
	g, r := trapGame(t)
	trap(g, "torabasami", Point{r.X + 1, r.Y})
	move(t, g, "right")
	n := g.Stuck
	if n != 10 {
		t.Fatalf("stuck %d", n)
	}
	at := g.Player
	for i := range n {
		move(t, g, "right")
		if (g.Player != at) != (i == n) {
			t.Fatalf("moved after %d turns", i+1)
		}
	}
	if g.Stuck != 0 || !strings.Contains(g.Message, "抜け出した") {
		t.Errorf("stuck %d, message %q", g.Stuck, g.Message)
	}
	move(t, g, "right")
	if g.Player == at {
		t.Error("cannot move after the snare")
	}
}

func TestRust(t *testing.T) {
	g, r := trapGame(t)
	sword := carry(g, "tetsuken", 1)
	shield := carry(g, "kitate", 0)
	sword.Equipped, shield.Equipped = true, true
	trap(g, "sabi", Point{r.X + 1, r.Y})
	move(t, g, "right")
	if sword.Upgrade != 0 || shield.Upgrade != -1 {
		t.Errorf("upgrades %d %d", sword.Upgrade, shield.Upgrade)
	}
}

func TestDashStopsAtTraps(t *testing.T) {
	g, r := trapGame(t)
	tr := trap(g, "harahe", Point{r.X + 3, r.Y})
	tr.Found = true
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != (Point{r.X + 2, r.Y}) {
		t.Errorf("at %v, before a trap at %v", g.Player, tr.Pos)
	}

	// A hidden trap stops a dash when it goes off.
	g.Player = Point{r.X, r.Y + 1}
	hidden := trap(g, "harahe", Point{r.X + 2, r.Y + 1})
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != hidden.Pos {
		t.Errorf("at %v, the trap at %v", g.Player, hidden.Pos)
	}
}
