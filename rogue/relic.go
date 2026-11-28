package rogue

import (
	"math/rand/v2"
	"slices"
)

// Relics (神器, after Shiren 6; Shougo, 2026-11-28): now and then a weapon
// or a shield comes stronger than its kind, one in rules.txt 神器. A blue
// relic (青) is the stronger, by +5 to +10; a gold one (金), one in
// relicGold of them, by +3 to +8 only, but with one or two slots more and
// one to three seals in them. The higher the bonus, the rarer (bonus
// rolls: a weight of 6 for the lowest down to 1 for the highest). The
// upgrade value and the curse are rolled as for the others. A relic is
// shown with （青神器） or （金神器）, and costs twice (blue) or three
// times (gold) as much. Merged into another weapon or shield, it gives its
// seals and upgrade value as any (synthesis), not its being a relic; as
// the base of a synthesis, it stays one.
const (
	RelicBlue = 1
	RelicGold = 2
	// relicGold is one in how many relics are gold.
	relicGold = 4
)

// relicBonuses are the bonuses of the relics, lowest first.
var relicBonuses = map[int][2]int{RelicBlue: {5, 10}, RelicGold: {3, 8}}

// rollRelic makes it, a weapon or a shield just made, a relic, by the
// chance of the dungeon.
func (g *Game) rollRelic(rng *rand.Rand, it *Item) {
	n := g.Dungeon.Rules.Relic
	if n <= 0 || rng.IntN(n) != 0 {
		return
	}
	it.Relic = RelicBlue
	if rng.IntN(relicGold) == 0 {
		it.Relic = RelicGold
	}
	b := relicBonuses[it.Relic]
	it.RelicBonus = b[0] + weightedLow(rng, b[1]-b[0]+1)
	if it.Relic != RelicGold {
		return
	}
	it.MoreSlots += 1 + rng.IntN(2)
	var seals []*SealDef
	for _, s := range g.Dungeon.Seals {
		if (s.For == "" || s.For == it.Def.Category) && s.AsSeal > 0 && !slices.Contains(seals, s) {
			seals = append(seals, s)
		}
	}
	for range 1 + rng.IntN(3) {
		if len(seals) == 0 || len(it.Seals) >= it.slots() {
			break
		}
		it.Seals = append(it.Seals, seals[rng.IntN(len(seals))].Glyph)
	}
}

// weightedLow picks one of 0..n-1, n times as likely for 0 as for n-1.
func weightedLow(rng *rand.Rand, n int) int {
	total := n * (n + 1) / 2
	r := rng.IntN(total)
	for i := range n {
		r -= n - i
		if r < 0 {
			return i
		}
	}
	return n - 1
}

// relicLabel is what shows a relic in its name.
func relicLabel(it *Item) string {
	switch it.Relic {
	case RelicBlue:
		return "（青神器）"
	case RelicGold:
		return "（金神器）"
	}
	return ""
}

// relicPrice is the price of it in percents of its kind's.
func relicPrice(it *Item) int {
	return []int{100, 200, 300}[it.Relic]
}

// relicGlint tells the player, entering a room, that a relic lies in it
// (Shiren 6's キラーン), once a room a floor.
func (g *Game) relicGlint() {
	room := g.Level.RoomAt(g.Player)
	if room < 0 {
		return
	}
	if g.glintDepth != g.Depth {
		g.glinted, g.glintDepth = map[int]bool{}, g.Depth
	}
	if g.glinted[room] {
		return
	}
	g.glinted[room] = true
	for _, it := range g.Items {
		if it.Relic > 0 && g.Level.RoomAt(it.Pos) == room {
			g.say("何かが光った！")
			return
		}
	}
}
