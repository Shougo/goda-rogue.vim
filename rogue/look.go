package rogue

import (
	"fmt"
	"slices"
)

// promptLook asks which of what the player sees to look at.
const promptLook = "look"

// speedNames are the speeds as the player reads them.
var speedNames = map[Speed]string{SpeedSlow: "鈍足", SpeedNormal: "通常", SpeedDouble: "倍速"}

// sight is something the player sees: a monster or an item.
type sight struct {
	pos     Point
	monster *Monster
	item    *Item
}

// lookAround asks which of the monsters and items the player sees to look
// at, the nearest first. It costs no turn.
func (g *Game) lookAround() {
	var sights []sight
	for _, m := range g.Monsters {
		if m.Ally || g.Visible(m.Pos) || g.wears(EffectSenseEnemy) {
			sights = append(sights, sight{pos: m.Pos, monster: m})
		}
	}
	for _, it := range g.Items {
		if g.IsSeen(it.Pos) && g.MonsterAt(it.Pos) == nil || g.senses(it.Pos) {
			sights = append(sights, sight{pos: it.Pos, item: it})
		}
	}
	if len(sights) == 0 {
		g.say("見えるものはない。")
		return
	}
	slices.SortStableFunc(sights, func(a, b sight) int {
		return chebyshev(a.pos, g.Player) - chebyshev(b.pos, g.Player)
	})
	p := &Prompt{Title: "見えるもの", kind: promptLook}
	for _, s := range sights {
		name := ""
		if s.monster != nil {
			name = s.monster.Glyph + " " + s.monster.Name
		} else {
			name = g.Label(s.item)
		}
		p.Choices = append(p.Choices, name+"  "+g.whereIs(s.pos))
	}
	g.Prompt = p
	g.sights = sights
}

// whereIs says where p is from the player: "右5 上2".
func (g *Game) whereIs(p Point) string {
	dx, dy := p.X-g.Player.X, p.Y-g.Player.Y
	s := ""
	switch {
	case dx > 0:
		s += fmt.Sprintf("右%d", dx)
	case dx < 0:
		s += fmt.Sprintf("左%d", -dx)
	}
	if dx != 0 && dy != 0 {
		s += " "
	}
	switch {
	case dy > 0:
		s += fmt.Sprintf("下%d", dy)
	case dy < 0:
		s += fmt.Sprintf("上%d", -dy)
	}
	return s
}

// lookAt tells about the sight n of the prompt.
func (g *Game) lookAt(n int) {
	s := g.sights[n]
	g.sights = nil
	if s.item != nil {
		g.say(g.describe(s.item) + "が落ちている。")
		return
	}
	m := s.monster
	msg := fmt.Sprintf("%s：ランク%d  %s", m.Name, m.Rank, speedNames[m.Speed])
	switch {
	case m.Ally:
		msg += fmt.Sprintf("  仲間 Lv%d  HP %d/%d", m.Level, m.HP, m.MaxHP)
	case m.Keeper && !m.Angry:
		msg += "  店主"
	case m.Angry:
		msg += "  怒っている"
	case m.Dozing:
		msg += "  眠っている"
	}
	g.say(msg)
}
