package rogue

import (
	"strings"
	"testing"
)

func TestParseAbilities(t *testing.T) {
	as, err := parseAbilities("怒り:33:150 ふらふら:50")
	if err != nil {
		t.Fatal(err)
	}
	want := []Ability{{AbilityAnger, []int{33, 150}}, {AbilityErratic, []int{50}}}
	if len(as) != len(want) {
		t.Fatalf("got %v, want %v", as, want)
	}
	for i := range want {
		if as[i].Name != want[i].Name || len(as[i].Values) != len(want[i].Values) {
			t.Fatalf("got %v, want %v", as, want)
		}
		for j := range want[i].Values {
			if as[i].Values[j] != want[i].Values[j] {
				t.Fatalf("got %v, want %v", as, want)
			}
		}
	}
	for _, s := range []string{"", "-"} {
		if as, err := parseAbilities(s); err != nil || as != nil {
			t.Errorf("%q: got %v, %v; want none", s, as, err)
		}
	}
}

// TestMoveDoubleAttacksOnce: a monster that moves twice a turn attacks
// once, and a fast one twice.
func TestMoveDoubleAttacksOnce(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want int
	}{{"ookami1", 1}, {"hachi1", 2}} {
		g, r := roomGame(t, 4, 3)
		g.HP, g.MaxHP = 9999, 9999
		d := *g.Dungeon
		d.Rules.MonsterHitRate = 100
		g.Dungeon = &d
		place(g, kind(tc.id), Point{r.X + 1, r.Y})
		wait(t, g, 1)
		if n := strings.Count(g.Message, "の攻撃。"); n != tc.want {
			t.Errorf("%s: %d attacks in a turn, want %d (%q)", tc.id, n, tc.want, g.Message)
		}
	}
}

// TestAnger: at a third of its HP or below, an oni's attack goes up,
// once.
func TestAnger(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	m := place(g, kind("oni1"), Point{r.X + 1, r.Y})
	attack := m.Attack
	m.HP = m.MaxHP/3 + 2
	g.hurt(m, 1)
	if m.Enraged || m.Attack != attack {
		t.Fatalf("above a third of its HP: angry %v, attack %d (was %d)", m.Enraged, m.Attack, attack)
	}
	m.HP = m.MaxHP / 3
	g.hurt(m, 1)
	if !m.Enraged || m.Attack != attack*150/100 {
		t.Fatalf("at a third of its HP: angry %v, attack %d, want %d", m.Enraged, m.Attack, attack*150/100)
	}
	g.hurt(m, 1)
	if m.Attack != attack*150/100 {
		t.Errorf("angry twice: attack %d, want %d", m.Attack, attack*150/100)
	}
}

// TestCritical: a bear's critical blows do 1.5 times the damage.
func TestCritical(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	m := place(g, kind("kuma2"), Point{r.X + 1, r.Y})
	crit, plain := 0, 0
	for range 400 {
		if n := g.critical(m, 10, true); n == 15 {
			crit++
		} else if n == 10 {
			plain++
		} else {
			t.Fatalf("damage %d, want 10 or 15", n)
		}
	}
	if crit < 60 || crit > 140 {
		t.Errorf("%d critical blows in 400, want about 100", crit)
	}
	if n := g.critical(newMonster(kind("kuma1")), 10, true); n != 10 {
		t.Errorf("a cub: damage %d, want 10 (no critical blows)", n)
	}
}

// TestErratic: a ghost steps at random about half of the time, even next
// to the player.
func TestErratic(t *testing.T) {
	g, r := roomGame(t, 5, 5)
	m := place(g, kind("rei1"), Point{r.X + 2, r.Y + 2})
	n := 0
	for range 400 {
		m.Pos = Point{r.X + 2, r.Y + 2}
		before := m.Pos
		if g.erratic(m) {
			n++
			if chebyshev(before, m.Pos) != 1 {
				t.Fatalf("stepped from %v to %v", before, m.Pos)
			}
		}
	}
	if n < 160 || n > 240 {
		t.Errorf("%d random steps in 400, want about 200", n)
	}
}

// TestAsleep: the share asleep on a new floor is the kind's (75% for
// bears), a third by default.
func TestAsleep(t *testing.T) {
	g, _ := roomGame(t, 1, 1)
	for _, tc := range []struct {
		id     string
		lo, hi int
	}{{"kuma1", 250, 350}, {"nezumi1", 100, 170}} {
		m := newMonster(kind(tc.id))
		n := 0
		for range 400 {
			if m.dozes(g.rng) {
				n++
			}
		}
		if n < tc.lo || n > tc.hi {
			t.Errorf("%s: %d asleep in 400, want %d to %d", tc.id, n, tc.lo, tc.hi)
		}
	}
}

// TestCharge: a boar on the player's line rushes up, hurts it and blows
// it away; a monster in between takes it instead; next to the player it
// only hits.
func TestCharge(t *testing.T) {
	g, r := roomGame(t, 12, 1)
	g.HP, g.MaxHP = 999, 999
	b := place(g, kind("inoshishi1"), Point{r.X + 4, r.Y})
	g.act(b)
	if b.Pos != (Point{r.X + 1, r.Y}) {
		t.Errorf("the boar at %v, want next to the player at %v", b.Pos, g.Player)
	}
	if want := (Point{r.X, r.Y}); g.Player != want {
		t.Errorf("the player at %v, want %v (against the wall)", g.Player, want)
	}
	if want := 999 - 5 - blowAwayDamage; g.HP != want {
		t.Errorf("HP %d, want %d (the charge and the bump)", g.HP, want)
	}

	// Blown away from the wall, over the room.
	g, r = roomGame(t, 12, 1)
	g.HP, g.MaxHP = 999, 999
	g.Player = Point{r.X + 3, r.Y}
	b = place(g, kind("inoshishi1"), Point{r.X, r.Y})
	g.act(b)
	if g.Player.X <= r.X+3 {
		t.Errorf("the player at %v, want blown away to the right", g.Player)
	}

	// A monster in between takes the charge.
	g, r = roomGame(t, 12, 1)
	g.HP, g.MaxHP = 999, 999
	b = place(g, kind("inoshishi1"), Point{r.X + 5, r.Y})
	m := place(g, kind("kame4"), Point{r.X + 2, r.Y})
	hp := m.HP
	g.act(b)
	if g.HP != 999 || m.HP >= hp {
		t.Errorf("HP %d, the turtle %d/%d: want the turtle hurt and not the player", g.HP, m.HP, hp)
	}

	// Out of reach, it walks.
	g, r = roomGame(t, 12, 1)
	b = place(g, kind("inoshishi1"), Point{r.X + 7, r.Y})
	g.act(b)
	if chebyshev(b.Pos, g.Player) != 6 || g.Player != (Point{r.X, r.Y}) {
		t.Errorf("out of reach: the boar at %v, the player at %v; want a step closer", b.Pos, g.Player)
	}
}

// TestPull: a frog pulls the player along its line next to it; its rank
// 4 pulls from anywhere in its room.
func TestPull(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	f := place(g, kind("kaeru1"), Point{r.X + 3, r.Y})
	g.act(f)
	if want := (Point{r.X + 2, r.Y}); g.Player != want {
		t.Errorf("the player at %v, want %v", g.Player, want)
	}

	// Not on its line: it walks.
	g, r = roomGame(t, 6, 3)
	f = place(g, kind("kaeru1"), Point{r.X + 3, r.Y + 1})
	g.act(f)
	if g.Player != (Point{r.X, r.Y}) {
		t.Errorf("off its line: the player at %v, want not pulled", g.Player)
	}

	// Rank 4: anywhere in its room, by chance.
	g, r = roomGame(t, 6, 3)
	f = place(g, kind("kaeru4"), Point{r.X + 5, r.Y + 2})
	pulled := false
	for range 100 {
		g.act(f)
		if chebyshev(g.Player, f.Pos) == 1 {
			pulled = true
			break
		}
		f.Pos = Point{r.X + 5, r.Y + 2}
	}
	if !pulled {
		t.Errorf("rank 4 never pulled the player")
	}
}
