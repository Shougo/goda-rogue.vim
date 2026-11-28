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
		if !g.hidden(m) && (m.Ally || g.Visible(m.Pos) || g.sensesMonster(m.Pos)) {
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
	for _, a := range m.Def.Attrs {
		msg += "  " + a
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
		// The prompt hides the messages: its title tells the status.
		g.askOrder(m, msg)
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
	case AbilityPoison:
		return fmt.Sprintf("殴りが当たると%d%%で、ちからを%d下げる", v[0], v[1])
	case AbilityBreath:
		var from string
		switch {
		case v[0] == 0:
			from = "直線上から"
		case v[0] == 1:
			from = "同じ部屋のどこからでも"
		case v[0] >= breathFloor:
			from = "フロアのどこからでも"
		default:
			from = fmt.Sprintf("%dマス以内から壁ごしにも", v[0])
		}
		return from + fmt.Sprintf("%d%%で炎を吐く（ダメージ%d、防御力は関係ない）", v[1], v[2])
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
	case AbilitySplit:
		return fmt.Sprintf("殴ると%d%%で分裂する", v[0])
	case AbilityRot:
		return rotText(v)
	case AbilityCurse:
		return curseText(v)
	case AbilityHunger:
		return hungerText(v)
	case AbilityHeal:
		return fmt.Sprintf("%d%%で、見えている傷ついた仲間のHPを%d回復する", v[0], v[1])
	case AbilityCheer:
		return fmt.Sprintf("%d%%で、見えている仲間のランクを上げる。祈る者どうしは上げない", v[0])
	case AbilityPack:
		return fmt.Sprintf("%d匹の仲間と群れで現れる", v[0])
	case AbilityFlee:
		return "プレイヤーから逃げる。追い詰められると戦う。他のモンスターに狩られる（倒したモンスターはランクが上がる）"
	case AbilityTrap:
		return fmt.Sprintf("%d%%で足元に罠を仕掛ける。倒すとそこに罠が残る", v[0])
	case AbilityLong:
		return "まっすぐ2マス先まで攻撃が届く"
	case AbilityBlink:
		return fmt.Sprintf("見えていると%d%%で目の前に現れる", v[0])
	case AbilityKnight:
		return fmt.Sprintf("桂馬のように跳んで動く。桂馬の位置から舞い降りて%dのダメージ（防御は効かない）", v[0])
	case AbilityInvisible:
		return "姿が見えない。目薬草を飲むと、その階では見える"
	case AbilityWalls:
		return "どこにいても、壁の中を通り抜けて向かってくる。壁の中では攻撃しない"
	case AbilityRevive:
		return fmt.Sprintf("一度倒しても、最大HPの%d%%で起き上がる", v[0])
	case AbilityExplode:
		return fmt.Sprintf("HPが%d%%以下になると、次の行動で爆発する。周りのHPを%d%%減らし、周りの道具を燃やす", v[0], v[1])
	case AbilityStill:
		return "その場から動かない"
	case AbilityHold:
		return holdText(v)
	case AbilityBurn:
		return "投げたものや矢は燃やされて効かない。殴るしかない"
	case AbilitySynthesis:
		s := fmt.Sprintf("投げた道具を%d個まで飲み込んで合成し、倒すと落とす", v[0])
		if v[1] > 0 {
			s += fmt.Sprintf("。隣にいると%d%%で、持っている武器か盾を吸い込む（装備中のものも）", v[1])
		}
		return s
	case AbilitySteal:
		what := "お金を盗み"
		if v[1] == stealItems {
			what = fmt.Sprintf("道具を%d個盗み", v[2])
			if v[3] == 1 {
				what = fmt.Sprintf("装備中のものも含めて道具を%d個盗み", v[2])
			}
		}
		chance := ""
		if v[0] < 100 {
			chance = fmt.Sprintf("%d%%で", v[0])
		}
		return "隣にいると" + chance + what + "、ワープして逃げる（倒すと盗んだものを落とす）"
	case AbilityThrow:
		where := line(v[0])
		if v[2] == 1 {
			where = fmt.Sprintf("%dマス以内の見える所なら", v[0])
		}
		return where + fmt.Sprintf("%d%%で、足元の道具を投げつけてくる（草はその効果が出る）。プレイヤーより先に、見える道具のところへ行く", v[1])
	case AbilityReflect:
		what := [...]string{"杖の魔法弾", "杖の魔法弾と矢", "杖の魔法弾と、矢や投げた道具"}[v[1]]
		chance := ""
		if v[0] < 100 {
			chance = fmt.Sprintf("%d%%で", v[0])
		}
		return chance + what + "をはね返す"
	case AbilityRuin:
		what := [...]string{"杖の回数を0にする", "草か巻物を1つなくす", "壺を普通の壺に変える", "杖の回数を0にするか、草か巻物をなくすか、壺を普通の壺に変える"}[v[1]]
		return fmt.Sprintf("隣にいると%d%%で、持ち物の%s（盗みよけで防げる）", v[0], what)
	case AbilityAfflict:
		where := "隣にいると"
		if v[1] == 1 {
			where = "見えていると"
		}
		if v[2] == afflictMute {
			return fmt.Sprintf("%s%d%%で、口を封じる（その階のあいだ）", where, v[0])
		}
		what := [...]string{"目つぶし（隣しか見えない）にする", "眠らせる", "混乱させる", "攻撃力か防御力を下げる（30%、その階のあいだ）"}[v[2]]
		return fmt.Sprintf("%s%d%%で、%dターン%s", where, v[0], v[3], what)
	case AbilityRust:
		what := [...]string{"装備している武器か盾を%dさびさせる", "装備している武器と盾を%dさびさせる", "装備している武器か盾を%dさびさせるか、印を1つ消す"}[v[2]]
		return fmt.Sprintf("攻撃しない。隣にいると%d%%で、", v[0]) + fmt.Sprintf(what, v[1]) + "（サビよけの印を先に消す。素の能力のサビよけには効かない）"
	case AbilityMimic:
		if v[0] == mimicDeep {
			return "道具や階段に化け、持ち物の中でも気づかれない（使うか、次のフロアで正体を現す。識別で見破れる）"
		}
		return "道具や店の品に化ける（拾おうとすると正体を現す。気配察知や目薬で見破れる）"
	}
	return string(a.Name)
}
