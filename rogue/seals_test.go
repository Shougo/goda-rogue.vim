package rogue

import (
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
	if g.askSlot() {
		t.Error("an 11th slot")
	}
	g.Prompt = nil

	erase := carry(g, "inkeshi", 0)
	command(t, g, erase, commandRead)
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
	if g.askErase() {
		t.Error("asked to erase the ability born with")
	}
}
