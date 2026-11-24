package rogue

import (
	"slices"
	"testing"
)

// TestBotSynthesis: a bot with a pot of synthesis it knows puts the
// weapon it has equipped in, then another worth merging, breaks the pot
// on a wall, and equips the weapon merged.
func TestBotSynthesis(t *testing.T) {
	g, _ := roomGame(t, 6, 4)
	b := NewBot(BotCareful, 1)
	g.HP, g.MaxHP = 50, 50
	katana := carry(g, "katana", 0)
	katana.Equipped, katana.Known = true, true
	carry(g, "kaishin", 0)
	p := pot(g, "gousei", 3)
	g.identified[p.Def.ID] = true
	var rules []string
	for range 40 {
		action, args := b.Next(g)
		if g.Prompt == nil {
			rules = append(rules, b.rule)
		}
		if err := g.Do(action, args); err != nil {
			t.Fatal(err)
		}
		if katana.Equipped && slices.Contains(katana.Seals, "会") && slices.Contains(g.Inventory, katana) {
			return
		}
	}
	t.Errorf("katana %+v, seals %v, carried %v; rules %q", katana.Equipped, katana.Seals, slices.Contains(g.Inventory, katana), rules)
}
