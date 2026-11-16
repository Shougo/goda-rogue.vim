package rogue

import (
	"slices"
	"strings"
	"testing"
)

// item returns a new item id of the built-in dungeon at p.
func item(g *Game, id string, p Point) *Item {
	def := g.Dungeon.Item(id)
	it := &Item{Def: def, Pos: p, Glyph: def.Glyph}
	g.Items = append(g.Items, it)
	return it
}

func TestPickUpByWalking(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "yakusou", Point{r.X + 1, r.Y})
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 0 || !slices.Equal(g.Inventory, []*Item{it}) || !strings.Contains(g.Message, "薬草を拾った。") {
		t.Errorf("floor %v, inventory %v, %q", g.Items, g.Inventory, g.Message)
	}
}

func TestDashDoesNotPickUp(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "yakusou", Point{r.X + 2, r.Y})
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != it.Pos || len(g.Inventory) != 0 || !strings.Contains(g.Message, "薬草が落ちている。") {
		t.Fatalf("player %v, inventory %v, %q", g.Player, g.Inventory, g.Message)
	}
	turn := g.Turn
	if err := g.Do("pickup", nil); err != nil {
		t.Fatal(err)
	}
	if len(g.Inventory) != 1 || g.Turn != turn+1 {
		t.Errorf("pickup: inventory %v, turn %d", g.Inventory, g.Turn)
	}
	if err := g.Do("pickup", nil); err != nil {
		t.Fatal(err)
	}
	if g.Message != "足元には何もない。" || g.Turn != turn+1 {
		t.Errorf("pickup of nothing: %q, turn %d", g.Message, g.Turn)
	}
}

func TestMoney(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	it := item(g, "okane", Point{r.X + 1, r.Y})
	it.Amount = 123
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Gold != 123 || len(g.Inventory) != 0 {
		t.Errorf("gold %d, inventory %v", g.Gold, g.Inventory)
	}
	if top := g.Render(Size{80, 24}).Lines[0]; !strings.HasSuffix(top, "  123G") {
		t.Errorf("top line %q", top)
	}
}

func TestCapacity(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	for range g.Dungeon.Rules.Capacity {
		g.Inventory = append(g.Inventory, &Item{Def: g.Dungeon.Item("yakusou"), Glyph: "草"})
	}
	item(g, "onigiri", Point{r.X + 1, r.Y})
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 1 || len(g.Inventory) != g.Dungeon.Rules.Capacity || !strings.Contains(g.Message, "持ち物がいっぱい") {
		t.Errorf("floor %d, inventory %d, %q", len(g.Items), len(g.Inventory), g.Message)
	}
}

func TestInventorySorted(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	for i, id := range []string{"onigiri", "yakusou", "tetsuken", "kitate", "bokutou"} {
		item(g, id, Point{r.X + 1 + i, r.Y})
	}
	for range 5 {
		if err := g.Do("move", []string{"right"}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, it := range g.Inventory {
		got = append(got, it.Label())
	}
	want := []string{"剣 木刀", "剣 鉄の剣", "盾 木の盾", "草 薬草", "食 おにぎり"}
	if !slices.Equal(got, want) {
		t.Errorf("inventory %q, want %q", got, want)
	}
}

func TestInventoryPrompt(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if g.Prompt != nil || g.Message != "何も持っていない。" {
		t.Fatalf("empty: %+v, %q", g.Prompt, g.Message)
	}
	item(g, "yakusou", Point{r.X + 1, r.Y})
	item(g, "onigiri", Point{r.X + 2, r.Y})
	for range 2 {
		if err := g.Do("move", []string{"right"}); err != nil {
			t.Fatal(err)
		}
	}

	turn := g.Turn
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	s := g.Render(Size{80, 24})
	if s.Prompt == nil || s.Prompt.Title != "持ち物" || !slices.Equal(s.Prompt.Choices, []string{"草 薬草", "食 おにぎり"}) {
		t.Fatalf("prompt %+v", s.Prompt)
	}
	if err := g.Do("choose", []string{"9"}); err == nil {
		t.Error("a choice out of range was accepted")
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Prompt == nil || g.Prompt.Title != "草 薬草" || !slices.Contains(g.Prompt.Choices, commandDrop) {
		t.Fatalf("item prompt %+v", g.Prompt)
	}
	// Under the player, there is nothing (it picked the onigiri up).
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if g.Prompt != nil || len(g.Inventory) != 1 || g.Turn != turn+1 {
		t.Fatalf("drop: %+v, inventory %v, turn %d", g.Prompt, g.Inventory, g.Turn)
	}
	if it, ok := g.ItemAt(g.Player); !ok || it.Def.ID != "yakusou" {
		t.Errorf("under the player: %v", it)
	}

	// Cannot drop where something lies.
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := g.Do("choose", []string{"0"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.Inventory) != 1 || g.Message != "ここには置けない。" || g.Turn != turn+1 {
		t.Errorf("drop on an item: %v, %q, turn %d", g.Inventory, g.Message, g.Turn)
	}

	// Cancel, and any other action, drop the prompt.
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("cancel", nil); err != nil || g.Prompt != nil {
		t.Errorf("cancel: %v, %+v", err, g.Prompt)
	}
	if err := g.Do("inventory", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("wait", nil); err != nil || g.Prompt != nil {
		t.Errorf("wait: %v, %+v", err, g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err == nil {
		t.Error("choose without a prompt was accepted")
	}
}
