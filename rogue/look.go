package rogue

import (
	"fmt"
	"slices"
	"strings"
)

// promptLook asks which of what the player sees to look at.
const promptLook = "look"

// speedNames are the speeds as the player reads them.
var speedNames = map[Speed]string{SpeedSlow: "鈍足", SpeedNormal: "通常", SpeedDouble: "倍速", SpeedMoveDouble: "移動倍速"}

// sight is something the player sees: a monster or an item.
type sight struct {
	pos     Point
	monster *Monster
	item    *Item
	trap    *Trap
}

// lookAround asks which of the monsters and items the player sees to look
// at, the nearest first. It costs no turn.
func (g *Game) lookAround() {
	var sights []sight
	for _, m := range g.Monsters {
		if m.Ally || g.Visible(m.Pos) || g.sensesMonster(m.Pos) {
			sights = append(sights, sight{pos: m.Pos, monster: m})
		}
	}
	for _, it := range g.Items {
		if g.IsSeen(it.Pos) && g.MonsterAt(it.Pos) == nil || g.senses(it.Pos) {
			sights = append(sights, sight{pos: it.Pos, item: it})
		}
	}
	for _, t := range g.Traps {
		if t.Found && g.IsSeen(t.Pos) && g.MonsterAt(t.Pos) == nil {
			sights = append(sights, sight{pos: t.Pos, trap: t})
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
		switch {
		case s.monster != nil:
			name = s.monster.Glyph + " " + s.monster.Name
		case s.trap != nil:
			name = glyphTrap + " " + s.trap.Def.Name
		default:
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
	if t := s.trap; t != nil {
		g.say(t.Def.Name + "：" + trapTexts[t.Def.Effect])
		return
	}
	m := s.monster
	msg := fmt.Sprintf("%s：ランク%d  %s", m.Name, m.Rank, speedNames[m.Speed])
	switch {
	case m.Ally:
		msg += fmt.Sprintf("  仲間 Lv%d  HP %d/%d  命令：%s", m.Level, m.HP, m.MaxHP, m.order())
	case m.Keeper && !m.Angry:
		msg += "  店主"
	case m.Angry, m.Enraged:
		msg += "  怒っている"
	case m.Dozing:
		msg += "  眠っている"
	}
	for _, name := range m.States.names("封印") {
		msg += "  " + name
	}
	g.say(msg)
	if len(m.Def.Abilities) > 0 {
		var texts []string
		for _, a := range m.Def.Abilities {
			texts = append(texts, g.abilityText(a))
		}
		text := strings.Join(texts, "。") + "。"
		if m.Sealed {
			text = "（封印中）" + text
		}
		g.say(text)
	}
	if m.Def.Description != "" {
		g.say(m.Def.Description)
	}
	if m.Ally {
		g.askOrder(m)
	}
}

// abilityText tells what the ability a does, from its numbers.
func (g *Game) abilityText(a Ability) string {
	v := a.Values
	line := func(reach int) string {
		return fmt.Sprintf("直線上%dマス以内から", reach)
	}
	switch a.Name {
	case AbilityAnger:
		return fmt.Sprintf("HPが%d%%以下になると怒り、攻撃が%d%%になる", v[0], v[1])
	case AbilityCritical:
		return fmt.Sprintf("攻撃の%d%%が痛恨の一撃（ダメージ%d%%）", v[0], v[1])
	case AbilityErratic:
		return fmt.Sprintf("行動の%d%%はふらふら歩く", v[0])
	case AbilityCharge:
		return line(v[0]) + fmt.Sprintf("突進してくる（ダメージ%d、吹き飛ばす）", v[1])
	case AbilityPull:
		from := line(v[0])
		if v[0] == 0 {
			from = "同じ部屋のどこからでも"
		}
		s := from + fmt.Sprintf("%d%%で引き寄せる", v[1])
		if v[2] == 1 {
			s += "（そのあと殴る）"
		}
		return s
	case AbilityShoot:
		s := line(v[0]) + g.Dungeon.Item(a.Item).Name + "を撃つ"
		if v[1] < 100 {
			s += fmt.Sprintf("（%d%%）", v[1])
		}
		if v[3] == 1 {
			s += "（貫通）"
		}
		return s
	case AbilityMagic:
		es := make([]string, len(a.Effects))
		for i, e := range a.Effects {
			es[i] = string(e)
		}
		return line(v[0]) + fmt.Sprintf("%d%%で杖を振る（%s）", v[1], strings.Join(es, "、"))
	}
	return string(a.Name)
}
