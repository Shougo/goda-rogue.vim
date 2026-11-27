package rogue

import (
	"fmt"
	"math"
)

// 焼きおにぎり (Shougo, 2026-11-27): an onigiri the player carries (any:
// big or rotten too, not one in a pot) becomes a grilled one in a blast
// (a mine, a monster exploding, a bracelet of 爆発) or in the fire of
// 火炎草 thrown at it, and one thrown at a monster that burns things
// lands grilled.

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

// fireAround is how far the damage of fire spreads around 強さ of 火炎草
// (drunk; thrown, half of it): 70 to 80, and 35 to 45 (Shiren 6).
const fireAround = 5

// fireDamage is the damage of the fire of 火炎草 of strength n, drunk or
// (thrown) not.
func (g *Game) fireDamage(n int, drunk bool) int {
	if !drunk {
		// (About half: 40 of 75.)
		n = (n + fireAround) / 2
	}
	return between(g.rng, max(n-fireAround, 1), n+fireAround)
}

// breatheFire is 火炎草 drunk (strength n): the player breathes fire to
// its front, which burns the first monster on its line (defense does not
// lower it).
func (g *Game) breatheFire(n int) {
	g.say("口から炎を吐いた！")
	d := Directions[g.Facing]
	for p, i := g.Player, 0; i < throwRange && g.clearFrom(p, d); i++ {
		p = p.Add(d)
		if !g.Level.At(p).Open() {
			break
		}
		if m := g.MonsterAt(p); m != nil {
			g.burnMonster(m, g.fireDamage(n, true))
			return
		}
	}
	g.say("炎は何にも当たらなかった。")
}

// burnMonster is fire (火炎草) burning m by n: not a blow, the player's
// attack down does not lower it.
func (g *Game) burnMonster(m *Monster, n int) {
	g.provoke(m)
	n = m.harm(n)
	m.HP -= n
	m.Bound = 0
	m.Dozing = false
	g.say(fmt.Sprintf("%sは炎に包まれた。%dのダメージ。", m.Name, n))
	if m.HP > 0 {
		g.wounded(m)
		return
	}
	g.kill(m, nil, true, true)
}

// burnPlayer is fire (火炎草 thrown by m) burning the player by n; the
// onigiri carried are grilled. It says whether the player fell.
func (g *Game) burnPlayer(n int, m *Monster) bool {
	g.say("炎に包まれた！")
	if s := g.sealShare(SealFireGuard); s > 0 && n > 0 {
		n = max(int(math.Round(float64(n)*(1-s))), 1)
	}
	if g.playerHurt(n, m) {
		return true
	}
	g.grillCarried()
	return false
}
