package rogue

// The staffs of 2026-11-28 that send no bolt (Shougo).

// makeWall is the staff of 壁作り: a wall on the tile ahead, an empty
// floor or corridor (no one, no item, no stairs, no trap, not in a shop),
// which a blow of the player breaks again (breakWall).
func (g *Game) makeWall() {
	d := Directions[g.Facing]
	p := g.Player.Add(d)
	t := g.Level.At(p)
	if !g.clearFrom(g.Player, d) || (t != TileRoom && t != TileCorridor) || g.occupied(p) ||
		g.stairsAt(p) || g.TrapAt(p) != nil || g.inShop(p) {
		g.say("何も起こらなかった。")
		return
	}
	if _, ok := g.ItemAt(p); ok {
		g.say("何も起こらなかった。")
		return
	}
	if g.Level.Made == nil {
		g.Level.Made = map[Point]Tile{}
	}
	g.Level.Made[p] = t
	g.Level.set(p, TileRock)
	g.say("目の前に壁ができた。")
}

// breakWall breaks the wall a staff made on p, if there is one, and says
// whether it did.
func (g *Game) breakWall(p Point) bool {
	t, ok := g.Level.Made[p]
	if !ok || g.Level.At(p) != TileRock {
		return false
	}
	delete(g.Level.Made, p)
	g.Level.set(p, t)
	g.say("壁が崩れた。")
	return true
}

// leapTo is the staff of 飛びつき: the bolt flies ahead, and the player
// flies after it, to the tile before what it hit, a monster or a wall
// (Shougo: the other way round of 引き寄せ, walls too).
func (g *Game) leapTo() {
	d := Directions[g.Facing]
	to := g.Player
	for p := g.Player; g.clearFrom(p, d); {
		next := p.Add(d)
		if !g.Level.At(next).Walkable() || g.MonsterAt(next) != nil {
			break
		}
		p = next
		to = p
	}
	if to == g.Player {
		g.say("何も起こらなかった。")
		return
	}
	g.say("飛びついた！")
	g.shove(to)
}

// guide is the staff of 導き: the stairs of the floor are shown.
func (g *Game) guide() {
	g.markSeen(g.Stairs)
	g.say("階段の場所が分かった。")
}
