package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestAllyFights: an ally goes for an awake enemy it sees near the
// player, lets a sleeping one sleep, and stays with the player otherwise.
func TestAllyFights(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	a := place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	a.Ally = true
	e := place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 1})
	g.act(a)
	if d := chebyshev(a.Pos, e.Pos); d != 3 {
		t.Errorf("ally at %v, %d from the enemy: want a step toward it", a.Pos, d)
	}
	a.Pos = Point{r.X + 4, r.Y + 1}
	hp := e.HP
	for range 20 {
		if e.HP < hp || !contains(g.Monsters, e) {
			break
		}
		g.act(a)
	}
	if e.HP == hp && contains(g.Monsters, e) {
		t.Errorf("the ally never hit the enemy next to it")
	}

	// A sleeping enemy: the ally stays with the player.
	g, r = roomGame(t, 8, 3)
	a = place(g, kind("nezumi1"), Point{r.X + 1, r.Y + 1})
	a.Ally = true
	place(g, kind("nezumi1"), Point{r.X + 5, r.Y + 1}).Dozing = true
	g.act(a)
	if a.Pos != (Point{r.X + 1, r.Y + 1}) {
		t.Errorf("ally at %v, want it staying next to the player", a.Pos)
	}
}

// TestAllyHitRate: an ally attacks at the dungeon's 仲間の命中, and an
// enemy attacks the ally at 敵の命中.
func TestAllyHitRate(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	d := *g.Dungeon
	d.Rules.AllyHitRate, d.Rules.MonsterHitRate = 0, 100
	g.Dungeon = &d
	a := place(g, kind("nezumi1"), Point{r.X + 3, r.Y + 1})
	a.Ally = true
	a.HP, a.MaxHP = 9999, 9999
	e := place(g, kind("nezumi1"), Point{r.X + 4, r.Y + 1})
	e.HP, e.MaxHP = 9999, 9999
	for range 20 {
		g.hitMonster(a, e)
		g.hitMonster(e, a)
	}
	if e.HP != 9999 {
		t.Errorf("the ally hit at 仲間の命中 0: enemy HP %d", e.HP)
	}
	if a.HP == 9999 {
		t.Error("the enemy never hit the ally at 敵の命中 100")
	}
	d.Rules.AllyHitRate = 100
	g.hitMonster(a, e)
	if e.HP == 9999 {
		t.Error("the ally missed at 仲間の命中 100")
	}
}

// AllyHitRate defaults to 敵の命中 in a dungeon without 仲間の命中.
func TestAllyHitRateDefault(t *testing.T) {
	if r := frozen(t).Rules; r.AllyHitRate != r.MonsterHitRate {
		t.Errorf("仲間の命中 %d, want 敵の命中 %d", r.AllyHitRate, r.MonsterHitRate)
	}
}

func contains(ms []*Monster, m *Monster) bool {
	for _, x := range ms {
		if x == m {
			return true
		}
	}
	return false
}

// wave waves a staff of the kind id, with charges, to the player's front.
func wave(t *testing.T, g *Game, id string) {
	t.Helper()
	it := carry(g, id, 0)
	it.Charges = 2
	command(t, g, it, commandWave)
}

// TestRecruit: a staff of 仲間 makes the monster it hits an ally, up to
// the dungeon's limit; not a shopkeeper.
func TestRecruit(t *testing.T) {
	g, r := roomGame(t, 10, 3)
	g.Monsters = nil
	g.HP, g.MaxHP = 9999, 9999
	g.Facing = "right"
	limit := g.Dungeon.Rules.MaxAllies
	if limit != 3 {
		t.Fatalf("the cave's limit is %d, want 3", limit)
	}
	for i := range limit {
		m := place(g, kind("oni1"), Point{r.X + 2 + i, r.Y})
		m.Dozing = true
		// (Each one in the front row, behind those recruited: move the
		// recruited ones out of the bolt's way.)
		for _, a := range g.allies() {
			a.Pos = Point{r.X + 2 + i, r.Y + 2}
			a.Order = OrderWait
		}
		wave(t, g, "nakama")
		if !m.Ally || m.Dozing || !strings.Contains(g.Message, m.Name+"が仲間になった！") {
			t.Fatalf("ally %v, dozing %v, %q", m.Ally, m.Dozing, g.Message)
		}
	}
	g.allies()[0].Pos, g.allies()[1].Pos, g.allies()[2].Pos = Point{r.X + 5, r.Y + 2}, Point{r.X + 6, r.Y + 2}, Point{r.X + 7, r.Y + 2}
	e := place(g, kind("oni1"), Point{r.X + 3, r.Y})
	e.Dozing = true
	wave(t, g, "nakama")
	if e.Ally || !strings.Contains(g.Message, "これ以上仲間を連れて行けない。") {
		t.Errorf("a fourth ally %v, %q", e.Ally, g.Message)
	}

	g, r = roomGame(t, 10, 3)
	g.Monsters = nil
	g.Facing = "right"
	k := place(g, g.Dungeon.Shops[0].Keeper, Point{r.X + 2, r.Y})
	k.Keeper = true
	wave(t, g, "nakama")
	if k.Ally || !strings.Contains(g.Message, "には効かなかった。") {
		t.Errorf("a shopkeeper ally %v, %q", k.Ally, g.Message)
	}
}

// TestOrders: looking at an ally asks what to tell it; told to wait it
// stays, free it goes for a far enemy; a sealed mouth gives no orders.
func TestOrders(t *testing.T) {
	order := func(g *Game, o Order) {
		t.Helper()
		if err := g.Do("look", nil); err != nil {
			t.Fatal(err)
		}
		if err := g.Do("choose", []string{"0"}); err != nil {
			t.Fatal(err)
		}
		if g.Prompt == nil || g.Prompt.Choices[0] != "ついてきて [今]" {
			t.Fatalf("prompt %+v", g.Prompt)
		}
		chooseLabel(t, g, string(o))
	}
	g, r := roomGame(t, 12, 3)
	g.Monsters = nil
	a := place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	a.Ally = true
	turn := g.Turn
	order(g, OrderWait)
	if a.Order != OrderWait || g.Turn != turn || !strings.Contains(g.Message, "鼠に「待って」と伝えた。") {
		t.Fatalf("order %q, turns %d, %q", a.Order, g.Turn-turn, g.Message)
	}
	g.Player = Point{r.X + 8, r.Y + 2}
	g.act(a)
	if a.Pos != (Point{r.X + 1, r.Y}) {
		t.Errorf("told to wait, it went to %v", a.Pos)
	}
	// Free: it goes for an enemy far from the player.
	g.Player = Point{r.X, r.Y + 2}
	e := place(g, kind("nezumi1"), Point{r.X + 10, r.Y})
	g.act(a)
	if a.Pos != (Point{r.X + 1, r.Y}) {
		t.Errorf("waiting, it went for a far enemy: %v", a.Pos)
	}
	a.Order = OrderFree
	g.act(a)
	if chebyshev(a.Pos, e.Pos) != 8 {
		t.Errorf("free, it is at %v: want a step to the enemy", a.Pos)
	}

	g, r = roomGame(t, 12, 3)
	g.Monsters = nil
	a = place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	a.Ally = true
	g.Sealed = true
	g.Do("look", nil)
	g.Do("choose", []string{"0"})
	if g.Prompt != nil || !strings.Contains(g.Message, "口が封じられていて命令できない。") {
		t.Errorf("prompt %+v, %q", g.Prompt, g.Message)
	}
}

// TestAlliesComeAlong: the allies next to the player or in its room go
// down the stairs with it, one told to wait following again; one in
// another room stays behind.
func TestAlliesComeAlong(t *testing.T) {
	g, r := roomGame(t, 12, 3)
	g.Monsters = nil
	near := place(g, kind("nezumi1"), Point{r.X + 1, r.Y})
	far := place(g, kind("nezumi2"), Point{r.X + 10, r.Y + 2})
	far.Order = OrderWait
	var out Point
	for i, tile := range g.Level.Tiles {
		p := Point{i % g.Level.W, i / g.Level.W}
		if tile.Walkable() && g.Level.RoomAt(p) != g.Level.RoomAt(g.Player) {
			out = p
		}
	}
	left := place(g, kind("nezumi3"), out)
	near.Ally, far.Ally, left.Ally = true, true, true
	g.descend()
	as := g.allies()
	if len(as) != 2 || !slices.Contains(as, near) || !slices.Contains(as, far) {
		t.Fatalf("%d allies came along, want the two of the room", len(as))
	}
	for _, a := range as {
		if chebyshev(a.Pos, g.Player) != 1 || a.order() != OrderFollow {
			t.Errorf("%s at %v (the player at %v), order %s", a.Name, a.Pos, g.Player, a.order())
		}
	}
}
