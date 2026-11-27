package rogue

// 焼きおにぎり (Shougo, 2026-11-27): an onigiri the player carries (any:
// big or rotten too, not one in a pot) becomes a grilled one in a blast
// (a mine, a monster exploding, a bracelet of 爆発), and one thrown at a
// monster that burns things lands grilled.

// grillable says whether it is an onigiri that grills: food of 満腹 or
// rotten.
func grillable(it *Item) bool {
	return it.Def.Category == CategoryFood && (it.Def.Effect == EffectFood || it.Def.Effect == EffectRotten)
}

// grilledDef returns the kind of the grilled onigiri, or nil.
func (d *Dungeon) grilledDef() *ItemDef {
	for _, def := range d.Items {
		if def.Category == CategoryFood && def.Effect == EffectGrilled {
			return def
		}
	}
	return nil
}

// grill makes it a grilled onigiri, keeping its blessing or curse.
func (g *Game) grill(it *Item) {
	def := g.Dungeon.grilledDef()
	*it = Item{Def: def, Glyph: def.Glyph, Pos: it.Pos, Blessed: it.Blessed, Cursed: it.Cursed, CurseTold: it.CurseTold}
}

// grillCarried is a blast on the player: the onigiri it carries are
// grilled.
func (g *Game) grillCarried() {
	if g.Dungeon.grilledDef() == nil {
		return
	}
	n := 0
	for _, it := range g.Inventory {
		if grillable(it) {
			g.grill(it)
			n++
		}
	}
	if n > 0 {
		g.say("持っていたおにぎりが焼きおにぎりになった。")
	}
}
