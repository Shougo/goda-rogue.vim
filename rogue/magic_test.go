package rogue

import (
	"strings"
	"testing"
)

// spell is the ability of a mage that waves one effect at every chance.
func spell(e MagicEffect) Ability {
	return Ability{Name: AbilityMagic, Values: []int{10, 100, 0}, Effects: []MagicEffect{e}}
}

// TestCastAt: a mage on the player's line waves its staff; the bolt takes
// the first creature on the way.
func TestCastAt(t *testing.T) {
	g, r := roomGame(t, 12, 1)
	m := place(g, kind("mahou1"), Point{r.X + 5, r.Y})
	if !g.castAt(m, spell(MagicConfuse)) || g.Confused != confuseTurns {
		t.Errorf("confused %d, want %d", g.Confused, confuseTurns)
	}

	g, r = roomGame(t, 12, 1)
	m = place(g, kind("mahou1"), Point{r.X + 5, r.Y})
	f := place(g, kind("kaeru1"), Point{r.X + 2, r.Y})
	g.castAt(m, spell(MagicSeal))
	if g.Sealed || !f.Sealed {
		t.Errorf("player sealed %v, the frog %v: want the frog", g.Sealed, f.Sealed)
	}
	if _, ok := f.ability(AbilityPull); ok {
		t.Error("a sealed frog still pulls")
	}

	// Out of its line, it walks.
	g, r = roomGame(t, 12, 3)
	m = place(g, kind("mahou1"), Point{r.X + 4, r.Y + 2})
	if g.castAt(m, spell(MagicConfuse)) {
		t.Error("it waved off its line")
	}
}

// TestMagicOnPlayer: what each bolt does to the player.
func TestMagicOnPlayer(t *testing.T) {
	g, r := roomGame(t, 12, 3)
	m := place(g, kind("mahou1"), Point{r.X + 5, r.Y})
	from := g.Player
	g.magicOnPlayer(MagicWarp, Point{-1, 0}, m)
	if g.Player == from {
		t.Error("no warp")
	}

	g, _ = roomGame(t, 12, 3)
	g.HP = 21
	g.magicOnPlayer(MagicHalf, Point{-1, 0}, m)
	if g.HP != 10 {
		t.Errorf("HP %d, want 10", g.HP)
	}
	g.magicOnPlayer(MagicQuarter, Point{-1, 0}, m)
	g.magicOnPlayer(MagicQuarter, Point{-1, 0}, m)
	if g.HP != 1 {
		t.Errorf("HP %d, want 1 (not below)", g.HP)
	}

	g, _ = roomGame(t, 12, 3)
	growth := g.Dungeon.Growth[g.Dungeon.Rules.Growth]
	base := g.MaxHP
	for i := range 4 {
		g.MaxHP += growth[i].HP
	}
	g.PlayerLevel, g.EXP, g.HP = 5, growth[3].EXP+3, g.MaxHP
	g.magicOnPlayer(MagicLevel3, Point{-1, 0}, m)
	if g.PlayerLevel != 2 || g.EXP != growth[0].EXP || g.MaxHP != base+growth[0].HP || g.HP != g.MaxHP {
		t.Errorf("level %d, EXP %d, HP %d/%d: want 2, %d, %d", g.PlayerLevel, g.EXP, g.HP, g.MaxHP, growth[0].EXP, base+growth[0].HP)
	}
	g.magicOnPlayer(MagicLevel3, Point{-1, 0}, m)
	if g.PlayerLevel != 1 || g.EXP != 0 || g.MaxHP != base {
		t.Errorf("level %d, EXP %d, max HP %d: want 1, 0, %d", g.PlayerLevel, g.EXP, g.MaxHP, base)
	}
}

// TestPlayerSpeed: fast, the player acts twice a turn; slow, an action
// takes two turns; asleep, the turns pass by themselves.
func TestPlayerSpeed(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	g.Hasted = hasteTurns
	turn := g.Turn
	wait(t, g, 4)
	if g.Turn != turn+2 {
		t.Errorf("fast: %d turns in 4 actions, want 2", g.Turn-turn)
	}

	g, _ = roomGame(t, 6, 3)
	g.Slowed = slowTurns
	turn = g.Turn
	wait(t, g, 2)
	if g.Turn != turn+4 || !strings.Contains(g.Status(), "鈍足") {
		t.Errorf("slow: %d turns in 2 actions, want 4 (%q)", g.Turn-turn, g.Status())
	}

	g, _ = roomGame(t, 6, 3)
	g.Asleep = sleepTurns
	turn = g.Turn
	wait(t, g, 1)
	if g.Turn != turn+1+sleepTurns || g.Asleep != 0 || !strings.Contains(g.Message, "目が覚めた") {
		t.Errorf("asleep: %d turns, asleep %d, %q: want %d", g.Turn-turn, g.Asleep, g.Message, 1+sleepTurns)
	}
}

// TestPlayerConfused: confused, the player moves and attacks at random,
// and walking into a wall costs the turn.
func TestPlayerConfused(t *testing.T) {
	g, r := roomGame(t, 8, 5)
	g.Player = Point{r.X + 3, r.Y + 2}
	g.Confused = 100
	other := false
	for range 20 {
		turn := g.Turn
		if err := g.Do("move", []string{"right"}); err != nil {
			t.Fatal(err)
		}
		if g.Turn != turn+1 {
			t.Fatalf("a confused step took %d turns", g.Turn-turn)
		}
		if g.Facing != "right" {
			other = true
		}
		g.Player = Point{r.X + 3, r.Y + 2}
	}
	if !other {
		t.Error("confused, it always went right")
	}
}

// TestMouthSealed: a sealed mouth keeps the player from eating, drinking,
// reading and talking, not from waving; until the next floor.
func TestMouthSealed(t *testing.T) {
	g, _ := roomGame(t, 6, 3)
	g.Sealed = true
	for _, c := range []string{commandEat, commandDrink, commandRead} {
		if !g.mouthBlocks(c) {
			t.Errorf("%s with a sealed mouth", c)
		}
	}
	if g.mouthBlocks(commandWave) {
		t.Error("a sealed mouth keeps from waving")
	}
	if !strings.Contains(g.Status(), "口封じ") {
		t.Errorf("status %q", g.Status())
	}
	g.descend()
	if g.Sealed {
		t.Error("still sealed on the next floor")
	}
}

// TestMonsterStates: an asleep monster does nothing, a slowed one acts
// every other turn, a hasted one is faster, one taken down ranks down.
func TestMonsterStates(t *testing.T) {
	g, r := roomGame(t, 10, 1)
	m := place(g, kind("nezumi1"), Point{r.X + 8, r.Y})
	m.Asleep = 3
	wait(t, g, 3)
	if m.Pos != (Point{r.X + 8, r.Y}) || m.Asleep != 0 {
		t.Errorf("asleep, it moved to %v (asleep %d)", m.Pos, m.Asleep)
	}
	m.Slowed = 4
	wait(t, g, 4)
	if d := r.X + 8 - m.Pos.X; d != 2 {
		t.Errorf("slowed, %d steps in 4 turns, want 2", d)
	}

	g, r = roomGame(t, 10, 1)
	m = place(g, kind("nezumi1"), Point{r.X + 5, r.Y})
	g.magicOn(m, MagicHaste, Point{-1, 0})
	if m.Speed != SpeedDouble {
		t.Errorf("speed %v, want double", m.Speed)
	}
	m = place(g, kind("nezumi4"), Point{r.X + 7, r.Y})
	g.magicOn(m, MagicLevel3, Point{-1, 0})
	if m.Def.ID != "nezumi1" {
		t.Errorf("%s, want nezumi1", m.Def.ID)
	}
	g.magicOn(m, MagicLevel1, Point{-1, 0})
	if m.Def.ID != "nezumi1" {
		t.Errorf("%s, want nezumi1 (no rank below)", m.Def.ID)
	}
}

func TestParseMagic(t *testing.T) {
	as, err := parseAbilities("魔法:10:50:ワープ,睡眠")
	if err != nil || len(as) != 1 || len(as[0].Effects) != 2 || as[0].Effects[1] != MagicSleep {
		t.Fatalf("got %v, %v", as, err)
	}
	if _, err := parseAbilities("魔法:10:50:ワープ,石化"); err == nil || !strings.Contains(err.Error(), `unknown effect "石化"`) {
		t.Errorf("error %v", err)
	}
}

// TestMagicOnShopkeeper: the bolt of a monster does not take a calm
// shopkeeper out of its shop.
func TestMagicOnShopkeeper(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	m := place(g, kind("oni1"), Point{r.X + 3, r.Y + 1})
	m.Keeper = true
	at := m.Pos
	for _, e := range []MagicEffect{MagicWarp, MagicBlow} {
		g.magicOn(m, e, Point{1, 0})
		if m.Pos != at {
			t.Fatalf("%s moved it to %v", e, m.Pos)
		}
	}
	m.Angry = true
	g.magicOn(m, MagicBlow, Point{1, 0})
	if m.Pos == at {
		t.Error("an angry one is not blown away")
	}
}
