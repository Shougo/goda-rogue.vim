package rogue

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// unidentified is a game in a room, with the items unidentified as at the
// start of a run.
func unidentified(t *testing.T) *Game {
	t.Helper()
	g, _ := roomGame(t, 4, 3)
	g.identified = map[string]bool{}
	g.nameItems()
	return g
}

func TestFakeNames(t *testing.T) {
	g := NewGame(cave(), "names")
	d := g.Dungeon
	used := map[string]bool{}
	for _, it := range d.Items {
		item := &Item{Def: it}
		if len(d.FakeNames[it.Category]) == 0 {
			if !g.identified[it.ID] {
				t.Errorf("%s is unidentified", it.ID)
			}
			continue
		}
		name := g.Name(item)
		if g.identified[it.ID] || !slices.Contains(d.FakeNames[it.Category], name) {
			t.Errorf("%s: %q", it.ID, name)
		}
		if used[name] {
			t.Errorf("%q twice", name)
		}
		used[name] = true
	}

	// The same seed names them the same; another one, mostly not.
	same, other := NewGame(cave(), "names"), NewGame(cave(), "other")
	differ := 0
	for _, it := range d.Items {
		if g.fakeNames[it.ID] != same.fakeNames[it.ID] {
			t.Errorf("%s: %q, then %q", it.ID, g.fakeNames[it.ID], same.fakeNames[it.ID])
		}
		if g.fakeNames[it.ID] != other.fakeNames[it.ID] {
			differ++
		}
	}
	if differ == 0 {
		t.Error("another seed gives the same names")
	}
	// Names do not change the floors.
	if !slices.Equal(g.Level.Tiles, same.Level.Tiles) {
		t.Error("the same seed made other floors")
	}
}

func TestUseIdentifies(t *testing.T) {
	g := unidentified(t)
	g.HP, g.MaxHP = 10, 100
	it := carry(g, "yakusou", 0)
	fake := g.Name(it)
	if g.Label(it) != "草 "+fake+"?" {
		t.Errorf("label %q", g.Label(it))
	}
	command(t, g, it, commandDrink)
	want := fake + "を飲んだ。" + fake + "は薬草だった。HPが50回復した。"
	if g.Message != want {
		t.Errorf("message %q, want %q", g.Message, want)
	}
	// Every 薬草 is known now.
	if other := carry(g, "yakusou", 0); g.Label(other) != "草 薬草" {
		t.Errorf("label %q", g.Label(other))
	}
}

func TestEquipIdentifies(t *testing.T) {
	g := unidentified(t)
	b := carry(g, "kehai", 0)
	b.Known = false
	fake := g.Name(b)
	// Equipping a bracelet does not tell what it is: its effect showing
	// does.
	command(t, g, b, commandEquip)
	if !g.unknown(b) || strings.Contains(g.Message, "だった。") {
		t.Fatalf("identified when equipped: %q", g.Message)
	}
	// A monster out of sight, within the bracelet's reach (the player
	// moved near one, if none is near: the floor comes from the data).
	reach := b.Def.Strength
	placed := false
	for i := 0; i < len(g.Level.Tiles) && !placed; i++ {
		p := Point{i % g.Level.W, i / g.Level.W}
		if !g.Level.At(p).Walkable() || !g.free(p) {
			continue
		}
		for j := range g.Level.Tiles {
			q := Point{j % g.Level.W, j / g.Level.W}
			if q == p || !g.Level.At(q).Walkable() || !g.free(q) || chebyshev(p, q) > reach {
				continue
			}
			from := g.Player
			g.Player = q
			if g.Visible(p) {
				g.Player = from
				continue
			}
			place(g, kind("nezumi1"), p).Dozing = true
			placed = true
			break
		}
	}
	if !placed {
		t.Fatal("no tile out of sight within the reach")
	}
	wait(t, g, 1)
	if g.unknown(b) || !strings.Contains(strings.Join(g.Log, ""), fake+"は気配察知の腕輪だった。") {
		t.Errorf("not identified by its effect: %q", g.Message)
	}

	// The upgrade value of a weapon shows once it is equipped.
	sword := carry(g, "tetsuken", 2)
	sword.Known = false
	if g.Label(sword) != "剣 鉄の剣?" {
		t.Errorf("label %q", g.Label(sword))
	}
	command(t, g, sword, commandEquip)
	if g.Entry(sword) != "剣 E 鉄の剣+2" {
		t.Errorf("entry %q", g.Entry(sword))
	}
}

func TestStaffCharges(t *testing.T) {
	g := unidentified(t)
	staff := carry(g, "fukitobashi", 0)
	staff.Charges, staff.Known = 3, false
	if l := g.Label(staff); l != "杖 "+g.fakeNames["fukitobashi"]+"?" {
		t.Errorf("label %q", l)
	}
	// Waving it tells what it is; its charges stay unknown, the waves
	// are counted.
	command(t, g, staff, commandWave)
	if l := g.Label(staff); l != "杖 吹き飛ばしの杖[-1]" {
		t.Errorf("label %q", l)
	}
	command(t, g, staff, commandWave)
	if l := g.Label(staff); l != "杖 吹き飛ばしの杖[-2]" {
		t.Errorf("label %q", l)
	}
	// Another one of the kind, not waved yet.
	other := carry(g, "fukitobashi", 0)
	other.Known = false
	if l := g.Label(other); l != "杖 吹き飛ばしの杖[?]" {
		t.Errorf("label %q", l)
	}
	// Known, the charges show.
	staff.Known = true
	if l := g.Label(staff); l != "杖 吹き飛ばしの杖[1]" {
		t.Errorf("label %q", l)
	}
}

func TestIdentifyScroll(t *testing.T) {
	g := unidentified(t)
	grass := carry(g, "otogiri", 0)
	sword := carry(g, "bokutou", 1)
	sword.Known = false
	carry(g, "onigiri", 0) // Known: nothing to identify.
	scroll := carry(g, "shikibetsu", 0)
	fake := g.Name(scroll)
	turn := g.Turn
	command(t, g, scroll, commandRead)
	if !strings.Contains(g.Message, fake+"は識別の巻物だった。") || g.Turn != turn+1 {
		t.Errorf("message %q, turn %d", g.Message, g.Turn)
	}
	p := g.Prompt
	if p == nil || p.Title != "何を識別する？" || len(p.Choices) != 2 {
		t.Fatalf("prompt %+v", p)
	}
	i := slices.Index(p.Choices, g.Entry(grass))
	if err := g.Do("choose", []string{fmt.Sprint(i)}); err != nil {
		t.Fatal(err)
	}
	if g.Name(grass) != "弟切草" || !strings.Contains(g.Message, "は弟切草だった。") || g.Turn != turn+1 {
		t.Errorf("message %q, turn %d", g.Message, g.Turn)
	}

	// The sword: its upgrade value.
	command(t, g, carry(g, "shikibetsu", 0), commandRead)
	if len(g.Prompt.Choices) != 1 {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Message != "木刀+1を識別した。" {
		t.Errorf("message %q", g.Message)
	}

	// An item under the player too.
	foot := putAtFoot(g, "chikarakusa")
	command(t, g, carry(g, "shikibetsu", 0), commandRead)
	if !slices.Equal(g.Prompt.Choices, []string{g.Entry(foot) + " [足元]"}) {
		t.Fatalf("choices %q", g.Prompt.Choices)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.unknown(foot) {
		t.Error("the item under the player is still unknown")
	}
	g.discard(foot)

	// Nothing left to identify.
	command(t, g, carry(g, "shikibetsu", 0), commandRead)
	if g.Prompt != nil || !strings.Contains(g.Message, "識別するものはなかった。") {
		t.Errorf("prompt %+v, message %q", g.Prompt, g.Message)
	}
}
