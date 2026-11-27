package rogue

import (
	"fmt"
	"slices"
)

// split is the mud m, hit by a blow of the player that left it with 2 HP
// or more, splitting by its ability (AbilitySplit): its HP halves, and
// another with that HP appears next to it. The new one gives nothing
// when defeated.
func (g *Game) split(m *Monster) {
	a, ok := m.ability(AbilitySplit)
	if !ok || m.HP < 2 || g.rng.IntN(100) >= a.Values[0] {
		return
	}
	var free []Point
	for _, d := range around {
		p := m.Pos.Add(d)
		if g.Level.At(p).Walkable() && !g.occupied(p) && p != g.Player && g.clearFrom(m.Pos, d) {
			free = append(free, p)
		}
	}
	if len(free) == 0 {
		return
	}
	m.HP /= 2
	c := newMonster(m.Def)
	c.Pos, c.HP, c.Spawned, c.Rank, c.Name = free[g.rng.IntN(len(free))], m.HP, true, m.Rank, m.Name
	g.Monsters = append(g.Monsters, c)
	g.say(m.Name + "が分裂した！")
}

// rotted returns the kind of the rotten onigiri (an item of the effect
// 腐った), or nil.
func (d *Dungeon) rotted() *ItemDef {
	for _, def := range d.Items {
		if def.Effect == EffectRotten {
			return def
		}
	}
	return nil
}

// onigiri says whether it is food that rots: food of the effect 満腹.
func (it *Item) onigiri() bool {
	return it.Def.Category == CategoryFood && it.Def.Effect == EffectFood
}

// rot turns the carried item it into a rotten onigiri, unless a blessing
// takes it.
func (g *Game) rot(it *Item, rotten *ItemDef) {
	if g.shielded(it) {
		return
	}
	name := g.Name(it)
	*it = Item{Def: rotten, Glyph: rotten.Glyph, ForSale: it.ForSale}
	g.say(name + "が腐ってしまった！")
}

// rotOnigiri rots up to n of the onigiri the player carries (not those
// in pots), and says how many it rotted.
func (g *Game) rotOnigiri(n int) int {
	rotten := g.Dungeon.rotted()
	if rotten == nil {
		return 0
	}
	done := 0
	for _, it := range g.Inventory {
		if done == n {
			break
		}
		if it.onigiri() {
			g.rot(it, rotten)
			done++
		}
	}
	return done
}

// spoil is the mud m next to the player rotting its onigiri, by its
// ability (AbilityRot), instead of a blow; with none, and the ability
// says so, an item that is no equipment (nor money) turns into one. It
// says whether it did something.
func (g *Game) spoil(m *Monster, a Ability) bool {
	if g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	if g.rotOnigiri(a.Values[1]) > 0 {
		return true
	}
	rotten := g.Dungeon.rotted()
	if a.Values[2] == 0 || rotten == nil {
		return false
	}
	var can []*Item
	for _, it := range g.Inventory {
		if !it.equipment() && it.Def.Category != CategoryMoney && it.Def != rotten {
			can = append(can, it)
		}
	}
	if len(can) == 0 {
		return false
	}
	g.rot(can[g.rng.IntN(len(can))], rotten)
	return true
}

// rottenStates are the states a rotten onigiri may bring, one at random.
var rottenStates = []MagicEffect{MagicConfuse, MagicSlow, MagicSleep}

// eatRotten is eating a rotten onigiri: n of fullness, and a bad state.
func (g *Game) eatRotten(n int) {
	g.fill(n)
	g.say("お腹がふくれたが、腐っていた……。")
	e := rottenStates[g.rng.IntN(len(rottenStates))]
	if g.prevents(e) {
		return
	}
	g.States.take(e)
	g.say(stateTexts[e].player)
}

// rotTrap is the trap of rot: every onigiri carried rots.
func (g *Game) rotTrap() {
	if g.rotOnigiri(len(g.Inventory)) == 0 {
		g.say("何も起こらなかった。")
	}
}

// keepOnigiri sorts the bots' food: the rotten last.
func keepOnigiri(foods []*Item) []*Item {
	slices.SortStableFunc(foods, func(a, b *Item) int {
		return boolInt(a.Def.Effect == EffectRotten) - boolInt(b.Def.Effect == EffectRotten)
	})
	return foods
}

// boolInt is 1 for true.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rotText tells what the rot of mud does.
func rotText(v []int) string {
	s := fmt.Sprintf("隣にいると%d%%でおにぎりを%d個腐らせる", v[0], v[1])
	if v[2] == 1 {
		s += "（おにぎりがなければ、装備以外の道具を腐ったおにぎりに変える）"
	}
	return s
}
