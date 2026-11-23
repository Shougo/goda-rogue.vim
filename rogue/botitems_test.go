package rogue

import "testing"

// planned plays the bot's next action against a dragon next to the
// player (a fight it would likely lose, and an item can turn), with the item of the kind id carried and known, and returns
// the command it plans with it ("" if it does something else).
func planned(t *testing.T, id string) string {
	t.Helper()
	g, r := roomGame(t, 8, 3)
	b := NewBot(BotCareful, 1)
	g.Player = Point{r.X + 1, r.Y + 1}
	g.Facing = "right"
	g.HP, g.MaxHP = 80, 80
	m := place(g, kind("ryu2"), Point{r.X + 2, r.Y + 1})
	if !b.dangerous(g, []*Monster{m}) {
		t.Fatal("not dangerous")
	}
	it := carry(g, id, 0)
	it.Charges = 3
	if a, _ := b.Next(g); a != "inventory" || len(b.plan) != 2 || b.plan[0].item != it {
		return ""
	}
	return b.plan[1].label
}

// TestBotUsesItems: in danger, the bots throw the grasses of sleep and
// of confusion, drink the grass of warping, read the scrolls of the
// room, and wave the staffs that weaken; not what does not help.
func TestBotUsesItems(t *testing.T) {
	for id, want := range map[string]string{
		"suimin":     commandThrow,
		"konran":     commandThrow,
		"takatobi":   commandDrink,
		"bakusui":    commandRead,
		"konranmaki": commandRead,
		"fukou":      commandWave,
		"donsoku":    commandWave,
		"subayasa":   "",
		"bashogae":   "",
		"akari":      "",
	} {
		if got := planned(t, id); got != want {
			t.Errorf("%s: the bot plans %q, want %q", id, got, want)
		}
	}
}

// TestAffected: what the bots reckon a bolt does to a fight.
func TestAffected(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Player = Point{r.X + 1, r.Y + 1}
	g.HP, g.MaxHP = 80, 80
	m := place(g, kind("ryu2"), Point{r.X + 2, r.Y + 1})
	ms := []*Monster{m}
	fight := fightDeath(g, ms, g.HP, 1)
	if len(affected(g, ms, m, MagicWarp)) != 0 {
		t.Error("warped, it is still in the fight")
	}
	for _, e := range []MagicEffect{MagicSleep, MagicConfuse, MagicSlow, MagicLevel1, MagicHalf} {
		after := affected(g, ms, m, e)
		if c := fightDeath(g, after, g.HP, 1); c >= fight {
			t.Errorf("%s: chance %.3f, not under %.3f", e, c, fight)
		}
		if after[0] == m || m.Asleep != 0 || m.Def.ID != "ryu2" {
			t.Errorf("%s changed the monster itself", e)
		}
	}
	if after := affected(g, ms, m, MagicHaste); after[0] != m {
		t.Error("haste changes the fight")
	}
	// Asleep by a grass, the player is hit all the while.
	if c := drinkDeath(g, ms, g.Dungeon.Item("suimin")); c <= fight {
		t.Errorf("asleep: chance %.3f, not above %.3f", c, fight)
	}
}
