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
	if !strings.HasPrefix(g.Message, far.Name+"：ランク1  ") || !strings.HasSuffix(g.Message, "眠っている") || g.Turn != 0 {
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
