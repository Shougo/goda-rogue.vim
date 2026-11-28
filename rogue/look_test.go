package rogue

import (
	"strings"
	"testing"
)

func TestLook(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	if err := g.Do("look", nil); err != nil {
		t.Fatal(err)
	}
	if g.Prompt != nil || g.Message != "見えるものはない。" {
		t.Fatalf("prompt %+v, message %q", g.Prompt, g.Message)
	}

	far := place(g, kind("hebi1"), Point{r.X + 4, r.Y + 2})
	far.Dozing = true
	near := place(g, kind("ookami2"), Point{r.X + 1, r.Y})
	item(g, "yakusou", Point{r.X + 2, r.Y + 1})
	if err := g.Do("look", nil); err != nil {
		t.Fatal(err)
	}
	p := g.Prompt
	want := []string{
		near.Glyph + " " + near.Name + "  右1",
		"草 薬草  右2 下1",
		far.Glyph + " " + far.Name + "  右4 下2",
	}
	if p == nil || p.Title != "見えるもの" || strings.Join(p.Choices, "|") != strings.Join(want, "|") {
		t.Fatalf("prompt %+v", p)
	}
	if err := g.Do("choose", []string{"2"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(g.Message, far.Name+"：ランク1  ") || !strings.Contains(g.Message, "眠っている") || g.Turn != 0 {
		t.Errorf("message %q, turn %d", g.Message, g.Turn)
	}
	if err := g.Do("look", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Message, "倍速") {
		t.Errorf("message %q", g.Message)
	}
}

// TestLookAbilities: looking at a monster tells what its abilities do,
// from their numbers, its states, and what the dungeon's maker says of
// it.
func TestLookAbilities(t *testing.T) {
	look := func(g *Game) string {
		t.Helper()
		if err := g.Do("look", nil); err != nil {
			t.Fatal(err)
		}
		if err := g.Do("choose", []string{"0"}); err != nil {
			t.Fatal(err)
		}
		return g.Message
	}
	g, r := roomGame(t, 6, 3)
	place(g, kind("shashu3"), Point{r.X + 2, r.Y})
	if msg := look(g); !strings.Contains(msg, "直線上10マス以内から鉄の矢を撃つ（貫通）。") {
		t.Errorf("message %q", msg)
	}

	g, r = roomGame(t, 6, 3)
	def := *kind("mahou2")
	def.Description = "杖を持った魔物。"
	m := place(g, &def, Point{r.X + 2, r.Y})
	m.Sealed, m.Confused = true, 3
	msg := look(g)
	for _, want := range []string{"  混乱  封印", "（封印中）直線上10マス以内から50%で杖を振る（混乱、鈍足、レベル-1）。", "杖を持った魔物。"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q, want %q", msg, want)
		}
	}

	// Each ability has its own text, not its bare name.
	for _, d := range cave().Monsters {
		for _, a := range d.Abilities {
			if text := g.abilityText(a); text == string(a.Name) {
				t.Errorf("%s: no text for %s", d.ID, a.Name)
			}
		}
	}
}
