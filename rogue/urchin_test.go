package rogue

import (
	"slices"
	"testing"
)

// TestThrowAt: a monster that throws throws the item it stands on at the
// player; a grass has its effect on the player.
func TestThrowAt(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterThrowRate = 100 })
	g.Player = Point{r.X, r.Y + 1}
	g.HP, g.MaxHP = 10, 100
	m := place(g, kind("warabe4"), Point{r.X + 4, r.Y + 1})
	grass := putAtFoot(g, "yakusou")
	grass.Pos = m.Pos
	for range 50 {
		if g.HP > 10 {
			break
		}
		g.act(m)
	}
	if g.HP <= 10 || slices.Contains(g.Items, grass) {
		t.Errorf("HP %d, the grass on the floor %v", g.HP, slices.Contains(g.Items, grass))
	}
	// With no item at its feet, it goes for one it sees.
	g.Player = Point{-5, -5}
	it := putAtFoot(g, "onigiri")
	it.Pos = Point{r.X + 7, r.Y + 3}
	if p, ok := g.itemToThrow(m); !ok || p != it.Pos {
		t.Errorf("item to throw %v %v", p, ok)
	}
}

// TestReflect: a mirror sends the bolt of a staff back to the player, and
// the arrows (rank 3).
func TestReflect(t *testing.T) {
	g, r := roomGame(t, 8, 4)
	g.Player = Point{r.X, r.Y + 1}
	g.Facing = "right"
	m := place(g, kind("kagami3"), Point{r.X + 3, r.Y + 1})
	m.Asleep = 99
	staff := carry(g, "donsoku", 0)
	staff.Charges = 3
	command(t, g, staff, commandWave)
	if g.Slowed == 0 || m.Slowed != 0 {
		t.Errorf("player slowed %d, mirror slowed %d", g.Slowed, m.Slowed)
	}
	g.Slowed = 0
	g.HP, g.MaxHP = 999, 999
	arrows := carry(g, "kiya", 0)
	arrows.Count, arrows.Equipped = 5, true
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	hp := m.HP
	if err := g.Do("shoot", nil); err != nil {
		t.Fatal(err)
	}
	if g.HP == 999 || m.HP != hp {
		t.Errorf("arrow: player HP %d, mirror HP %d of %d", g.HP, m.HP, hp)
	}
	// Rank 1 reflects no arrow.
	m.Def, m.Asleep = kind("kagami1"), 99
	g.HP = 999
	for range 3 {
		if err := g.Do("shoot", nil); err != nil {
			t.Fatal(err)
		}
	}
	if g.HP != 999 {
		t.Errorf("rank 1 sent an arrow back: HP %d", g.HP)
	}
}

// TestRuin: a crow ruins a staff, a grass, a pot; the seal 盗みよけ wards
// it off.
func TestRuin(t *testing.T) {
	g, _ := sealGame(t, nil, nil)
	m := place(g, kind("karasu1"), g.Player.Add(Point{1, 0}))
	staff := carry(g, "donsoku", 0)
	staff.Charges = 4
	a := Ability{Name: AbilityRuin, Values: []int{100, ruinStaff}}
	if !g.ruin(m, a) || staff.Charges != 0 {
		t.Errorf("staff charges %d", staff.Charges)
	}
	grass := carry(g, "yakusou", 0)
	if a.Values[1] = ruinGrass; !g.ruin(m, a) || slices.Contains(g.Inventory, grass) {
		t.Error("the grass is still carried")
	}
	p := pot(g, "gousei", 3)
	if a.Values[1] = ruinPot; !g.ruin(m, a) || p.Def.Effect != EffectKeep || p.Charges != 3 {
		t.Errorf("pot %s, room %d", p.Def.ID, p.Charges)
	}
	// Nothing to ruin: it attacks.
	if g.ruin(m, a) {
		t.Error("ruined a pot of keeping")
	}
	// Warded off.
	g.equipped(CategoryShield).Equipped = false
	carry(g, "nusumiyoke", 0).Equipped = true
	carry(g, "yakusou", 0)
	n := len(g.Inventory)
	if a.Values[1] = ruinGrass; !g.ruin(m, a) || len(g.Inventory) != n {
		t.Error("not warded off")
	}
}

// TestRustBy: a rust bug rusts the weapon or the shield, both at rank 2.
func TestRustBy(t *testing.T) {
	g, _ := sealGame(t, nil, nil)
	m := place(g, kind("sabimushi2"), g.Player.Add(Point{1, 0}))
	sword, board := g.equipped(CategoryWeapon), g.equipped(CategoryShield)
	sword.Upgrade, board.Upgrade = 3, 3
	a := Ability{Name: AbilityRust, Values: []int{100, 2, 1}}
	if !g.rustBy(m, a) || sword.Upgrade != 1 || board.Upgrade != 1 {
		t.Errorf("upgrades %d %d", sword.Upgrade, board.Upgrade)
	}
	a.Values[2] = 0
	if !g.rustBy(m, a) || sword.Upgrade+board.Upgrade != 0 {
		t.Errorf("one: upgrades %d %d", sword.Upgrade, board.Upgrade)
	}
}

// TestSendsBack: what a mirror sends back, by its rank.
func TestSendsBack(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	one := place(g, kind("kagami1"), Point{-1, -1})
	three := place(g, kind("kagami3"), Point{-2, -2})
	if !sendsBack(one, reflectStaffs) || sendsBack(one, reflectArrows) || !sendsBack(three, reflectArrows) || sendsBack(three, reflectAll) {
		t.Error("what a mirror sends back")
	}
}
