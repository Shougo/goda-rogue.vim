package rogue

// tunnel is the staff of トンネル (Shiren): its bolt digs the rock to the
// player's front into a corridor, n tiles of it (10), up to the edge of
// the floor, a shop, or a creature. Going diagonally, it digs the tile
// beside too, to walk the tunnel.
func (g *Game) tunnel(n int) {
	d := Directions[g.Facing]
	dug := 0
	dig := func(p Point) bool {
		if g.Level.At(p) != TileRock {
			return true
		}
		if p.X <= 0 || p.Y <= 0 || p.X >= g.Level.W-1 || p.Y >= g.Level.H-1 || g.Shop != nil && g.Shop.Area.Around(p) {
			return false
		}
		g.Level.set(p, TileCorridor)
		dug++
		return true
	}
	for p := g.Player; dug < n; {
		if !g.clearFrom(p, d) && !dig(p.Add(Point{d.X, 0})) {
			break
		}
		next := p.Add(d)
		if next == g.Player || g.MonsterAt(next) != nil || !dig(next) {
			break
		}
		p = next
	}
	if dug == 0 {
		g.say("何も起こらなかった。")
		return
	}
	g.say("壁が掘られて、通路ができた。")
}
