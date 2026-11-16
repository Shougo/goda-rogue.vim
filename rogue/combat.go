package rogue

import (
	"math/rand/v2"
)

// The provisional combat formulas. They are to be chosen with a balance
// simulator; their constants are rules of the dungeon. Keep them in this
// file, behind these functions.

// damage returns the damage of an attack of attack on a defense of
// defense, by the provisional defense curve damage x k / (k + defense)
// (every point of defense cuts the same fraction of the damage, whatever
// the attack, and defense never makes it 0): randomized by +-1/8, and at
// least 1. k is a rule of the dungeon.
func damage(rng *rand.Rand, attack, defense, k int) int {
	r := 0.875 + rng.Float64()*0.25
	d := float64(attack) * r * float64(k) / float64(k+defense)
	return max(int(d+0.5), 1)
}

// hits says whether an attack of a hit rate (in percent) hits.
func hits(rng *rand.Rand, rate int) bool { return rng.IntN(100) < rate }

// playerAttack returns the attack of the player: from its level (as Shiren
// 6: +1.5 a level up to 5, +1 up to 13, +0.5 above) and its strength. There
// are no weapons yet.
func playerAttack(level, strength int) int {
	var lv float64
	switch {
	case level <= 5:
		lv = 1 + float64(level-1)*1.5
	case level <= 13:
		lv = 7.5 + float64(level-5)
	default:
		lv = 15.5 + float64(level-13)*0.5
	}
	return int(lv) + strength
}
