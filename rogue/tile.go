package rogue

// tileNeed is what a tile must be free of for something to be put on it.
type tileNeed uint

const (
	// needNoCreature: neither the player nor a monster stands on it.
	needNoCreature tileNeed = 1 << iota
	// needNoItem: no item lies on it.
	needNoItem
	// needNoStairs: it is not the stairs, nor what looks like them.
	needNoStairs
	// needNoTrap: no trap is hidden on it.
	needNoTrap
	// needNoShop: it is not in the shop.
	needNoShop
	// needUnseen: the player does not see it, and is more than
	// unseenReach tiles away (for what appears while the player is on
	// the floor).
	needUnseen
)

const (
	// needFree: nothing stands or lies on it (Game.free).
	needFree = needNoCreature | needNoItem | needNoStairs
	// needHidden: for what is hidden on the floor as an item or as
	// stairs (a mimic): free, no trap, not in the shop.
	needHidden = needFree | needNoTrap | needNoShop
)

// unseenReach is how far from the player a tile out of its sight must be
// for needUnseen.
const unseenReach = 4

// findTries is how many tiles find tries.
const findTries = 100

// blocked says whether p has one of the things of n on it.
func (g *Game) blocked(p Point, n tileNeed) bool {
	switch {
	case n&needNoCreature != 0 && g.occupied(p),
		n&needNoStairs != 0 && g.stairsAt(p),
		n&needNoTrap != 0 && g.TrapAt(p) != nil,
		n&needNoShop != 0 && g.inShop(p),
		n&needUnseen != 0 && (g.sees(g.Player, p) || chebyshev(p, g.Player) <= unseenReach):
		return true
	}
	if n&needNoItem != 0 {
		if _, ok := g.ItemAt(p); ok {
			return true
		}
	}
	return false
}

// fits says whether something that needs n can be put on p: a walkable
// tile with none of the things of n on it, not on an island (flood).
func (g *Game) fits(p Point, n tileNeed) bool {
	return g.Level.At(p).Walkable() && !g.Level.Island[p] && !g.blocked(p, n)
}

// free says whether nothing stands or lies on p.
func (g *Game) free(p Point) bool { return !g.blocked(p, needFree) }

// find returns a tile that fits n (and also, if given), among those pick
// draws: the first of findTries. Every search for a tile at random comes
// here.
func (g *Game) find(pick func() Point, n tileNeed, also ...func(Point) bool) (Point, bool) {
tries:
	for range findTries {
		p := pick()
		if !g.fits(p, n) {
			continue
		}
		for _, f := range also {
			if !f(p) {
				continue tries
			}
		}
		return p, true
	}
	return Point{}, false
}
