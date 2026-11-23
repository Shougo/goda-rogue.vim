package rogue

import (
	"slices"
	"strings"
	"testing"
)

// synthGame returns a game with the monster of synthesis id in front of
// the player, two tiles away, and every throw hitting.
func synthGame(t *testing.T, id string) (*Game, *Monster) {
	t.Helper()
	g, r := roomGame(t, 12, 5)
	g.HP, g.MaxHP = 999, 999
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerThrowRate = 100 })
	g.Player = Point{r.X + 3, r.Y + 2}
	g.Facing = directionName(Point{1, 0})
	m := place(g, kind(id), Point{r.X + 5, r.Y + 2})
	m.Asleep = 99
	return g, m
}

// TestSwallow: the monster swallows what is thrown at it, as many items
// as it can; weapons merge, a material gives its seal (which no pot
// does), the rest stays apart; then a thrown item hits it. Defeated, it
// drops it all.
func TestSwallow(t *testing.T) {
	g, m := synthGame(t, "gou2")
	sword := carry(g, "katana", 1)
	other := carry(g, "bokutou", 2)
	other.Seals = []string{"会"}
	grass := carry(g, "chikarakusa", 0)
	command(t, g, sword, commandThrow)
	if !slices.Equal(m.belly, []*Item{sword}) || slices.Contains(g.Inventory, sword) || len(g.Items) != 0 ||
		!strings.Contains(g.Message, "大混ぜ虫はカタナを飲み込んだ。") || m.HP != m.MaxHP {
		t.Fatalf("belly %v, items %v, HP %d, %q", m.belly, g.Items, m.HP, g.Message)
	}
	command(t, g, other, commandThrow)
	command(t, g, grass, commandThrow)
	if len(m.belly) != 1 || sword.Upgrade != 3 || !slices.Equal(sword.Seals, []string{"会", "力"}) || m.room() != 0 {
		t.Fatalf("belly %v, upgrade %d, seals %v, room %d", m.belly, sword.Upgrade, sword.Seals, m.room())
	}
	// Full: the next one hits.
	stone := carry(g, "yakusou", 0)
	command(t, g, stone, commandThrow)
	if len(m.belly) != 1 || !strings.Contains(g.Message, "薬草が大混ぜ虫に当たった。") {
		t.Errorf("full: belly %v, %q", m.belly, g.Message)
	}
	g.kill(m, nil, true, true)
	if !slices.Contains(g.Items, sword) || sword.Pos != m.Pos || len(g.Items) != 1 {
		t.Errorf("items %v: want the sword where it fell", g.Items)
	}

	// What does not merge stays apart: two grasses, as in a pot.
	g, m = synthGame(t, "gou2")
	a, b := carry(g, "yakusou", 0), carry(g, "chikarakusa", 0)
	command(t, g, a, commandThrow)
	command(t, g, b, commandThrow)
	if !slices.Equal(m.belly, []*Item{a, b}) || m.room() != 1 {
		t.Errorf("belly %v, room %d", m.belly, m.room())
	}

	// Sealed, it swallows nothing.
	g, m = synthGame(t, "gou1")
	m.Sealed = true
	a = carry(g, "bokutou", 0)
	command(t, g, a, commandThrow)
	if len(m.belly) != 0 || !slices.Contains(g.Items, a) {
		t.Errorf("sealed: belly %v, items %v", m.belly, g.Items)
	}
}

// TestSuck: ranks 3 and 4, next to the player, suck a weapon or a shield
// in instead of a blow, an equipped and cursed one too, while they have
// room; ranks 1 and 2 do not.
func TestSuck(t *testing.T) {
	near := func(id string) (*Game, *Monster) {
		g, m := synthGame(t, id)
		m.Asleep = 0
		m.Pos = g.Player.Add(Point{1, 0})
		return g, m
	}
	g, m := near("gou4")
	for i := range m.Def.Abilities {
		if m.Def.Abilities[i].Name == AbilitySynthesis {
			def := *m.Def
			def.Abilities = slices.Clone(def.Abilities)
			def.Abilities[i].Values = []int{2, 100}
			m.Def = &def
		}
	}
	sword := carry(g, "katana", 0)
	sword.Cursed = true
	// (Equipping takes a turn: it sleeps through it.)
	m.Asleep = 99
	command(t, g, sword, commandEquip)
	m.Asleep = 0
	shield := carry(g, "kitate", 0)
	grass := carry(g, "yakusou", 0)
	p := pot(g, "hozon", 2)
	in := carry(g, "bokutou", 0)
	g.Inventory = slices.DeleteFunc(g.Inventory, func(x *Item) bool { return x == in })
	p.Contents = []*Item{in}
	g.act(m)
	g.act(m)
	if !slices.Contains(m.belly, sword) || !slices.Contains(m.belly, shield) || sword.Equipped || g.HP != 999 {
		t.Fatalf("belly %v, equipped %v, HP %d, %q", m.belly, sword.Equipped, g.HP, g.Message)
	}
	if !slices.Contains(g.Inventory, grass) || !slices.Contains(p.Contents, in) || !strings.Contains(g.Message, "を吸い込まれた！") {
		t.Errorf("inventory %v, pot %v, %q", g.Inventory, p.Contents, g.Message)
	}
	// Full: it hits.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
	carry(g, "katana", 0)
	g.act(m)
	if len(m.belly) != 2 || g.HP == 999 {
		t.Errorf("full: belly %v, HP %d", m.belly, g.HP)
	}

	g, m = near("gou2")
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.MonsterHitRate = 100 })
	sword = carry(g, "katana", 0)
	for range 20 {
		g.act(m)
	}
	if !slices.Contains(g.Inventory, sword) || len(m.belly) != 0 {
		t.Errorf("rank 2 sucked: belly %v", m.belly)
	}
}
