package rogue

import (
	"math"
	"slices"
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

// TestDropBracelet: the monsters drop twice as often; the player lets
// its items fall now and then, not the equipped ones.
func TestDropBracelet(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	if g.dropRate(30) != 30 {
		t.Error("without the bracelet")
	}
	wear(g, "taranagashi")
	if g.dropRate(30) != 60 || g.dropRate(80) != 100 || g.dropRate(0) != 0 {
		t.Errorf("rates %d %d %d", g.dropRate(30), g.dropRate(80), g.dropRate(0))
	}
	g.Inventory = slices.DeleteFunc(g.Inventory, func(it *Item) bool { return !it.Equipped })
	for range 5 {
		carry(g, "yakusou", 0)
	}
	n := 0
	for range 200 {
		before := len(g.Inventory)
		g.leak()
		n += before - len(g.Inventory)
	}
	if n != 5 || len(g.Inventory) != 1 || !g.Inventory[0].Equipped {
		t.Errorf("fell %d, left %d", n, len(g.Inventory))
	}
	if _, ok := g.ItemAt(g.Player); !ok {
		t.Error("nothing at the feet")
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

// TestTrapperBracelet: every trap shows, the player walks over them, and
// the monsters set them off.
func TestTrapperBracelet(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X, r.Y}
	hp := g.HP
	pit := trap(g, "otoshiana", Point{r.X + 1, r.Y})
	poison := trap(g, "dokuya", Point{r.X + 2, r.Y})
	// The monster may come either way.
	trap(g, "dokuya", Point{r.X + 2, r.Y + 1})
	wear(g, "wanashi")
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 1 || g.HP != hp || g.Player != pit.Pos {
		t.Fatalf("depth %d, HP %d, at %v: the pit went off", g.Depth, g.HP, g.Player)
	}
	if !pit.Found || !poison.Found {
		t.Error("traps hidden")
	}
	// A monster coming to the player over the poisoned arrows is hurt.
	m := place(g, kind("nezumi1"), Point{r.X + 3, r.Y})
	m.Dozing = false
	m.HP, m.MaxHP = 50, 50
	g.monstersAct()
	if m.Pos.X != r.X+2 || m.HP != 50-poison.Def.Strength {
		t.Errorf("monster at %v, HP %d: the arrows did not go off", m.Pos, m.HP)
	}
	// Without the bracelet, the monsters do not.
	g.equipped(CategoryBracelet).Equipped = false
	m.HP, m.Pos = 50, Point{r.X + 3, r.Y}
	g.monstersAct()
	if m.Pos.X != r.X+2 || m.HP < 50 {
		t.Errorf("without the bracelet: at %v, HP %d", m.Pos, m.HP)
	}
}

// TestSmallStomachBracelet: the blows of the monsters do 20% less (not
// other damage), 25 less fullness held.
func TestSmallStomachBracelet(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	g.Fullness = 100
	wear(g, "ishukushou")
	if n := g.cushioned(10); n != 8 {
		t.Errorf("a blow of 10: %d, want 8", n)
	}
	if g.cushioned(1) != 1 {
		t.Error("a blow of 1 made 0")
	}
	if g.weakened(10) != 10 {
		t.Error("other damage lessened")
	}
	g.getHungry()
	if g.Fullness != 75 {
		t.Errorf("fullness %d, want 75", g.Fullness)
	}
	g.fill(50)
	if g.Fullness != 75 || g.fullnessLabel() != "75%（最大 75%）" {
		t.Errorf("fullness %d, %q", g.Fullness, g.fullnessLabel())
	}
}

// TestCritTakenBracelet: half the blows on the player are critical, for
// 150%; a shield of 防恨 takes 2/3 off, its seal 1/2.
func TestCritTakenBracelet(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	m := place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	count := func() (crit int) {
		for range 400 {
			if n := g.criticalOn(m, 30); n != 30 {
				crit++
			}
		}
		return crit
	}
	if count() != 0 {
		t.Error("critical blows without the bracelet")
	}
	wear(g, "tsuukon")
	if c := count(); c < 160 || c > 240 {
		t.Errorf("%d critical in 400", c)
	}
	n := 0
	for n != 45 {
		n = g.criticalOn(m, 30)
	}
	shield := carry(g, "bokon", 0)
	shield.Equipped = true
	for range 100 {
		if n := g.criticalOn(m, 30); n != 30 && n != 15 {
			t.Fatalf("damage %d with the shield, want 30 or 15", n)
		}
	}
	shield.Equipped = false
	plain := carry(g, "kitate", 0)
	plain.Seals = []string{"恨"}
	plain.Equipped = true
	for range 100 {
		if n := g.criticalOn(m, 30); n != 30 && n != 23 {
			t.Fatalf("damage %d with the seal, want 30 or 23", n)
		}
	}
}

// TestCallBracelet: the monsters of the floor wake and come.
func TestCallBracelet(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X, r.Y}
	m := place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 2})
	m.Dozing = true
	wear(g, "mamonoyobi")
	g.call()
	if m.Dozing {
		t.Fatal("still dozing")
	}
	before := chebyshev(m.Pos, g.Player)
	g.monstersAct()
	if chebyshev(m.Pos, g.Player) >= before {
		t.Errorf("did not come: %v", m.Pos)
	}
}

// TestLeapBracelet: the player leaps away now and then.
func TestLeapBracelet(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X, r.Y}
	wear(g, "takatobiu")
	n := 0
	for range 1000 {
		at := g.Player
		if g.leap() {
			n++
			if g.Player == at {
				t.Log("leapt in place")
			}
		}
	}
	if n < 60 || n > 140 {
		t.Errorf("%d leaps in 1000 turns", n)
	}
}

// TestBlastBracelet: HP halves, the enemies around are gone, an ally's HP
// halves, the walls around crumble.
func TestBlastBracelet(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X, r.Y}
	g.HP, g.MaxHP = 100, 100
	enemy := place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	ally := place(g, kind("nezumi1"), Point{r.X, r.Y + 1})
	ally.Ally = true
	ally.HP, ally.MaxHP = 20, 20
	far := place(g, kind("nezumi1"), Point{r.X + 3, r.Y})
	near := carry(g, "yakusou", 0)
	g.drop(slices.Index(g.Inventory, near))
	kept := carry(g, "yakusou", 0)
	kept.Pos = Point{r.X + 3, r.Y + 1}
	g.Inventory = slices.DeleteFunc(g.Inventory, func(it *Item) bool { return it == kept })
	g.Items = append(g.Items, kept)
	wall := Point{r.X - 1, r.Y - 1}
	if g.Level.At(wall) != TileRock {
		t.Fatalf("no wall at %v", wall)
	}
	wear(g, "bakuhatsu")
	for !g.blast() {
	}
	if g.HP != 50 {
		t.Errorf("HP %d, want 50", g.HP)
	}
	if slices.Contains(g.Monsters, enemy) || !slices.Contains(g.Monsters, far) {
		t.Error("the enemy next to it stays, or the far one is gone")
	}
	if !slices.Contains(g.Monsters, ally) || ally.HP != 10 {
		t.Errorf("ally HP %d", ally.HP)
	}
	if g.Level.At(wall) != TileCorridor {
		t.Error("the wall stands")
	}
	if slices.Contains(g.Items, near) || !slices.Contains(g.Items, kept) {
		t.Error("the item under the player stays, or the far one is gone")
	}
}

// TestClumsyBracelet: a thrown item misses and falls at the feet of the
// monster, setting the trap there off on it.
func TestClumsyBracelet(t *testing.T) {
	g, r := trapGame(t)
	g.Player = Point{r.X, r.Y}
	g.Facing = "right"
	m := place(g, kind("nezumi1"), Point{r.X + 3, r.Y})
	m.HP, m.MaxHP = 50, 50
	poison := trap(g, "dokuya", m.Pos)
	wear(g, "hetanage")
	stone := carry(g, "yakusou", 0)
	g.throwItem(stone)
	if m.HP != 50-poison.Def.Strength || !poison.Found {
		t.Errorf("HP %d: the trap did not go off", m.HP)
	}
	if !slices.Contains(g.Items, stone) || stone.Pos != m.Pos {
		t.Errorf("the item at %v, the monster at %v", stone.Pos, m.Pos)
	}
}
