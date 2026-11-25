package rogue

import (
	"math"
	"testing"
)

// wear puts on a new bracelet id, taking off the one worn.
func wear(g *Game, id string) *Item {
	if b := g.equipped(CategoryBracelet); b != nil {
		b.Equipped = false
	}
	it := carry(g, id, 0)
	it.Equipped = true
	return it
}

// TestWardBracelets: the bracelets of wards keep the curses, blindness,
// the seal of the mouth and weakening off.
func TestWardBracelets(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Player = Point{r.X, r.Y}
	g.HP, g.MaxHP = 999, 999
	near := place(g, kind("sennin1"), g.Player.Add(Point{1, 0}))
	weaken := Ability{Name: AbilityAfflict, Values: []int{100, 0, afflictWeaken, 7}}

	food := carry(g, "onigiri", 0)
	wear(g, "noroiyoke")
	if g.curse(food) || food.Cursed {
		t.Error("cursed with the bracelet")
	}
	wear(g, "metsubushiyoke")
	if g.afflict(MagicBlind); g.Blind != 0 {
		t.Error("blind with the bracelet")
	}
	wear(g, "kuchifuujiyoke")
	if g.afflict(MagicSeal); g.Sealed {
		t.Error("sealed with the bracelet")
	}
	wear(g, "yowariyoke")
	if g.afflictBy(near, weaken); g.AttackDown+g.DefenseDown != 0 {
		t.Error("weakened with the bracelet")
	}
	// And without.
	wear(g, "kehai")
	if !g.curse(food) {
		t.Error("not cursed without")
	}
	if g.afflict(MagicSeal); !g.Sealed {
		t.Error("not sealed without")
	}
	if g.afflictBy(near, weaken); g.AttackDown+g.DefenseDown == 0 {
		t.Error("not weakened without")
	}
}

// TestHungryBracelet: twice as hungry, blows 20% stronger.
func TestHungryBracelet(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Player = Point{r.X, r.Y}
	g.HP, g.MaxHP = 999, 999
	m := place(g, kind("nezumi1"), g.Player.Add(Point{1, 0}))
	hit := func() int {
		m.HP, m.MaxHP, m.Defense = 9999, 9999, 0
		total := 0
		for range 300 {
			before := m.HP
			g.strike(m, 1)
			total += before - m.HP
		}
		return total
	}
	plain := hit()
	wear(g, "haraheri")
	if strong := hit(); float64(strong) < float64(plain)*1.1 {
		t.Errorf("blows %d with the bracelet, %d without", strong, plain)
	}
	g.Fullness, g.hunger = 100, 0
	for range 10 * g.Dungeon.Rules.HungerTurns {
		g.getHungry()
	}
	if g.Fullness != 80 {
		t.Errorf("fullness %d after 10 hunger turns' time", g.Fullness)
	}
}

// TestDropBracelet: the monsters drop twice as often.
func TestDropBracelet(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	if g.dropRate(30) != 30 {
		t.Error("without the bracelet")
	}
	wear(g, "taranagashi")
	if g.dropRate(30) != 60 || g.dropRate(80) != 100 || g.dropRate(0) != 0 {
		t.Errorf("rates %d %d %d", g.dropRate(30), g.dropRate(80), g.dropRate(0))
	}
}

// TestSealBracelets: a bracelet of a seal has its ability at the
// strength the items are born with.
func TestSealBracelets(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	for _, c := range []struct {
		id string
		e  SealEffect
		n  float64
	}{
		{"kaishinu", SealCrit, 0.25}, {"mikiriu", SealDodge, 0.15}, {"hangekiu", SealCounter, 0.30},
		{"mikawashiu", SealDodgeThrown, 0.5}, {"hicchuuu", SealSure, 1}, {"haraherazuu", SealNoHunger, 0.5},
	} {
		wear(g, c.id)
		if got := g.sealShare(c.e); math.Abs(got-c.n) > 1e-9 {
			t.Errorf("%s: %s %v, want %v", c.id, c.e, got, c.n)
		}
		if lines := g.info(g.equipped(CategoryBracelet)); len(lines) < 2 || lines[1] == "特に効果はない。" {
			t.Errorf("%s: info %q", c.id, lines)
		}
	}
}
