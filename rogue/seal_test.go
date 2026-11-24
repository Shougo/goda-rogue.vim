package rogue

import (
	"strings"
	"testing"
)

// sealGame returns a game with a sword and a shield equipped, with the
// seals given, every blow of the player hitting.
func sealGame(t *testing.T, weapon, shield []string) (*Game, Room) {
	t.Helper()
	g, r := roomGame(t, 6, 5)
	g.Player = Point{r.X + 2, r.Y + 2}
	g.Facing = "right"
	sword, board := carry(g, "katana", 0), carry(g, "tetsutate", 0)
	sword.Equipped, board.Equipped = true, true
	sword.Seals, board.Seals = weapon, shield
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate = 100 })
	return g, r
}

// TestSealThree: a blow strikes the tile ahead and the two beside it,
// not an ally there, nor the tiles at the sides.
func TestSealThree(t *testing.T) {
	g, _ := sealGame(t, []string{"三"}, nil)
	var ms []*Monster
	for _, d := range []Point{{1, 0}, {1, -1}, {1, 1}, {0, -1}} {
		m := place(g, kind("nezumi4"), g.Player.Add(d))
		m.HP, m.MaxHP, m.Asleep = 9999, 9999, 99
		ms = append(ms, m)
	}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if ms[0].HP == 9999 || ms[1].HP == 9999 || ms[2].HP == 9999 || ms[3].HP != 9999 {
		t.Fatalf("HP %d %d %d ahead, %d at the side", ms[0].HP, ms[1].HP, ms[2].HP, ms[3].HP)
	}
	ms[1].Ally = true
	hp := ms[1].HP
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if ms[1].HP != hp {
		t.Error("the blow struck an ally beside the enemy")
	}
	// Nothing ahead: the blow goes wide, the sides are left alone.
	g.Monsters = []*Monster{ms[2]}
	hp = ms[2].HP
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if ms[2].HP != hp || !strings.Contains(g.Message, "空振りした。") {
		t.Errorf("no enemy ahead: HP %d to %d, %q", hp, ms[2].HP, g.Message)
	}

	// Without the seal, the one ahead only.
	g, _ = sealGame(t, nil, nil)
	a := place(g, kind("nezumi4"), g.Player.Add(Point{1, 0}))
	b := place(g, kind("nezumi4"), g.Player.Add(Point{1, 1}))
	a.Asleep, b.Asleep = 99, 99
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if a.HP == a.MaxHP || b.HP != b.MaxHP {
		t.Errorf("without the seal: HP %d/%d ahead, %d/%d beside", a.HP, a.MaxHP, b.HP, b.MaxHP)
	}
}

// TestSealNoTheft: born with it (the shield of 盗みよけ), nothing is
// stolen; as a seal, half the time; without it, always.
func TestSealNoTheft(t *testing.T) {
	for _, c := range []struct {
		shield   string
		seals    []string
		min, max int
	}{{"nusumiyoke", nil, 0, 0}, {"tetsutate", []string{"盗"}, 60, 140}, {"tetsutate", nil, 200, 200}} {
		g, _ := sealGame(t, nil, c.seals)
		g.equipped(CategoryShield).Equipped = false
		carry(g, c.shield, 0).Equipped = true
		g.equipped(CategoryShield).Seals = c.seals
		stolen := 0
		for range 200 {
			g.Gold = 100
			m := place(g, kind("touzoku1"), g.Player.Add(Point{1, 0}))
			for _, a := range m.Def.Abilities {
				if a.Name == AbilitySteal {
					a.Values = append([]int{100}, a.Values[1:]...)
					if g.thieve(m, a) {
						stolen++
					}
				}
			}
			g.Monsters = nil
		}
		if stolen < c.min || stolen > c.max {
			t.Errorf("%s %v: stolen %d times of 200, want %d-%d", c.shield, c.seals, stolen, c.min, c.max)
		}
	}
}

// TestSealDodge: the blows of the monsters hit less often by the share of
// 見切り: 15% born with it, 8% as a seal, more with two seals.
func TestSealDodge(t *testing.T) {
	for _, c := range []struct {
		shield   string
		seals    []string
		min, max int
	}{{"mikiri", nil, 300, 380}, {"tetsutate", []string{"避"}, 340, 395}, {"tetsutate", []string{"避", "避"}, 320, 385},
		{"tetsutate", nil, 400, 400}} {
		g, _ := sealGame(t, nil, nil)
		g.equipped(CategoryShield).Equipped = false
		carry(g, c.shield, 0).Equipped = true
		g.equipped(CategoryShield).Seals = c.seals
		g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
		g.HP, g.MaxHP = 99999, 99999
		m := place(g, kind("nezumi1"), g.Player.Add(Point{1, 0}))
		hit := 0
		for range 400 {
			hp := g.HP
			g.hitPlayer(m)
			if g.HP < hp {
				hit++
			}
		}
		if hit < c.min || hit > c.max {
			t.Errorf("%s %v: hit %d times of 400, want %d-%d", c.shield, c.seals, hit, c.min, c.max)
		}
	}
}

// TestSealSure: 必中 born with it, every blow hits; as a seal, half the
// misses hit, three quarters with two.
func TestSealSure(t *testing.T) {
	for _, c := range []struct {
		sword    string
		seals    []string
		min, max int
	}{{"hicchuu", nil, 400, 400}, {"katana", []string{"必"}, 160, 240}, {"katana", []string{"必", "必"}, 260, 340},
		{"katana", nil, 0, 0}} {
		g, _ := sealGame(t, nil, nil)
		g.equipped(CategoryWeapon).Equipped = false
		carry(g, c.sword, 0).Equipped = true
		g.equipped(CategoryWeapon).Seals = c.seals
		g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate = 0 })
		m := place(g, kind("nezumi4"), g.Player.Add(Point{1, 0}))
		hit := 0
		for range 400 {
			m.HP, m.MaxHP = 9999, 9999
			g.strike(m, 1)
			if m.HP < 9999 {
				hit++
			}
		}
		if hit < c.min || hit > c.max {
			t.Errorf("%s %v: hit %d times of 400, want %d-%d", c.sword, c.seals, hit, c.min, c.max)
		}
	}
}
