package rogue

import (
	"math/rand/v2"
)

// The provisional combat rules. They are to be chosen with a balance
// simulator, and then to become data: keep them in this file, behind these
// functions.

// defenseK is K of the provisional defense curve, damage x K / (K +
// defense): every point of defense cuts the same fraction of the damage,
// whatever the attack, and defense never makes it 0.
const defenseK = 20

// damage returns the damage of an attack of attack on a defense of defense:
// randomized by +-1/8, and at least 1.
func damage(rng *rand.Rand, attack, defense int) int {
	r := 0.875 + rng.Float64()*0.25
	d := float64(attack) * r * defenseK / float64(defenseK+defense)
	return max(int(d+0.5), 1)
}

// The hit rates, in percent (as in Shiren 5).
const (
	playerHitRate  = 92
	monsterHitRate = 88
)

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

// rankStats are the stats of a monster by its rank, until monsters are
// data.
var rankStats = [...]struct{ HP, Attack, Defense int }{
	{},
	{HP: 6, Attack: 3, Defense: 1},
	{HP: 15, Attack: 6, Defense: 4},
	{HP: 30, Attack: 11, Defense: 8},
	{HP: 60, Attack: 18, Defense: 14},
}

// The player at the start of a run.
const (
	playerHP       = 15
	playerStrength = 8
)

// regenTurns is how many turns of walking or waiting restore 1 HP.
const regenTurns = 2

// The rules of lingering on a floor, until dungeons are data.
const (
	// spawnTurns is how often a monster appears, out of the player's
	// sight; maxMonsters is how many there can be.
	spawnTurns  = 40
	maxMonsters = 16
)

// windTurns are the turns on a floor at which Kron's wind blows: a
// warning, then a strong monster (and every enemy one rank stronger), then
// the end of the run.
var windTurns = [3]int{800, 900, 1000}
