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
	m.HP, m.MaxHP, m.Asleep = 9999, 9999, 99
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Message, "連続攻撃！") || strings.Count(g.Message, m.Name+"に") != 2 {
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

// TestSealSlay: 竜殺し does more to a ドラゴン than to another.
func TestSealSlay(t *testing.T) {
	g, _, _ := gearGame(t, "ryuugoroshi")
	m := place(g, kind("ryu1"), g.Player.Add(Point{1, 0}))
	o := place(g, kind("nezumi1"), g.Player.Add(Point{0, 1}))
	if s := g.slayShare(m); s != 1 {
		t.Errorf("share on 竜 %v, want 1 (the test's 100%%)", s)
	}
	if s := g.slayShare(o); s != 0 {
		t.Errorf("share on 鼠 %v, want 0", s)
	}
}

// TestSealReach: 長槍 strikes the enemy two tiles ahead, not past one
// next to the player.
func TestSealReach(t *testing.T) {
	g, _, _ := gearGame(t, "nagayari")
	m := place(g, kind("nezumi4"), g.Player.Add(Point{2, 0}))
	m.HP, m.MaxHP, m.Asleep = 9999, 9999, 99
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if m.HP == 9999 {
		t.Errorf("not struck two tiles ahead: %q", g.Message)
	}
	g.Dungeon = withSealStrength(g.Dungeon, "槍", CategoryWeapon, 0)
	hp := m.HP
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if m.HP != hp {
		t.Error("struck two tiles ahead without the seal")
	}
}

// TestBreath: 若龍 breathes fire along its line, not next to the player,
// and 龍鱗 halves it.
func TestBreath(t *testing.T) {
	g, _, _ := gearGame(t, "ryuurin")
	g.Dungeon = withSealStrength(g.Dungeon, "鱗", CategoryShield, 50)
	m := place(g, kind("ryuu1"), g.Player.Add(Point{3, 0}))
	a, _ := m.ability(AbilityBreath)
	a.Values = []int{0, 100, 20}
	g.HP, g.MaxHP = 100, 100
	if !g.breathe(m, a) {
		t.Fatalf("no breath on its line: %q", g.Message)
	}
	if g.HP != 90 {
		t.Errorf("HP %d, want 90 (20 halved)", g.HP)
	}
	m.Pos = g.Player.Add(Point{1, 0})
	if g.breathe(m, a) {
		t.Error("a breath next to the player")
	}
	m.Pos = g.Player.Add(Point{3, 1})
	if g.breathe(m, a) {
		t.Error("a breath off its line")
	}
}

// TestSealNoSealing: 封印よけ keeps the mouth open, against the trap of
// 封印 too.
func TestSealNoSealing(t *testing.T) {
	g, _, _ := gearGame(t, "fuuinyoke")
	g.afflict(MagicSeal)
	if g.Sealed {
		t.Error("the mouth sealed with 封印よけ")
	}
	g, _, _ = gearGame(t, "katana")
	tr := &Trap{Def: g.Dungeon.Trap("fuuinwana"), Pos: g.Player}
	g.Traps = append(g.Traps, tr)
	g.spring(tr, true)
	if !g.Sealed {
		t.Errorf("the trap of 封印 did nothing: %q", g.Message)
	}
}

// TestAfflictMute: a monster of 状態 W 4 seals the mouth of the player,
// unless 封印よけ wards it off.
func TestAfflictMute(t *testing.T) {
	a := Ability{Name: AbilityAfflict, Values: []int{100, 0, afflictMute, 1}}
	g, _, _ := gearGame(t, "katana")
	m := place(g, kind("nezumi1"), g.Player.Add(Point{1, 0}))
	if !g.afflictBy(m, a) || !g.Sealed {
		t.Errorf("not sealed: %q", g.Message)
	}
	g, _, _ = gearGame(t, "fuuinyoke")
	m = place(g, kind("nezumi1"), g.Player.Add(Point{1, 0}))
	if g.afflictBy(m, a); g.Sealed {
		t.Error("sealed with 封印よけ")
	}
}

// TestPoisonBlow: a blow of 蛇 that hits poisons (the strength down), not
// with 毒よけ.
func TestPoisonBlow(t *testing.T) {
	for _, guard := range []bool{false, true} {
		id := "katana"
		if guard {
			id = "dokuyoke"
		}
		g, _, _ := gearGame(t, id)
		g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
		m := place(g, kind("hebi4"), g.Player.Add(Point{1, 0}))
		g.HP, g.MaxHP, g.Strength = 999, 999, 8
		poisoned := false
		for range 40 {
			g.hitPlayer(m)
			if g.Strength < 8 {
				poisoned = true
				break
			}
		}
		if poisoned == guard {
			t.Errorf("guard %v: strength %d", guard, g.Strength)
		}
	}
}

// TestRelic: a relic is stronger than its kind, shows it in its name and
// costs more; a gold one has seals in more slots.
func TestRelic(t *testing.T) {
	g, _ := roomGame(t, 6, 5)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.Relic = 1 })
	var blue, gold *Item
	for i := 0; i < 400 && (blue == nil || gold == nil); i++ {
		it := g.newItemOf(g.rng, g.Dungeon.Item("katana"), g.Player)
		switch it.Relic {
		case RelicBlue:
			blue = it
		case RelicGold:
			gold = it
		}
	}
	if blue == nil || gold == nil {
		t.Fatal("no relics")
	}
	if b := blue.RelicBonus; b < 5 || b > 10 {
		t.Errorf("blue bonus %d", b)
	}
	if blue.Strength() != blue.Def.Strength+blue.RelicBonus+blue.Upgrade {
		t.Errorf("strength %d", blue.Strength())
	}
	if gold.slots() <= gold.Def.Slots || len(gold.Seals) == 0 {
		t.Errorf("gold slots %d, seals %v", gold.slots(), gold.Seals)
	}
	if price(blue) != 2*blue.Def.Price || !strings.Contains(g.Label(blue), "（青神器）") {
		t.Errorf("price %d, label %q", price(blue), g.Label(blue))
	}
}
