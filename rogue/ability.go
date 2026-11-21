package rogue

import (
	"fmt"
	"math/rand/v2"
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
)

// abilityDef is what an ability takes: its numbers, each in a range.
type abilityDef struct {
	// values are the names of its numbers (for the errors), mins and
	// maxs their ranges.
	values     []string
	mins, maxs []int
}

var abilityDefs = map[AbilityName]abilityDef{
	AbilityAnger:    {[]string{"HP %", "attack %"}, []int{1, 100}, []int{99, 1000}},
	AbilityCritical: {[]string{"chance %", "damage %"}, []int{1, 100}, []int{100, 1000}},
	AbilityErratic:  {[]string{"chance %"}, []int{1}, []int{100}},
}

// abilityNames are the abilities in a fixed order, for the error that
// lists them.
var abilityNames = []AbilityName{AbilityAnger, AbilityCritical, AbilityErratic}

// Ability is an ability of a kind of monster, with its numbers.
type Ability struct {
	Name   AbilityName
	Values []int
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

// ability returns the ability of m named n, if it has it.
func (m *Monster) ability(n AbilityName) (Ability, bool) {
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
