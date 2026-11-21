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
