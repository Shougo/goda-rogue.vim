package rogue

import (
	"slices"
	"strings"
	"testing"
)

// TestCatalog: the catalog shows the kinds identified with the fake name
// carried, a fake name carried with its price once seen in a shop, and
// the kinds not identified; it costs no turn and is not recorded.
func TestCatalog(t *testing.T) {
	g := shopGame(t, "yorozu")
	forget(g, CategoryGrass)
	herb := carry(g, "yakusou", 0)
	carry(g, "otogiri", 0)
	g.learn(herb)
	turn := g.Turn
	if err := g.Do("catalog", nil); err != nil {
		t.Fatal(err)
	}
	if g.Prompt == nil || g.Prompt.Title != "識別帳" || g.Turn != turn || recorded("catalog") {
		t.Fatalf("prompt %+v, turn %d to %d", g.Prompt, turn, g.Turn)
	}
	lines := g.Prompt.Choices
	if !slices.Contains(lines, "  薬草 ← "+g.fakeNames["yakusou"]) || !slices.Contains(lines, "  "+g.fakeNames["otogiri"]+"?") {
		t.Errorf("lines %q", lines)
	}
	if !slices.ContainsFunc(lines, func(l string) bool {
		return strings.HasPrefix(l, "  未識別: ") && strings.Contains(l, "上薬草 200G")
	}) {
		t.Errorf("no kinds not identified: %q", lines)
	}
	// In the shop: its price, and the kinds of that price.
	r := g.shopRoom()
	g.Player = Point{r.X + r.W/2, r.Y + r.H/2}
	if err := g.Do("catalog", nil); err != nil {
		t.Fatal(err)
	}
	want := "  " + g.fakeNames["otogiri"] + "? 200G: 上薬草"
	if !slices.Contains(g.Prompt.Choices, want) {
		t.Errorf("want %q in %q", want, g.Prompt.Choices)
	}
}
