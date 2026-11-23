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

// TestSealNoTheft: with the seal nothing is stolen; without it, it is.
func TestSealNoTheft(t *testing.T) {
	for _, seals := range [][]string{{"盗"}, nil} {
		g, _ := sealGame(t, nil, seals)
		g.Gold = 100
		carry(g, "yakusou", 0)
		n := len(g.Inventory)
		for _, id := range []string{"touzoku1", "touzoku2"} {
			m := place(g, kind(id), g.Player.Add(Point{1, 0}))
			for i := range m.Def.Abilities {
				if a := m.Def.Abilities[i]; a.Name == AbilitySteal {
					a.Values = append([]int{100}, a.Values[1:]...)
					g.thieve(m, a)
				}
			}
		}
		stolen := g.Gold != 100 || len(g.Inventory) != n
		if stolen == (seals != nil) {
			t.Errorf("seals %v: gold %d, %d items of %d", seals, g.Gold, len(g.Inventory), n)
		}
	}
}

// TestSealDodge: the blows of the monsters hit less often by the
// strength of the seal.
func TestSealDodge(t *testing.T) {
	g, _ := sealGame(t, nil, []string{"避"})
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 15 })
	g.HP, g.MaxHP = 99999, 99999
	m := place(g, kind("nezumi1"), g.Player.Add(Point{1, 0}))
	for range 200 {
		g.hitPlayer(m)
	}
	if g.HP != g.MaxHP {
		t.Fatalf("hit at a rate of 15%% with a seal of 15: HP %d/%d", g.HP, g.MaxHP)
	}
	g.equipped(CategoryShield).Seals = nil
	for range 200 {
		g.hitPlayer(m)
	}
	if g.HP == g.MaxHP {
		t.Error("never hit at a rate of 15% without the seal")
	}
}
