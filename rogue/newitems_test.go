package rogue

import (
	"slices"
	"testing"
)

// The grasses, staffs and pots of 2026-11-28.

// TestHappyGrass: しあわせ草 raises the player a level, and ranks up the
// monster it is thrown at.
func TestHappyGrass(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	level, hp := g.PlayerLevel, g.MaxHP
	use(t, g, "shiawasesou")
	if g.PlayerLevel != level+1 || g.MaxHP <= hp {
		t.Errorf("level %d (was %d), max HP %d (was %d)", g.PlayerLevel, level, g.MaxHP, hp)
	}
	g.Facing = "right"
	m := place(g, kind("nezumi1"), Point{r.X + 2, r.Y})
	command(t, g, carry(g, "shiawasesou", 0), commandThrow)
	if m.Def.ID != "nezumi2" {
		t.Errorf("thrown: %s", m.Def.ID)
	}
}

// TestPoisonGrass: 毒草 takes 5 HP and a point of strength of who drinks
// it; thrown, 5 HP and half the attack of the monster, which is slowed.
func TestPoisonGrass(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.HP, g.MaxHP = 50, 50
	str := g.Strength
	use(t, g, "dokusou")
	if g.HP != 45 || g.Strength != str-1 {
		t.Errorf("HP %d, strength %d (was %d)", g.HP, g.Strength, str)
	}
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP, m.MaxHP = 100, 100
	attack := m.Attack
	command(t, g, carry(g, "dokusou", 0), commandThrow)
	if m.HP != 95 || m.Attack != max(attack/2, 1) || m.Slowed == 0 {
		t.Errorf("thrown: HP %d, attack %d (was %d), slowed %d", m.HP, m.Attack, attack, m.Slowed)
	}
}

// TestUnhurtGrass: 無敵草 keeps the player from harm for its turns; the
// monster it is thrown at too.
func TestUnhurtGrass(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.HP, g.MaxHP = 50, 50
	use(t, g, "mutekisou")
	if g.Unhurt < unhurtTurns-1 {
		t.Fatalf("unhurt %d", g.Unhurt)
	}
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	if g.playerHurt(30, m) || g.trapDamage(30, "x") || g.HP != 50 {
		t.Errorf("unhurt, HP %d", g.HP)
	}
	g.Unhurt = 0
	g.Facing = "right"
	m.HP, m.MaxHP = 100, 100
	command(t, g, carry(g, "mutekisou", 0), commandThrow)
	g.hurt(m, 30)
	if m.Unhurt == 0 || m.HP != 100 {
		t.Errorf("thrown: unhurt %d, HP %d", m.Unhurt, m.HP)
	}
}

// TestStomachSeeds: the seeds of 胃拡張 and 胃縮小 move the max fullness
// by 5, within 50 and 200.
func TestStomachSeeds(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	g.MaxFullness, g.Fullness = 100, 100
	use(t, g, "ikakuchoutane")
	if g.MaxFullness != 105 {
		t.Errorf("max fullness %d", g.MaxFullness)
	}
	use(t, g, "ishukushoutane")
	use(t, g, "ishukushoutane")
	if g.MaxFullness != 95 || g.Fullness != 95 {
		t.Errorf("max fullness %d, fullness %d", g.MaxFullness, g.Fullness)
	}
	g.MaxFullness = 52
	use(t, g, "ishukushoutane")
	g.MaxFullness = 198
	use(t, g, "ikakuchoutane")
	if g.MaxFullness != 200 {
		t.Errorf("max fullness %d", g.MaxFullness)
	}
}

// TestReviveGrass: 復活の草 carried brings the player back once with full
// HP; not in a pot, nor against the wind.
func TestReviveGrass(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	g.HP, g.MaxHP = 10, 50
	grass := carry(g, "fukkatsu", 0)
	delete(g.identified, "fukkatsu")
	if g.trapDamage(30, "x") || g.Over != "" || g.HP != 50 || slices.Contains(g.Inventory, grass) || !g.identified["fukkatsu"] {
		t.Fatalf("revived: over %q, HP %d, inventory %v", g.Over, g.HP, g.Inventory)
	}
	// In a pot, it does not.
	p := pot(g, "hozon", 2)
	g.putIn(p, carry(g, "fukkatsu", 0))
	if !g.trapDamage(99, "x") || g.Over == "" {
		t.Errorf("revived from a pot: over %q", g.Over)
	}
	// Nor against the wind.
	g, _ = roomGame(t, 4, 3)
	carry(g, "fukkatsu", 0)
	g.blown = true
	g.end("風に吹き飛ばされた", false)
	if g.Over == "" {
		t.Error("revived against the wind")
	}
}

// TestFireGrass: 火炎草 drunk burns the first monster to the front for
// 70 to 80; thrown, 35 to 45; thrown at the player, it burns it and
// grills its onigiri.
func TestFireGrass(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 3, r.Y})
	m.HP, m.MaxHP = 999, 999
	use(t, g, "kaensou")
	if n := 999 - m.HP; n < 70 || n > 80 {
		t.Errorf("drunk: %d", n)
	}
	m.HP, m.Pos = 999, Point{r.X + 3, r.Y}
	command(t, g, carry(g, "kaensou", 0), commandThrow)
	if n := 999 - m.HP; n < 35 || n > 45 {
		t.Errorf("thrown: %d: %s", n, g.Message)
	}
	g.HP, g.MaxHP = 100, 100
	food := carry(g, "onigiri", 0)
	g.hitPlayerWith(&Item{Def: g.Dungeon.Item("kaensou")}, m)
	if n := 100 - g.HP; n < 35 || n > 45 || food.Def.Effect != EffectGrilled {
		t.Errorf("at the player: %d, %s", n, food.Def.Name)
	}
}

// TestBindStaff: the staff of かなしばり binds the monster its bolt hits;
// the player bound waits for 20 turns, or until it loses HP.
func TestBindStaff(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 3, r.Y})
	m.HP, m.MaxHP = 999, 999
	wave(t, g, "kanashibarizue")
	if m.Bound == 0 {
		t.Fatal("not bound")
	}
	g.hurt(m, 1)
	if m.Bound != 0 {
		t.Error("a blow did not free it")
	}
	g.Monsters = nil
	g.HP, g.MaxHP = 50, 50
	g.magicOnPlayer(MagicBind, Point{1, 0}, nil)
	turn := g.Turn
	wait(t, g, 1)
	if g.Bound != 0 || g.Turn-turn < boundTurns {
		t.Errorf("bound %d, turns %d", g.Bound, g.Turn-turn)
	}
}

// TestTunnelStaff: the staff of トンネル digs the rock to the front into
// a corridor.
func TestTunnelStaff(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	g.Facing = "left"
	wall := Point{r.X - 1, r.Y}
	if g.Level.At(wall) != TileRock {
		t.Skip("no rock to the left")
	}
	wave(t, g, "tonneru")
	dug := 0
	for p := wall; dug < 10 && g.Level.At(p) == TileCorridor; p = p.Add(Point{-1, 0}) {
		dug++
	}
	if dug == 0 {
		t.Errorf("dug nothing: %v", g.Message)
	}
}

// TestChangingPots: a pot of 識別 identifies what is put in it, 強化 and
// 弱化 move the upgrade value of a weapon, 換金 makes money of it, 変化
// another item.
func TestChangingPots(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "shikibetsutsubo", 3)
	grass := carry(g, "yakusou", 0)
	delete(g.identified, "yakusou")
	g.putIn(p, grass)
	if !g.identified["yakusou"] || p.Contents[0] != grass {
		t.Errorf("identify: %v", p.Contents)
	}
	p = pot(g, "kyoukatsubo", 3)
	sword := carry(g, "tetsuken", 2)
	g.putIn(p, sword)
	p = pot(g, "jakkatsubo", 3)
	shield := carry(g, "kitate", 2)
	g.putIn(p, shield)
	if sword.Upgrade != 3 || shield.Upgrade != 1 {
		t.Errorf("upgrade %d, %d", sword.Upgrade, shield.Upgrade)
	}
	p = pot(g, "kankin", 3)
	grass = carry(g, "otogiri", 0)
	g.putIn(p, grass)
	if c := p.Contents[0]; c.Def.Category != CategoryMoney || c.Amount != sellPrice(grass) {
		t.Errorf("cash: %s %d", c.Def.Name, c.Amount)
	}
	p = pot(g, "henka", 3)
	g.putIn(p, carry(g, "yakusou", 0))
	if len(p.Contents) != 1 || p.Contents[0].Def.ID == "henka" || p.Contents[0].Def.Category == CategoryMoney {
		t.Errorf("change: %v", p.Contents)
	}
}
