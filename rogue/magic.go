package rogue

import "fmt"

// castAt is m waving its staff along its line to the player, which it
// sees: a bolt of one of its effects, at random, to the first creature
// on the way.
func (g *Game) castAt(m *Monster, a Ability) bool {
	d, _, ok := g.lineTo(m.Pos, g.Player, a.Values[0])
	if !ok || !g.sees(m.Pos, g.Player) || g.rng.IntN(100) >= a.Values[1] {
		return false
	}
	e := a.Effects[g.rng.IntN(len(a.Effects))]
	g.say(m.Name + "は杖を振った！")
	for p := m.Pos.Add(d); ; p = p.Add(d) {
		if p == g.Player {
			if g.sealChance(SealReflect) {
				g.say("盾が魔法弾をはね返した！")
				g.magicOn(m, e, Point{-d.X, -d.Y})
				return true
			}
			g.magicOnPlayer(e, d, m)
			return true
		}
		if f := g.MonsterAt(p); f != nil {
			g.magicOn(f, e, d)
			return true
		}
	}
}

// magicOnPlayer is the bolt e of m, flying by d, taking the player.
func (g *Game) magicOnPlayer(e MagicEffect, d Point, m *Monster) {
	switch e {
	case MagicWarp:
		if p, ok := g.freeTile(); ok {
			g.say("ワープした。")
			g.shove(p)
		}
	case MagicHaste:
		g.Hasted, g.Slowed = hasteTurns, 0
		g.say("体が速くなった！")
	case MagicBlow:
		g.say("吹き飛ばされた。")
		g.flingPlayer(d, m)
	case MagicConfuse, MagicSlow, MagicSleep, MagicSeal, MagicBlind, MagicBind, MagicUnhurt:
		g.afflict(e)
	case MagicLevelUp:
		g.levelUp()
	case MagicPoison:
		g.poisoned(m)
	case MagicHalf, MagicQuarter:
		g.HP = max(g.HP/fraction(e), 1)
		g.say(fmt.Sprintf("HPが%dになった。", g.HP))
	case MagicLevel1, MagicLevel3:
		g.levelDown(levels(e))
	}
}

// magicOn is the bolt e, flying by d, taking the monster f.
func (g *Game) magicOn(f *Monster, e MagicEffect, d Point) {
	f.Dozing = false
	shown := g.sees(g.Player, f.Pos)
	say := func(s string) {
		if shown {
			g.say(f.Name + s)
		}
	}
	if f.Keeper && !f.Angry && (e == MagicWarp || e == MagicBlow) {
		// A shopkeeper keeps its shop: the bolt of a monster does not
		// take it away (the player's makes it angry first).
		say("には効かなかった。")
		return
	}
	switch e {
	case MagicWarp:
		if p, ok := g.freeTile(); ok {
			say("はワープした。")
			f.Pos = p
		}
	case MagicHaste:
		f.Speed = f.Speed.faster()
		f.Slowed = 0
		say("は速くなった！")
	case MagicBlow:
		say("は吹き飛ばされた。")
		g.fling(f, d, false)
	case MagicConfuse, MagicSlow, MagicSleep, MagicSeal, MagicBlind, MagicBind, MagicUnhurt:
		f.States.take(e)
		if f.Ally && e != MagicBind {
			// (Another state frees an ally of かなしばり.)
			f.Bound = 0
		}
		say(stateTexts[e].monster)
	case MagicHalf, MagicQuarter:
		f.HP = max(f.HP/fraction(e), 1)
		f.Bound = 0
		say(fmt.Sprintf("のHPが%dになった。", f.HP))
	case MagicLevel1, MagicLevel3:
		name := f.Name
		for range levels(e) {
			g.rankDown(f)
		}
		if shown && f.Name != name {
			g.say(name + "は" + f.Name + "になった。")
		}
	case MagicLevelUp:
		name := f.Name
		g.rankUp(f)
		if shown && f.Name != name {
			g.say(name + "は" + f.Name + "になった。")
		}
	case MagicPoison:
		n := f.harm(poisonDamage)
		f.HP -= n
		f.Bound = 0
		f.Attack = max(f.Attack/2, 1)
		// (Slowed too: Shougo, 2026-11-28.)
		f.Slowed, f.Hasted = slowTurns, 0
		say(fmt.Sprintf("に%dのダメージ。攻撃力が下がり、遅くなった。", n))
		if f.HP <= 0 {
			g.kill(f, nil, true, shown)
		} else {
			g.wounded(f)
		}
	}
}

// poisonDamage is the damage of 毒草: on the player, with a point of its
// strength; on a monster, with half its attack, and slowed.
const poisonDamage = 5

// poisoned is the poison of a bolt of m (nil: 毒草 drunk) on the player:
// poisonDamage, and a point of its strength (not under 1).
func (g *Game) poisoned(m *Monster) {
	var fell bool
	if m != nil {
		fell = g.playerHurt(poisonDamage, m)
	} else {
		fell = g.trapDamage(poisonDamage, "毒")
	}
	if fell {
		return
	}
	if g.Strength > 1 {
		g.Strength--
		g.say("ちからが1下がった。")
	}
}

// fraction is what the bolt e divides HP by.
func fraction(e MagicEffect) int {
	if e == MagicQuarter {
		return 4
	}
	return 2
}

// levels is how many levels the bolt e takes away.
func levels(e MagicEffect) int {
	if e == MagicLevel3 {
		return 3
	}
	return 1
}

// faster is the speed one step above s.
func (s Speed) faster() Speed {
	switch s {
	case SpeedSlow:
		return SpeedNormal
	case SpeedNormal, SpeedMoveDouble:
		return SpeedDouble
	}
	return s
}

// freeTile returns a walkable tile of the floor with no creature on it,
// at random.
func (g *Game) freeTile() (Point, bool) {
	return g.find(g.wanderTarget, needNoCreature)
}

// levelDown takes n levels from the player (not below 1): its EXP becomes
// the least of its new level, and its max HP loses what the levels gave.
func (g *Game) levelDown(n int) {
	growth := g.Dungeon.Growth[g.Dungeon.Rules.Growth]
	from := g.PlayerLevel
	for range n {
		if g.PlayerLevel <= 1 {
			break
		}
		g.PlayerLevel--
		g.MaxHP = max(g.MaxHP-growth[g.PlayerLevel-1].HP, 1)
	}
	if g.PlayerLevel == from {
		g.say("何も起こらなかった。")
		return
	}
	g.EXP = 0
	if g.PlayerLevel > 1 {
		g.EXP = growth[g.PlayerLevel-2].EXP
	}
	g.HP = min(g.HP, g.MaxHP)
	g.event("level %d", g.PlayerLevel)
	g.say(fmt.Sprintf("主はレベル%dに下がった。", g.PlayerLevel))
}

// rankDown makes m the kind of its family one rank below, if there is
// one, its HP in proportion.
func (g *Game) rankDown(m *Monster) {
	prev := g.Dungeon.prevRank(m.Def)
	if prev == nil {
		return
	}
	m.Def, m.Name, m.Rank, m.Speed, m.Enraged = prev, prev.Name, prev.Rank, prev.Speed, false
	m.HP = max(m.HP*prev.HP/m.MaxHP, 1)
	m.MaxHP, m.Attack, m.Defense = prev.HP, prev.Attack, prev.Defense
}

// mouthBlocks says whether the player's sealed mouth keeps it from the
// command c (eating, drinking, reading), and says so.
func (g *Game) mouthBlocks(c string) bool {
	if !g.Sealed {
		return false
	}
	switch c {
	case commandEat, commandDrink, commandRead:
		g.say("口が封じられていて" + c + "ことができない。")
		return true
	}
	return false
}

// confusedDirection is where a confused player goes: a direction at
// random.
func (g *Game) confusedDirection() (string, Point) {
	name := clockwise[g.rng.IntN(len(clockwise))]
	return name, Directions[name]
}

// actConfused is a confused monster's action: a step or a blow in a
// direction at random. It says whether m attacked.
func (g *Game) actConfused(m *Monster) bool {
	m.Confused--
	d := around[g.rng.IntN(len(around))]
	p := m.Pos.Add(d)
	if !g.clearFrom(m.Pos, d) {
		return false
	}
	if p == g.Player {
		g.hitPlayer(m)
		return true
	}
	if f := g.MonsterAt(p); f != nil {
		g.hitMonster(m, f)
		return true
	}
	if g.Level.At(p).Walkable() && !m.still() {
		m.Pos = p
	}
	return false
}
