package rogue

import (
	"fmt"
	"slices"
	"testing"
)

// TestSealScrolls: 印増大 adds a slot to a weapon or a shield chosen (10
// at most), 印消し takes a seal chosen away.
func TestSealScrolls(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	sword := carry(g, "kaishin", 0)
	sword.Seals = []string{"攻", "必", "攻"}
	scroll := carry(g, "inzoudai", 0)
	command(t, g, scroll, commandRead)
	if g.Prompt == nil || g.Prompt.kind != promptSlot {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("choose", []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if sword.slots() != 4 || !slices.Contains(g.info(sword), "印 3/4") {
		t.Errorf("slots %d, %q", sword.slots(), g.info(sword))
	}
	sword.MoreSlots = maxSlots - sword.Def.Slots
	command(t, g, carry(g, "inzoudai", 0), commandRead)
	choose(t, g, sword)
	if sword.slots() != maxSlots || g.Message != "何も起こらなかった。" {
		t.Errorf("slots %d: %q", sword.slots(), g.Message)
	}

	erase := carry(g, "inkeshi", 0)
	command(t, g, erase, commandRead)
	if g.Prompt == nil || g.Prompt.kind != promptEraseItem || len(g.Prompt.Choices) != len(g.Inventory) {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	choose(t, g, sword)
	if g.Prompt == nil || g.Prompt.kind != promptErase || len(g.Prompt.Choices) != 3 {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	if err := g.Do("choose", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sword.Seals, []string{"攻", "攻"}) || sealsLabel(sword) != "[会]【攻攻】" {
		t.Errorf("seals %v, %q", sword.Seals, sealsLabel(sword))
	}
	// The ability born with is no seal: nothing to take away.
	sword.Seals = nil
	command(t, g, carry(g, "inkeshi", 0), commandRead)
	choose(t, g, sword)
	if g.Prompt != nil {
		t.Error("asked to erase the ability born with")
	}
}

// choose chooses it in the prompt of g.
func choose(t *testing.T, g *Game, it *Item) {
	t.Helper()
	if g.Prompt == nil {
		t.Fatal("no prompt")
	}
	i := slices.Index(g.Prompt.targets, it)
	if i < 0 {
		t.Fatalf("%s not offered: %v", it.Def.ID, g.Prompt.Choices)
	}
	if err := g.Do("choose", []string{fmt.Sprint(i)}); err != nil {
		t.Fatal(err)
	}
}
