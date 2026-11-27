package rogue

import (
	"fmt"
	"math"
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
	// AbilityCurse: next to the player, by a chance (a percent), it
	// curses what the player carries instead of a blow: the weapon or
	// the shield equipped (0), those or the bracelet (1), or any item
	// (2), so many of them. A scroll of おはらい read near it destroys
	// it (curser.go).
	AbilityCurse AbilityName = "呪い"
	// AbilityHunger: next to the player, by a chance (a percent), it
	// takes so much of its fullness instead of a blow, or (the last
	// number above 0) of its max fullness. Food thrown at it defeats it
	// (ghost.go).
	AbilityHunger AbilityName = "飢え"
	// AbilityStill: it never moves by itself (still.go).
	AbilityStill AbilityName = "不動"
	// AbilityHold: next to the player, by a chance (a percent), a blow
	// that hits holds it too: it cannot step away for so many turns, or
	// until the monster is gone or no more next to it (still.go).
	AbilityHold AbilityName = "捕縛"
	// AbilityBurn: what is thrown or shot at it burns, with no effect;
	// only blows hurt it (still.go).
	AbilityBurn AbilityName = "燃える"
	// AbilityHeal: by a chance (a percent) of its actions, it heals
	// another hurt monster it sees by so many HP (support.go).
	AbilityHeal AbilityName = "回復"
	// AbilityCheer: by a chance (a percent) of its actions, it ranks up
	// another monster of the floors it sees (support.go).
	AbilityCheer AbilityName = "鼓舞"
	// AbilityRevive: defeated the first time, it gets up again with a
	// share (a percent) of its max HP (undead.go).
	AbilityRevive AbilityName = "復活"
	// AbilityExplode: hurt down to a share (a percent) of its max HP or
	// below, it explodes at its next action: every creature next to it
	// loses a share (a percent) of its HP, the items next to it burn, and
	// it is gone, giving nothing (undead.go).
	AbilityExplode AbilityName = "爆発"
	// AbilityPack: it comes with so many more of its kind next to it,
	// on a new floor and when it appears later (pack.go).
	AbilityPack AbilityName = "群れ"
	// AbilityFlee: it keeps away from the player it sees, and fights
	// only when it cannot; the other monsters that see it hunt it
	// before the player (hunt), and rank up by killing it (pack.go).
	AbilityFlee AbilityName = "逃げる"
	// AbilityTrap: by a chance (a percent) of its actions, out of the
	// player's reach, it hides a trap of the floor's table where it
	// stands; defeated, it leaves one there (pack.go).
	AbilityTrap AbilityName = "罠"
	// AbilityLong: it hits the player two tiles away on one of its
	// lines, with nothing in between (reach.go).
	AbilityLong AbilityName = "長柄"
	// AbilityBlink: with the player in sight and not next to it, by a
	// chance (a percent) of its actions, it appears next to the player
	// (reach.go).
	AbilityBlink AbilityName = "瞬間移動"
	// AbilityKnight: it moves as a knight of shogi or chess, over what is
	// in the way; with the player a knight's move away, it swoops on it
	// for so much damage, which defense does not lower (reach.go).
	AbilityKnight AbilityName = "桂馬"
	// AbilityInvisible: the player does not see it, unless eye drops
	// cleared its eyes on the floor (shade.go).
	AbilityInvisible AbilityName = "透明"
	// AbilityWalls: it goes through rock, straight to the player, wherever
	// it is (shade.go).
	AbilityWalls AbilityName = "壁抜け"
	// AbilityThrow: standing on an item, with the player on one of its
	// lines within a reach (anywhere within it in sight, with any: 1),
	// by a chance (a percent), it throws the item at the player; it goes
	// for the items it sees before the player (urchin.go).
	AbilityThrow AbilityName = "投げつけ"
	// AbilityReflect: by a chance (a percent), the bolts of staffs
	// waved at it go back to the player; with more (what: 1), the
	// arrows too; with 2, any item thrown at it (urchin.go).
	AbilityReflect AbilityName = "反射"
	// AbilityRuin: next to the player, by a chance (a percent), instead
	// of a blow, it ruins an item carried (what: 0 empties a staff, 1
	// takes a grass or a scroll away, 2 makes a pot a plain pot, 3
	// any of those); the seal 盗みよけ wards it off as a theft
	// (urchin.go).
	AbilityRuin AbilityName = "荒らし"
	// AbilityRust: next to the player, it never attacks: by a chance (a
	// percent), it rusts the weapon or the shield equipped by so much
	// (how: 0 one, 1 both, 2 one or, half of the time, a seal taken
	// away; a seal of サビよけ first), and otherwise watches (urchin.go).
	AbilityRust AbilityName = "サビ"
	// AbilityAfflict: by a chance (a percent), it brings a state to the
	// player (what: 0 blind, 1 asleep, 2 confused, 3 its attack or its
	// defense down) for so many turns: next to it, instead of a blow, or
	// (with sight: 1) from anywhere it sees the player (afflict.go).
	AbilityAfflict AbilityName = "状態"
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
	MagicBlind   MagicEffect = "目つぶし"
	// The bolts of the items that came later (2026-11-28): the staff of
	// かなしばり, しあわせ草, 無敵草, 毒草.
	MagicBind    MagicEffect = "かなしばり"
	MagicLevelUp MagicEffect = "レベル+1"
	MagicUnhurt  MagicEffect = "無敵"
	MagicPoison  MagicEffect = "毒"
)

// magicEffects are the effects, in a fixed order, for the error that lists
// them.
var magicEffects = []MagicEffect{
	MagicWarp, MagicHaste, MagicBlow, MagicConfuse, MagicSlow, MagicSleep,
	MagicSeal, MagicHalf, MagicQuarter, MagicLevel1, MagicLevel3, MagicBlind,
	MagicBind, MagicLevelUp, MagicUnhurt, MagicPoison,
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
	AbilityStill:     {nil, nil, nil, 0},
	AbilityHold:      {[]string{"chance %", "turns"}, []int{1, 1}, []int{100, 20}, 0},
	AbilityBurn:      {nil, nil, nil, 0},
	AbilityHeal:      {[]string{"chance %", "HP"}, []int{1, 1}, []int{100, 999}, 0},
	AbilityCheer:     {[]string{"chance %"}, []int{1}, []int{100}, 0},
	AbilityPack:      {[]string{"more"}, []int{1}, []int{8}, 0},
	AbilityFlee:      {nil, nil, nil, 0},
	AbilityLong:      {nil, nil, nil, 0},
	AbilityBlink:     {[]string{"chance %"}, []int{1}, []int{100}, 0},
	AbilityKnight:    {[]string{"damage"}, []int{1}, []int{999}, 0},
	AbilityInvisible: {nil, nil, nil, 0},
	AbilityWalls:     {nil, nil, nil, 0},
	AbilityThrow:     {[]string{"reach", "chance %", "any"}, []int{2, 1, 0}, []int{maxReach, 100, 1}, 0},
	AbilityReflect:   {[]string{"chance %", "what"}, []int{1, reflectStaffs}, []int{100, reflectAll}, 0},
	AbilityRuin:      {[]string{"chance %", "what"}, []int{1, ruinStaff}, []int{100, ruinAny}, 0},
	AbilityRust:      {[]string{"chance %", "rust", "how"}, []int{1, 1, rustOne}, []int{100, 9, rustOrSeal}, 0},
	AbilityAfflict:   {[]string{"chance %", "sight", "what", "turns"}, []int{1, 0, afflictBlind, 1}, []int{100, 1, afflictWeaken, 99}, 0},
	AbilityTrap:      {[]string{"chance %"}, []int{1}, []int{100}, 0},
	AbilityRevive:    {[]string{"HP %"}, []int{1}, []int{100}, 0},
	AbilityExplode:   {[]string{"HP %", "damage %"}, []int{1, 1}, []int{99, 100}, 0},
	AbilityHunger:    {[]string{"chance %", "fullness", "max fullness"}, []int{1, 0, 0}, []int{100, 999, 999}, 0},
	AbilityCurse:     {[]string{"chance %", "what", "items"}, []int{1, curseArms, 1}, []int{100, curseAny, 5}, 0},
	AbilitySteal:     {[]string{"chance %", "what", "items", "equipped"}, []int{1, stealGold, 0, 0}, []int{100, stealItems, 5, 1}, 0},
}

// maxReach is the farthest reach of an ability along a line.
const maxReach = 20

// abilityNames are the abilities in a fixed order, for the error that
// lists them.
var abilityNames = []AbilityName{AbilityAnger, AbilityCritical, AbilityErratic, AbilityCharge, AbilityPull, AbilityShoot, AbilityMagic, AbilityMimic, AbilitySteal, AbilitySplit, AbilityRot, AbilitySynthesis, AbilityCurse, AbilityHunger, AbilityStill, AbilityHold, AbilityBurn, AbilityHeal, AbilityCheer, AbilityRevive, AbilityExplode, AbilityPack, AbilityFlee, AbilityTrap, AbilityLong, AbilityBlink, AbilityKnight, AbilityInvisible, AbilityWalls, AbilityThrow, AbilityReflect, AbilityRuin, AbilityRust, AbilityAfflict}

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

// wounded is m hurt and still standing: it may get angry, or catch fire
// (fuse).
func (g *Game) wounded(m *Monster) {
	g.fuse(m)
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

// critTakenRate is the damage (%) of a critical blow a bracelet of 痛恨
// brings on the player (Shougo: 150%).
const critTakenRate = 150

// critical returns n, the damage of a blow of m, made a critical blow by
// chance, and whether it is one.
func (g *Game) critical(m *Monster, n int, shown bool) (int, bool) {
	a, ok := m.ability(AbilityCritical)
	if !ok || g.rng.IntN(100) >= a.Values[0] {
		return n, false
	}
	if shown {
		g.say("痛恨の一撃！")
	}
	return n * a.Values[1] / 100, true
}

// criticalOn returns n, the damage of a blow of m on the player, made a
// critical blow by chance: by m's own ability, or else by a bracelet of
// 痛恨; a critical blow does less with the guard of 防恨.
func (g *Game) criticalOn(m *Monster, n int) int {
	n, crit := g.critical(m, n, true)
	if b := g.equipped(CategoryBracelet); !crit && b != nil && b.Def.Effect == EffectCritTaken && g.rng.IntN(100) < b.Def.Strength {
		g.use(EffectCritTaken)
		g.say("痛恨の一撃！")
		n, crit = n*critTakenRate/100, true
	}
	if crit {
		if s := g.sealShare(SealCritGuard); s > 0 && n > 0 {
			n = max(int(math.Round(float64(n)*(1-s))), 1)
		}
	}
	return n
}

// erratic says whether m steps at random instead of acting, and makes
// the step.
func (g *Game) erratic(m *Monster) bool {
	a, ok := m.ability(AbilityErratic)
	if !ok || g.rng.IntN(100) >= a.Values[0] || m.still() {
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
		return rng.IntN(2) == 0
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
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Open() {
			return Point{}, 0, false
		}
	}
	return d, n, true
}

// reachAbility uses an ability of m that reaches the player from afar (a
// charge, a pull); it says whether m did.
func (g *Game) reachAbility(m *Monster) bool {
	if g.longHit(m) || g.blink(m) || g.swoop(m) {
		return true
	}
	if a, ok := m.ability(AbilityCharge); ok && g.charge(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityPull); ok && g.pull(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityShoot); ok && g.shootAt(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityThrow); ok && g.throwAt(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityAfflict); ok && a.Values[1] == 1 && g.afflictBy(m, a) {
		return true
	}
	if a, ok := m.ability(AbilityMagic); ok && g.castAt(m, a) {
		return true
	}
	return false
}

// charge is m rushing along its line to the player: it stops next to the
// first creature in the way, hurts it and blows it away. It does not rush
// over water it cannot stand on (the line itself may cross it).
func (g *Game) charge(m *Monster, a Ability) bool {
	d, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0])
	if !ok {
		return false
	}
	for p := m.Pos.Add(d); p != g.Player && g.MonsterAt(p) == nil; p = p.Add(d) {
		if !g.passable(m, p) {
			return false
		}
	}
	for next := m.Pos.Add(d); next != g.Player && g.MonsterAt(next) == nil; next = m.Pos.Add(d) {
		m.Pos = next
	}
	g.say(m.Name + "が突進してきた！")
	n := a.Values[1]
	if f := g.MonsterAt(m.Pos.Add(d)); f != nil {
		if f.Keeper && !f.Angry {
			// A shopkeeper keeps its shop (as against the bolts).
			g.say(f.Name + "には効かなかった。")
			return true
		}
		f.Dozing = false
		n = f.harm(n)
		f.HP -= n
		// (A blow frees it of かなしばり.)
		f.Bound = 0
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
	// Pulled over the water onto it, the player falls in (as blown).
	g.overboard()
	if hit && g.Over == "" && chebyshev(m.Pos, g.Player) == 1 {
		g.hitPlayer(m)
	}
	return true
}

// playerHurt hurts the player by n, done by m; it says whether the player
// fell.
func (g *Game) playerHurt(n int, m *Monster) bool {
	n = g.weakened(n)
	g.HP -= n
	g.say(fmt.Sprintf("%dのダメージ。", n))
	g.event("hurt %d %s", n, m.Def.ID)
	if g.HP <= 0 {
		g.HP = 0
		g.end(m.Name+"に倒された", false)
		// (Brought back by 復活の草, it did not.)
		return g.Over != ""
	}
	return false
}

// flingPlayer sends the player flying by d, up to blowAwayTiles, as m did;
// hitting a wall or a creature hurts it. A trap where it lands goes off;
// landing in water, without a bracelet of 水渡り, it is washed away
// (overboard).
func (g *Game) flingPlayer(d Point, m *Monster) {
	p := g.Player
	for range blowAwayTiles {
		next := p.Add(d)
		if g.clearFrom(p, d) && g.Level.At(next).Open() && !g.occupied(next) {
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
		g.overboard()
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
		if !g.clearFrom(p, d) || !g.Level.At(p.Add(d)).Open() {
			break
		}
		p = p.Add(d)
		if p == g.Player {
			if !hits(g.rng, r.MonsterThrowRate) {
				g.say("矢は当たらなかった。")
				continue
			}
			if !pierce && g.dodgeThrown(&Item{Def: def, Glyph: def.Glyph, Count: 1}) {
				return true
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
		if _, ok := f.ability(AbilityBurn); ok {
			if shown {
				g.say(def.Name + "は" + f.Name + "に燃やされた！")
			}
			return true
		}
		f.Dozing = false
		if !hits(g.rng, r.MonsterThrowRate) {
			if shown {
				g.say(def.Name + "は" + f.Name + "に当たらなかった。")
			}
			continue
		}
		n := damage(g.rng, attack, f.Defense, r.DefenseK)
		n = f.harm(n)
		f.HP -= n
		// (A blow frees it of かなしばり.)
		f.Bound = 0
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
