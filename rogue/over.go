package rogue

import (
	"math"
	"strconv"
)

// Over-rank monsters (Shougo, 2026-11-19 and 2026-11-24): a monster of
// rank 4 that ranks up (by the second wind, or by defeating another, a
// rabbit say) goes beyond its rank: each time, its attack and its
// defense grow by a share (rules.txt 超過攻撃 and 超過防御, 130 and 120
// percent by default), its max HP stays, as in Shiren (レベル4超過:
// attack nearly doubles, HP does not change; here less, x2 being too
// much). It keeps its kind, its color and its EXP; its name gets the
// level, from 2: 玉兎2, 玉兎3, ...

// The percents of the attack and the defense a level beyond rank 4, when
// rules.txt has none.
const (
	defaultOverAttack  = 130
	defaultOverDefense = 120
)

// overRank ranks m, of the last rank of its family, beyond it.
func (g *Game) overRank(m *Monster) {
	if m.Rank < maxRank {
		return
	}
	m.Over++
	r := g.Dungeon.Rules
	m.Attack = overStat(m.Def.Attack, r.OverAttack, m.Over)
	m.Defense = overStat(m.Def.Defense, r.OverDefense, m.Over)
	m.Name = m.Def.Name + strconv.Itoa(m.Over+1)
}

// overStat is base grown by pct percent, levels times.
func overStat(base, pct, levels int) int {
	return int(math.Round(float64(base) * math.Pow(float64(pct)/100, float64(levels))))
}
