package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// AbilityName names a special ability of monsters (monsters.txt 能力).
type AbilityName string

// The abilities. Each takes its own numbers, written after its name and
// colons (怒り:33:150); abilityDefs says how many and what they mean.
const (
	// AbilityAnger: at or below a share of its max HP (a percent), its
	// attack goes up (a percent of it), once.
	AbilityAnger AbilityName = "怒り"
	// AbilityCritical: a share of its blows (a percent) are critical
	// blows, their damage multiplied (a percent).
	AbilityCritical AbilityName = "痛恨"
	// AbilityErratic: a share of its actions (a percent) are a step in a
	// random direction, even next to the player.
	AbilityErratic AbilityName = "ふらふら"
	// AbilityCharge: with the player on one of its lines within a reach
	// and not next to it, it rushes up, hurts the first creature in the
	// way (a damage) and blows it away.
	AbilityCharge AbilityName = "突進"
	// AbilityPull: with the player on one of its lines within a reach (0:
	// anywhere in its room) and not next to it, it pulls the player next
	// to it by a chance (a percent), and then hits it (1) or not (0).
	AbilityPull AbilityName = "引き寄せ"
	// AbilityShoot: with the player in sight on one of its lines within a
	// reach and not next to it, it shoots an arrow (an item's ID, of the
	// kind 矢) at it by a chance (a percent), even with a creature in the
	// way: the arrow hits the first creature it meets, or with pierce (1)
	// every one. An arrow that hits no one falls where it stops, unless
	// it pierces.
	AbilityShoot AbilityName = "射撃"
	// AbilityMagic: with the player in sight on one of its lines within a
	// reach and not next to it, it waves a staff at it by a chance (a
	// percent): a magic bolt of one of its effects (MagicEffect), at
	// random, which takes the first creature it meets.
	AbilityMagic AbilityName = "魔法"
	// AbilityMimic: on a new floor it hides as an item, as stairs (by a
	// chance, a percent) or as goods of a shop (by another), until it
	// is found out: how, its depth says (mimic.go).
	AbilityMimic AbilityName = "擬態"
	// AbilitySteal: next to the player, by a chance (a percent), it
	// steals instead of a blow: money (0: half the purse) or items (1:
	// so many, carried, and with 1 last, equipped ones too). Then it
	// warps away and flees, and never steals again; defeated, it drops
	// what it stole (theft.go).
	AbilitySteal AbilityName = "盗み"
	// AbilitySplit: a blow of the player that leaves it with 2 HP or
	// more splits it by a chance (a percent): its HP halves, and another
	// with that HP appears next to it (mud.go).
	AbilitySplit AbilityName = "分裂"
	// AbilityRot: next to the player, by a chance (a percent), it rots
	// so many of the onigiri carried instead of a blow; with none, and
	// 1 last, it turns an item that is no equipment into a rotten
	// onigiri (mud.go).
	AbilityRot AbilityName = "腐らせる"
	// AbilitySynthesis: it swallows the items the player throws at it,
	// so many, and merges them in its belly as a pot of synthesis does,
	// and a material into a weapon or a shield too (異種合成); next to
	// the player, by a chance (a percent, 0 for never), it sucks a
	// weapon or a shield the player carries in instead of a blow, an
	// equipped one too. Defeated, it drops what it holds (synth.go).
	AbilitySynthesis AbilityName = "合成"
)

// MagicEffect is an effect of the bolt of AbilityMagic.
type MagicEffect string

// The effects of the bolts of monsters.
const (
	MagicWarp    MagicEffect = "ワープ"
	MagicHaste   MagicEffect = "倍速"
	MagicBlow    MagicEffect = "吹き飛ばし"
	MagicConfuse MagicEffect = "混乱"
	MagicSlow    MagicEffect = "鈍足"
	MagicSleep   MagicEffect = "睡眠"
	MagicSeal    MagicEffect = "封印"
	MagicHalf    MagicEffect = "HP1/2"
	MagicQuarter MagicEffect = "HP1/4"
	MagicLevel1  MagicEffect = "レベル-1"
	MagicLevel3  MagicEffect = "レベル-3"
)

// magicEffects are the effects, in a fixed order, for the error that lists
// them.
var magicEffects = []MagicEffect{
	MagicWarp, MagicHaste, MagicBlow, MagicConfuse, MagicSlow, MagicSleep,
	MagicSeal, MagicHalf, MagicQuarter, MagicLevel1, MagicLevel3,
}

// parseMagic reads the effects of AbilityMagic: separated by commas.
func parseMagic(s string) ([]MagicEffect, error) {
	var es []MagicEffect
	for _, f := range strings.Split(s, ",") {
		e := MagicEffect(f)
		if !slices.Contains(magicEffects, e) {
			names := make([]string, len(magicEffects))
			for i, n := range magicEffects {
				names[i] = string(n)
			}
			return nil, fmt.Errorf("unknown effect %q (want %s)", f, strings.Join(names, ", "))
		}
		es = append(es, e)
	}
	return es, nil
}

// abilityDef is what an ability takes: its numbers, each in a range.
type abilityDef struct {
	// values are the names of its numbers (for the errors), mins and
	// maxs their ranges.
	values     []string
	mins, maxs []int
	// text is the place of the value that is text (not a number), plus 1;
	// 0 for none.
	text int
}

var abilityDefs = map[AbilityName]abilityDef{
	AbilityAnger:     {[]string{"HP %", "attack %"}, []int{1, 100}, []int{99, 1000}, 0},
	AbilityCritical:  {[]string{"chance %", "damage %"}, []int{1, 100}, []int{100, 1000}, 0},
	AbilityErratic:   {[]string{"chance %"}, []int{1}, []int{100}, 0},
	AbilityCharge:    {[]string{"reach", "damage"}, []int{2, 0}, []int{maxReach, 999}, 0},
	AbilityPull:      {[]string{"reach", "chance %", "hit"}, []int{0, 1, 0}, []int{maxReach, 100, 1}, 0},
	AbilityShoot:     {[]string{"reach", "chance %", "arrow", "pierce"}, []int{2, 1, 0, 0}, []int{maxReach, 100, 0, 1}, 3},
	AbilityMagic:     {[]string{"reach", "chance %", "effects"}, []int{2, 1, 0}, []int{maxReach, 100, 0}, 3},
	AbilityMimic:     {[]string{"depth", "stairs %", "shop %"}, []int{mimicShallow, 0, 0}, []int{mimicDeep, 100, 100}, 0},
	AbilitySplit:     {[]string{"chance %"}, []int{1}, []int{100}, 0},
	AbilityRot:       {[]string{"chance %", "onigiri", "any item"}, []int{1, 1, 0}, []int{100, 5, 1}, 0},
	AbilitySynthesis: {[]string{"items", "suck %"}, []int{1, 0}, []int{9, 100}, 0},
	AbilitySteal:     {[]string{"chance %", "what", "items", "equipped"}, []int{1, stealGold, 0, 0}, []int{100, stealItems, 5, 1}, 0},
}

// maxReach is the farthest reach of an ability along a line.
const maxReach = 20

// abilityNames are the abilities in a fixed order, for the error that
// lists them.
var abilityNames = []AbilityName{AbilityAnger, AbilityCritical, AbilityErratic, AbilityCharge, AbilityPull, AbilityShoot, AbilityMagic, AbilityMimic, AbilitySteal, AbilitySplit, AbilityRot, AbilitySynthesis}

// lineAbilities are the abilities that reach the player from afar along
// a line, their reach their first number.
var lineAbilities = []AbilityName{AbilityCharge, AbilityPull, AbilityShoot, AbilityMagic}

// Ability is an ability of a kind of monster, with its numbers (0 in the
// place of its text) and its text, if it takes one: the ID of an item
// (AbilityShoot), or Effects (AbilityMagic).
type Ability struct {
	Name    AbilityName
	Values  []int
	Item    string
	Effects []MagicEffect
	// file and line are where it is read, for the check of the item.
	file string
	line int
}

// parseAbilities reads a 能力 cell: abilities separated by spaces, "-"
// for none.
func parseAbilities(s string) ([]Ability, error) {
	if s == "" || s == "-" {
		return nil, nil
	}
	var as []Ability
	seen := map[AbilityName]bool{}
	for _, f := range strings.Fields(s) {
		parts := strings.Split(f, ":")
		name := AbilityName(parts[0])
		def, ok := abilityDefs[name]
		if !ok {
			names := make([]string, len(abilityNames))
			for i, n := range abilityNames {
				names[i] = string(n)
			}
			return nil, fmt.Errorf("unknown ability %q (want %s)", name, strings.Join(names, ", "))
		}
		if seen[name] {
			return nil, fmt.Errorf("the ability %s twice", name)
		}
		seen[name] = true
		if len(parts)-1 != len(def.values) {
			return nil, fmt.Errorf("%s: want %d numbers (%s), not %d", name, len(def.values), strings.Join(def.values, ", "), len(parts)-1)
		}
		a := Ability{Name: name}
		for i, v := range parts[1:] {
			if i == def.text-1 {
				a.Values = append(a.Values, 0)
				if name != AbilityMagic {
					a.Item = v
					continue
				}
				es, err := parseMagic(v)
				if err != nil {
					return nil, fmt.Errorf("%s: %v", name, err)
				}
				a.Effects = es
				continue
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < def.mins[i] || n > def.maxs[i] {
				return nil, fmt.Errorf("%s: %s wants %d to %d, not %q", name, def.values[i], def.mins[i], def.maxs[i], v)
			}
			a.Values = append(a.Values, n)
		}
		as = append(as, a)
	}
	return as, nil
}

// ability returns the ability of m named n, if it has it (and is not
// sealed).
func (m *Monster) ability(n AbilityName) (Ability, bool) {
	if m.Sealed {
		return Ability{}, false
	}
	for _, a := range m.Def.Abilities {
		if a.Name == n {
			return a, true
		}
	}
	return Ability{}, false
}

// wounded is m hurt and still standing: it may get angry.
func (g *Game) wounded(m *Monster) {
	a, ok := m.ability(AbilityAnger)
	if !ok || m.Enraged || m.HP*100 > m.MaxHP*a.Values[0] {
		return
	}
	m.Enraged = true
	m.Attack = m.Attack * a.Values[1] / 100
	if g.sees(g.Player, m.Pos) {
		g.say(m.Name + "は怒った！")
	}
}

// critical returns n, the damage of a blow of m, made a critical blow by
// chance.
func (g *Game) critical(m *Monster, n int, shown bool) int {
	a, ok := m.ability(AbilityCritical)
	if !ok || g.rng.IntN(100) >= a.Values[0] {
		return n
	}
	if shown {
		g.say("痛恨の一撃！")
	}
	return n * a.Values[1] / 100
}

// erratic says whether m steps at random instead of acting, and makes
// the step.
func (g *Game) erratic(m *Monster) bool {
	a, ok := m.ability(AbilityErratic)
	if !ok || g.rng.IntN(100) >= a.Values[0] {
		return false
	}
	var free []Point
	for _, d := range around {
		p := m.Pos.Add(d)
		if g.Level.At(p).Walkable() && !g.occupied(p) && g.clearFrom(m.Pos, d) {
			free = append(free, p)
		}
	}
	if len(free) > 0 {
		m.Pos = free[g.rng.IntN(len(free))]
	}
	return true
}

// dozes says, by chance, whether m is asleep on a new floor: a third of
// the monsters, or as many as its kind says (monsters.txt 眠り).
func (m *Monster) dozes(rng *rand.Rand) bool {
	if m.Def.Asleep < 0 {
		return rng.IntN(3) == 0
	}
	return rng.IntN(100) < m.Def.Asleep
}

// lineTo returns the direction from a to b and how far, if b is on one of
// the eight lines from a, 2 to reach tiles away, with nothing but
// creatures in between (no rock, no corner of rock).
func (g *Game) lineTo(a, b Point, reach int) (Point, int, bool) {
	v := b.Sub(a)
	n := chebyshev(a, b)
	if n < 2 || n > reach || v.X != 0 && v.Y != 0 && abs(v.X) != abs(v.Y) {
		return Point{}, 0, false
	}
	d := Point{sign(v.X), sign(v.Y)}
	for p := a; p != b; p = p.Add(d) {
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Walkable() {
			return Point{}, 0, false
		}
	}
	return d, n, true
}

// reachAbility uses an ability of m that reaches the player from afar (a
// charge, a pull); it says whether m did.
func (g *Game) reachAbility(m *Monster) bool {
	if a, ok := m.ability(AbilityCharge); ok && g.charge(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityPull); ok && g.pull(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityShoot); ok && g.shootAt(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityMagic); ok && g.castAt(m, a) {
		return true
	}
	return false
}

// charge is m rushing along its line to the player: it stops next to the
// first creature in the way, hurts it and blows it away.
func (g *Game) charge(m *Monster, a Ability) bool {
	d, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0])
	if !ok {
		return false
	}
	for next := m.Pos.Add(d); next != g.Player && g.MonsterAt(next) == nil; next = m.Pos.Add(d) {
		m.Pos = next
	}
	g.say(m.Name + "が突進してきた！")
	n := a.Values[1]
	if f := g.MonsterAt(m.Pos.Add(d)); f != nil {
		f.Dozing = false
		f.HP -= n
		g.say(fmt.Sprintf("%sに%dのダメージ。", f.Name, n))
		if f.HP <= 0 {
			g.kill(f, m, false, true)
			return true
		}
		g.wounded(f)
		g.say(f.Name + "は吹き飛ばされた。")
		g.fling(f, d, false)
		return true
	}
	if g.playerHurt(n, m) {
		return true
	}
	g.say("吹き飛ばされた。")
	g.flingPlayer(d, m)
	return true
}

// pull is m pulling the player next to it: along its line (stopping before
// a creature in the way), or from anywhere in its room.
func (g *Game) pull(m *Monster, a Ability) bool {
	reach, chance, hit := a.Values[0], a.Values[1], a.Values[2] == 1
	to := g.Player
	if reach == 0 {
		i := g.Level.RoomAt(m.Pos)
		if i < 0 || i != g.Level.RoomAt(g.Player) || chebyshev(m.Pos, g.Player) <= 1 {
			return false
		}
		best := -1
		for _, d := range around {
			p := m.Pos.Add(d)
			if !g.Level.At(p).Walkable() || g.occupied(p) || !g.clearFrom(m.Pos, d) {
				continue
			}
			if n := chebyshev(p, g.Player); best < 0 || n < best {
				to, best = p, n
			}
		}
		if best < 0 {
			return false
		}
	} else {
		d, _, ok := g.lineTo(m.Pos, g.Player, reach)
		if !ok {
			return false
		}
		for next := to.Sub(d); next != m.Pos && !g.occupied(next); next = to.Sub(d) {
			to = next
		}
		if to == g.Player {
			return false
		}
	}
	if g.rng.IntN(100) >= chance {
		return false
	}
	g.say(m.Name + "に引き寄せられた！")
	g.shove(to)
	if hit && g.Over == "" && chebyshev(m.Pos, g.Player) == 1 {
		g.hitPlayer(m)
	}
	return true
}

// playerHurt hurts the player by n, done by m; it says whether the player
// fell.
func (g *Game) playerHurt(n int, m *Monster) bool {
	g.HP -= n
	g.say(fmt.Sprintf("%dのダメージ。", n))
	g.event("hurt %d %s", n, m.Def.ID)
	if g.HP <= 0 {
		g.HP = 0
		g.end(m.Name+"に倒された", false)
		return true
	}
	return false
}

// flingPlayer sends the player flying by d, up to blowAwayTiles, as m did;
// hitting a wall or a creature hurts it. A trap where it lands goes off.
func (g *Game) flingPlayer(d Point, m *Monster) {
	p := g.Player
	for range blowAwayTiles {
		next := p.Add(d)
		if g.clearFrom(p, d) && g.Level.At(next).Walkable() && !g.occupied(next) {
			p = next
			continue
		}
		g.say("ぶつかった。")
		if g.playerHurt(blowAwayDamage, m) {
			return
		}
		break
	}
	if p != g.Player {
		g.shove(p)
	}
}

// shootAt is m shooting its arrow along its line to the player, which it
// sees.
func (g *Game) shootAt(m *Monster, a Ability) bool {
	d, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0])
	if !ok || !g.sees(m.Pos, g.Player) || g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	def := g.Dungeon.Item(a.Item)
	pierce := a.Values[3] == 1
	r := g.Dungeon.Rules
	attack := m.Attack/2 + def.Strength
	g.say(m.Name + "は" + def.Name + "を撃った！")
	p := m.Pos
	for range throwRange {
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Walkable() {
			break
		}
		p = p.Add(d)
		if p == g.Player {
			if !hits(g.rng, r.MonsterThrowRate) {
				g.say("矢は当たらなかった。")
				continue
			}
			n := damage(g.rng, attack, g.Defense(), r.DefenseK)
			g.say(def.Name + "が当たった。")
			if g.playerHurt(n, m) || !pierce {
				return true
			}
			continue
		}
		f := g.MonsterAt(p)
		if f == nil {
			continue
		}
		shown := g.sees(g.Player, p)
		f.Dozing = false
		if !hits(g.rng, r.MonsterThrowRate) {
			if shown {
				g.say(def.Name + "は" + f.Name + "に当たらなかった。")
			}
			continue
		}
		n := damage(g.rng, attack, f.Defense, r.DefenseK)
		f.HP -= n
		if shown {
			g.say(fmt.Sprintf("%sが%sに当たった。%dのダメージ。", def.Name, f.Name, n))
		}
		if f.HP > 0 {
			g.wounded(f)
		} else {
			g.kill(f, m, false, shown)
		}
		if !pierce {
			return true
		}
	}
	if !pierce {
		g.land(&Item{Def: def, Glyph: def.Glyph, Count: 1}, p)
	}
	return true
}

// rankUpBy is the monster m ranking up for a defeat, if it is an enemy of
// the floors.
func (g *Game) rankUpBy(m *Monster, shown bool) {
	if m.Ally || m.Def.Kind != "" {
		return
	}
	name := m.Name
	g.rankUp(m)
	if shown && m.Name != name {
		g.say(name + "は" + m.Name + "になった！")
	}
}
