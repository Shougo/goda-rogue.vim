package rogue

import (
	"strings"
	"testing"
)

// withSealStrength returns a copy of d with the seal glyph on c born at
// the strength n (its seals copied, the others' left alone).
func withSealStrength(d *Dungeon, glyph string, c Category, n int) *Dungeon {
	cd := *d
	cd.Seals = nil
	for _, s := range d.Seals {
		if s.Glyph == glyph && (s.For == "" || s.For == c) {
			cs := *s
			cs.Strength = n
			s = &cs
		}
		cd.Seals = append(cd.Seals, s)
	}
	return &cd
}

// gearGame returns a game with the weapon or shield id equipped, every
// blow of the player hitting, and its innate seal at 100%.
func gearGame(t *testing.T, id string) (*Game, Room, *Item) {
	t.Helper()
	g, r := roomGame(t, 6, 5)
	g.Player = Point{r.X + 2, r.Y + 2}
	g.Facing = "right"
	it := carry(g, id, 0)
	it.Equipped = true
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate = 100 })
	g.Dungeon = withSealStrength(g.Dungeon, it.Def.Seal, it.Def.Category, 100)
	return g, r, it
}

// TestSealBlowState: a weapon of 睡眠 puts the monster it hits ahead to
// sleep.
func TestSealBlowState(t *testing.T) {
	g, _, _ := gearGame(t, "nemuriken")
	m := place(g, kind("nezumi4"), g.Player.Add(Point{1, 0}))
	m.HP, m.MaxHP = 9999, 9999
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if m.Asleep == 0 {
		t.Errorf("not asleep after a blow: %q", g.Message)
	}
}

// TestSealDouble: 連撃 strikes the one ahead a second time.
func TestSealDouble(t *testing.T) {
	g, _, _ := gearGame(t, "nitou")
	m := place(g, kind("nezumi4"), g.Player.Add(Point{1, 0}))
	m.HP, m.MaxHP = 9999, 9999
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Message, "連続攻撃！") || strings.Count(g.Message, "のダメージ。") != 2 {
		t.Errorf("one blow only: %q", g.Message)
	}
}

// TestSealDig: つるはし digs the rock ahead, not the edge of the floor.
func TestSealDig(t *testing.T) {
	g, r, _ := gearGame(t, "tsuruhashi")
	g.Player, g.Facing = Point{r.X, r.Y}, "up"
	p := Point{r.X, r.Y - 1}
	if g.Level.At(p) != TileRock || p.Y <= 0 {
		t.Skipf("no rock to dig at %v", p)
	}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if g.Level.At(p) != TileCorridor {
		t.Errorf("tile %v not dug: %q", g.Level.At(p), g.Message)
	}
}

// TestSealGrow: 成長 raises the weapon going down, up to +20.
func TestSealGrow(t *testing.T) {
	g, _, it := gearGame(t, "seichouken")
	g.grow()
	if it.Upgrade != 1 {
		t.Errorf("upgrade %d, want 1", it.Upgrade)
	}
	it.Upgrade = growCap
	g.grow()
	if it.Upgrade != growCap {
		t.Errorf("upgrade %d past %d", it.Upgrade, growCap)
	}
}

// TestSealWards: the wards of shields keep sleep and a curse off.
func TestSealWards(t *testing.T) {
	g, _, _ := gearGame(t, "nemurazu")
	g.afflict(MagicSleep)
	if g.Asleep > 0 {
		t.Error("asleep with 睡眠よけ")
	}
	g, _, _ = gearGame(t, "harai")
	other := carry(g, "katana", 0)
	if g.curse(other) || other.Cursed {
		t.Error("cursed with 呪いよけ")
	}
}

// TestSealFine: 絶好調 takes less damage with the HP full only.
func TestSealFine(t *testing.T) {
	g, _, _ := gearGame(t, "zekkouchou")
	g.Dungeon = withSealStrength(g.Dungeon, "絶", CategoryShield, 50)
	g.HP = g.MaxHP
	if n := g.weakened(10); n != 5 {
		t.Errorf("full HP: %d, want 5", n)
	}
	g.HP = g.MaxHP - 1
	if n := g.weakened(10); n != 10 {
		t.Errorf("not full: %d, want 10", n)
	}
}

// TestSealNoSeal: an ability of 印の強さ 0 (掘る) is not passed on by a
// synthesis.
func TestSealNoSeal(t *testing.T) {
	g, _, _ := gearGame(t, "nemuriken")
	base, pick := carry(g, "katana", 0), carry(g, "tsuruhashi", 0)
	if !g.merge(base, pick, false) {
		t.Fatal("not merged")
	}
	if len(base.Seals) != 0 {
		t.Errorf("seals %v, want none", base.Seals)
	}
}
