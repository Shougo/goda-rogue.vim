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
	// With no item at its feet, it goes for one it sees, before the
	// player.
	it := putAtFoot(g, "onigiri")
	it.Pos = Point{r.X + 7, r.Y + 3}
	m.Pos = Point{r.X + 5, r.Y + 3}
	if p, ok := g.itemToThrow(m); !ok || p != it.Pos {
		t.Errorf("item to throw %v %v", p, ok)
	}
	g.act(m)
	if chebyshev(m.Pos, it.Pos) != 1 {
		t.Errorf("went to %v, not to the item at %v; room %+v", m.Pos, it.Pos, r)
	}
	// Rank 1 throws on a line only; rank 3 from anywhere within its reach.
	g.Items = nil
	g.Player = Point{r.X, r.Y}
	for _, c := range []struct {
		id    string
		threw bool
	}{{"warabe1", false}, {"warabe3", true}} {
		g.Monsters = nil
		u := place(g, kind(c.id), Point{r.X + 2, r.Y + 1})
		stone := putAtFoot(g, "onigiri")
		stone.Pos = u.Pos
		a, _ := u.ability(AbilityThrow)
		a.Values = []int{a.Values[0], 100, a.Values[2]}
		if got := g.throwAt(u, a); got != c.threw {
			t.Errorf("%s threw %v", c.id, got)
		}
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
	p.Contents = []*Item{item(g, "yakusou", Point{})}
	g.Items = nil
	if a.Values[1] = ruinPot; !g.ruin(m, a) || p.Def.Effect != EffectPlain || p.Charges != 3 || len(p.Contents) != 1 {
		t.Errorf("pot %s, room %d, contents %d", p.Def.ID, p.Charges, len(p.Contents))
	}
	// What is in a plain pot neither comes out nor is used.
	i := slices.Index(g.Inventory, p)
	if cs := inPotCommands(p, p.Contents[0]); slices.Contains(cs, commandDrink) {
		t.Errorf("commands %v", cs)
	}
	g.doInPot(i, 0, commandTakeOut)
	if len(p.Contents) != 1 {
		t.Error("taken out of a plain pot")
	}
	// Nothing to ruin: it attacks.
	if g.ruin(m, a) {
		t.Error("ruined a plain pot")
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

// TestRustBy: a rust bug never attacks; it rusts the weapon or the
// shield, both at rank 2, or takes a seal away at ranks 3 and 4, a seal of
// サビよけ first; one born with サビよけ makes it fail, a blessing takes
// the rust.
func TestRustBy(t *testing.T) {
	g, _ := sealGame(t, nil, nil)
	m := place(g, kind("sabimushi2"), g.Player.Add(Point{1, 0}))
	sword, board := g.equipped(CategoryWeapon), g.equipped(CategoryShield)
	sword.Upgrade, board.Upgrade = 3, 3
	a := Ability{Name: AbilityRust, Values: []int{100, 2, rustBoth}}
	if !g.rustBy(m, a) || sword.Upgrade != 1 || board.Upgrade != 1 {
		t.Errorf("both: upgrades %d %d", sword.Upgrade, board.Upgrade)
	}
	a.Values[2] = rustOne
	if !g.rustBy(m, a) || sword.Upgrade+board.Upgrade != 0 {
		t.Errorf("one: upgrades %d %d", sword.Upgrade, board.Upgrade)
	}
	// By its chance only; otherwise it watches.
	hp := g.HP
	if a.Values[0] = 0; !g.rustBy(m, a) || sword.Upgrade+board.Upgrade != 0 || g.HP != hp {
		t.Error("not watching")
	}
	a.Values[0] = 100
	// A seal of サビよけ is taken away instead.
	board.Seals = []string{"防", "錆"}
	if g.rustBy(m, a); !slices.Equal(board.Seals, []string{"防"}) || sword.Upgrade+board.Upgrade != 0 {
		t.Errorf("seals %v, upgrades %d %d", board.Seals, sword.Upgrade, board.Upgrade)
	}
	// Seals or rust, half and half.
	a.Values[2] = rustOrSeal
	sword.Seals = []string{"攻", "攻", "攻", "攻"}
	board.Seals = nil
	for range 8 {
		g.rustBy(m, a)
	}
	if len(sword.Seals) == 4 || sword.Upgrade+board.Upgrade == 0 {
		t.Errorf("seals %v, upgrades %d %d", sword.Seals, sword.Upgrade, board.Upgrade)
	}
	// A blessing takes the rust.
	a.Values[2] = rustBoth
	up := sword.Upgrade
	sword.Blessed = true
	if g.rustBy(m, a); sword.Upgrade != up || sword.Blessed {
		t.Errorf("blessed: upgrade %d (was %d), blessed %v", sword.Upgrade, up, sword.Blessed)
	}
	// Born with サビよけ: it fails.
	board.Equipped = false
	gold := carry(g, "kintate", 0)
	gold.Equipped = true
	up = sword.Upgrade
	if g.rustBy(m, a); sword.Upgrade != up || gold.Upgrade != 0 {
		t.Errorf("born with サビよけ: upgrades %d %d", sword.Upgrade, gold.Upgrade)
	}
	// With nothing equipped, it fails.
	sword.Equipped, gold.Equipped = false, false
	if !g.rustBy(m, a) || g.HP != hp {
		t.Error("nothing equipped")
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

// TestGuardBracelets: with the bracelet of サビよけ nothing rusts, with
// that of 盗みよけ nothing is stolen nor ruined.
func TestGuardBracelets(t *testing.T) {
	g, _ := sealGame(t, nil, nil)
	sword := g.equipped(CategoryWeapon)
	carry(g, "sabiyoke", 0).Equipped = true
	m := place(g, kind("sabimushi4"), g.Player.Add(Point{1, 0}))
	sword.Seals = []string{"攻"}
	for range 10 {
		g.rustBy(m, Ability{Name: AbilityRust, Values: []int{100, 3, rustOrSeal}})
	}
	if g.rust([]Category{CategoryWeapon}, 1); sword.Upgrade != 0 || len(sword.Seals) != 1 {
		t.Errorf("rusted: upgrade %d, seals %v", sword.Upgrade, sword.Seals)
	}
	g.equipped(CategoryBracelet).Equipped = false
	carry(g, "nusumiyokeu", 0).Equipped = true
	grass := carry(g, "yakusou", 0)
	crow := place(g, kind("karasu2"), g.Player.Add(Point{0, 1}))
	g.ruin(crow, Ability{Name: AbilityRuin, Values: []int{100, ruinGrass}})
	thief := place(g, kind("touzoku1"), g.Player.Add(Point{-1, 0}))
	g.Gold = 100
	stolen := g.thieve(thief, Ability{Name: AbilitySteal, Values: []int{100, 0}})
	if !slices.Contains(g.Inventory, grass) || stolen || g.Gold != 100 {
		t.Errorf("grass carried %v, stolen %v, gold %d", slices.Contains(g.Inventory, grass), stolen, g.Gold)
	}
}
