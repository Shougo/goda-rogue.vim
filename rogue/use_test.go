package rogue

import (
	"strings"
	"testing"
)

// use uses the item id, carried: drinks, reads, waves or eats it.
func use(t *testing.T, g *Game, id string) *Item {
	t.Helper()
	it := carry(g, id, 0)
	if id == "fukitobashi" {
		it.Charges = 3
	}
	command(t, g, it, useCommands[it.Def.Category])
	return it
}

func TestGrass(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	g.HP, g.MaxHP = 10, 100
	turn := g.Turn
	use(t, g, "yakusou")
	if g.HP != 35 || g.Turn != turn+1 || len(g.Inventory) != 0 {
		t.Errorf("HP %d, turn %d, inventory %v", g.HP, g.Turn, g.Inventory)
	}
	if !strings.Contains(g.Message, "薬草を飲んだ。HPが25回復した。") {
		t.Errorf("message %q", g.Message)
	}
	use(t, g, "otogiri")
	if g.HP != 100 {
		t.Errorf("HP %d after 弟切草", g.HP)
	}
	// At full HP, the max HP goes up.
	use(t, g, "yakusou")
	if g.HP != 101 || g.MaxHP != 101 {
		t.Errorf("HP %d/%d", g.HP, g.MaxHP)
	}
	use(t, g, "otogiri")
	if g.MaxHP != 103 {
		t.Errorf("max HP %d", g.MaxHP)
	}
	s := g.Strength
	use(t, g, "chikarakusa")
	if g.Strength != s+1 {
		t.Errorf("strength %d", g.Strength)
	}
}

func TestLight(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	// A monster in another room.
	var far Point
	for _, o := range g.Level.Rooms {
		if o != r && !o.Dummy {
			far = Point{o.X, o.Y}
			break
		}
	}
	place(g, kind("nezumi1"), far).Dozing = true
	if g.Visible(far) || g.IsSeen(far) {
		t.Fatal("sees the other room")
	}
	use(t, g, "akari")
	if !g.Visible(far) || !g.IsSeen(far) {
		t.Error("the floor is not lit")
	}
	if s, _ := g.glyph(far); s != "鼠" {
		t.Errorf("%q in the other room", s)
	}
	g.descend()
	if g.Lit {
		t.Error("the next floor is lit")
	}
}

func TestIdentify(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	use(t, g, "shikibetsu")
	if !strings.Contains(g.Message, "識別するものはなかった。") || len(g.Inventory) != 0 {
		t.Errorf("message %q, inventory %v", g.Message, g.Inventory)
	}
}

func TestBlowAway(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 2, r.Y})
	m.HP = 100
	staff := use(t, g, "fukitobashi")
	if staff.Charges != 2 || !slicesContains(g.Inventory, staff) {
		t.Fatalf("charges %d", staff.Charges)
	}
	// It flies away (and then comes back a step), to the wall of the room
	// or beyond, and hurts itself if it hits something.
	if m.Pos.X <= r.X+3 {
		t.Errorf("at %v", m.Pos)
	}
	if strings.Contains(g.Message, "ぶつかって") != (m.HP == 100-blowAwayDamage) {
		t.Errorf("HP %d, message %q", m.HP, g.Message)
	}
	if !strings.Contains(g.Message, "鉄鼠を吹き飛ばした。") {
		t.Errorf("message %q", g.Message)
	}

	// A weak monster dies on the wall, and gives EXP.
	g.Monsters = nil
	w := place(g, kind("nezumi1"), Point{r.X + r.W - 2, r.Y})
	w.HP = 1
	exp := g.EXP
	use(t, g, "fukitobashi")
	if g.MonsterAt(w.Pos) == w || g.EXP <= exp {
		t.Errorf("not defeated: HP %d, EXP %d", w.HP, g.EXP)
	}

	// Nothing to hit.
	g.Monsters = nil
	use(t, g, "fukitobashi")
	if !strings.Contains(g.Message, "魔法弾は何にも当たらなかった。") {
		t.Errorf("message %q", g.Message)
	}

	// No charges left.
	empty := carry(g, "fukitobashi", 0)
	turn := g.Turn
	command(t, g, empty, commandWave)
	if !strings.Contains(g.Message, "何も起こらなかった。") || g.Turn != turn+1 || !slicesContains(g.Inventory, empty) {
		t.Errorf("message %q, turn %d", g.Message, g.Turn)
	}
	if g.Label(empty) != "杖 吹き飛ばしの杖[0]" {
		t.Errorf("label %q", g.Label(empty))
	}
}

func slicesContains(items []*Item, it *Item) bool {
	for _, x := range items {
		if x == it {
			return true
		}
	}
	return false
}

func TestHunger(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	// No monster comes to bother the player.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.SpawnTurns = 1 << 30 })
	r := g.Dungeon.Rules
	if g.Fullness != r.Fullness {
		t.Fatalf("fullness %d at the start", g.Fullness)
	}
	if !strings.Contains(g.topLine(map[string][]Span{}), "  満腹 100%  ") {
		t.Errorf("top line %q", g.topLine(map[string][]Span{}))
	}
	wait(t, g, r.HungerTurns*5)
	if g.Fullness != r.Fullness-5 {
		t.Errorf("fullness %d after %d turns", g.Fullness, r.HungerTurns*5)
	}

	g.Fullness, g.hunger = 31, 0
	wait(t, g, r.HungerTurns)
	if g.Fullness != 30 || !strings.Contains(strings.Join(g.Log, ""), "お腹が減ってきた。") {
		t.Errorf("fullness %d, log %q", g.Fullness, g.Log)
	}
	use(t, g, "onigiri")
	if g.Fullness != 80 {
		t.Errorf("fullness %d after eating", g.Fullness)
	}
	g.Fullness = 90
	use(t, g, "onigiri")
	if g.Fullness != r.Fullness {
		t.Errorf("fullness %d over the max", g.Fullness)
	}

	// A 回復 bracelet makes hungry twice as fast.
	command(t, g, carry(g, "kaifuku", 0), commandEquip)
	g.Fullness, g.hunger = 50, 0
	wait(t, g, r.HungerTurns*5)
	if g.Fullness != 40 {
		t.Errorf("fullness %d with 回復", g.Fullness)
	}

	// Starving: no HP comes back, a point goes each turn, and the run
	// ends.
	g.Fullness, g.HP, g.MaxHP = 0, 5, 50
	wait(t, g, 1)
	if g.HP != 4 {
		t.Errorf("HP %d while starving", g.HP)
	}
	wait(t, g, 4)
	if g.Over != "飢え死にした" {
		t.Errorf("over %q", g.Over)
	}
}

// brightGame starts a run in the built-in dungeon with every floor of the
// brightness b.
func brightGame(t *testing.T, b Brightness) *Game {
	t.Helper()
	d := *cave()
	d.FloorRows = append([]FloorCounts(nil), d.FloorRows...)
	for i := range d.FloorRows {
		d.FloorRows[i].Brightness = b
	}
	return NewGame(&d, "bright")
}

func TestBrightness(t *testing.T) {
	far := func(g *Game) Point {
		for i, r := range g.Level.Rooms {
			if i != g.Level.RoomAt(g.Player) && !r.Dummy {
				return Point{r.X, r.Y}
			}
		}
		t.Fatal("one room")
		return Point{}
	}

	g := brightGame(t, BrightnessBright)
	if p := far(g); !g.Visible(p) || !g.IsSeen(p) {
		t.Error("a bright floor is not seen")
	}
	g.descend()
	if !g.Lit {
		t.Error("the next bright floor is not lit")
	}

	g = brightGame(t, BrightnessDark)
	room := g.Level.Rooms[g.Level.RoomAt(g.Player)]
	inRoom := Point{room.X, room.Y}
	if inRoom == g.Player {
		inRoom = Point{room.X + room.W - 1, room.Y + room.H - 1}
	}
	if chebyshev(inRoom, g.Player) <= 1 {
		t.Skip("small room")
	}
	if g.Visible(inRoom) || g.IsSeen(inRoom) {
		t.Errorf("sees %v of its room on a dark floor", inRoom)
	}
	if !g.Visible(g.Player.Add(Point{1, 0})) {
		t.Error("does not see next to it")
	}
	// A scroll lights it.
	for _, it := range g.Dungeon.Items {
		g.identified[it.ID] = true
	}
	use(t, g, "akari")
	if !g.Visible(inRoom) {
		t.Error("the scroll does not light a dark floor")
	}
}
