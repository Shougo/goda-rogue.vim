package rogue

import (
	"fmt"
	"strings"
)

// commandInfo shows what an item is and does; it costs no turn.
const commandInfo = "説明"

// promptInfo shows the description of an item: any answer closes it.
const promptInfo = "info"

// effectTexts say what each effect does, by the category of the item
// ("%d" is its strength).
var effectTexts = map[Effect]string{
	EffectStrength:   "ちからが %d 上がる。",
	EffectRegen:      "HP の回復が 2 倍速くなる（お腹も 2 倍速く減る）。",
	EffectSenseEnemy: "%d マス以内の敵の居場所が分かる。",
	EffectSenseItem:  "フロアの道具の場所が分かる。",
	EffectHeal:       "HP が %d 回復する。HP が満タンなら最大 HP が上がる。",
	EffectLight:      "フロア全体が明るくなり、地形と道具と敵が見える。",
	EffectUncurse:    "持ち物すべての呪いを解く。",
	EffectIdentify:   "持ち物を 1 つ識別する。",
	EffectBlowAway:   "魔法弾が当たった敵を 10 マス吹き飛ばす。",
	EffectFood:       "満腹度が %d 回復する。",
	EffectRotten:     "満腹度が %d 回復するが、混乱、鈍足、睡眠のどれかになる。",
	EffectKeep:       "入れた道具をしまっておける。「見る」で中の道具を使える。",
	EffectSynthesis: "武器と盾は、先に入れた同じ種類のものに印と修正値が合成される。" +
		"同じ腕輪を入れるとひびが直る。中のものは出せない。",
	EffectHealPot: "道具を入れると HP が全部回復する。入れた道具はなくなり、壺の容量も 1 つ減る。",
}

// sealTexts say what each seal does ("%d" is its strength).
var sealTexts = map[SealEffect]string{
	SealAttack:   "攻撃力 +%d。",
	SealDefense:  "防御力 +%d。",
	SealCrit:     "%d%% の確率で会心の一撃（2 倍のダメージ）。",
	SealSure:     "攻撃が必ず当たる。",
	SealCounter:  "%d%% の確率で受けたダメージを相手に返す。",
	SealNoRust:   "武器と盾がサビない。",
	SealNoHunger: "お腹が減る速さが半分になる。",
	SealThree:    "正面と、その左右の斜め前の 3 方向を同時に攻撃する。",
	SealNoTheft:  "道具もお金も盗まれない。",
	SealDodge:    "敵の攻撃が当たる確率が %d%% 下がる。",
}

// usePrefixes say when the effect of an item of each category happens.
var usePrefixes = map[Category]string{
	CategoryBracelet: "装備している間、",
	CategoryGrass:    "飲むと、",
	CategoryScroll:   "読むと、",
	CategoryStaff:    "振ると、",
	CategoryFood:     "食べると、",
}

// effectText says what the effect of def does.
func effectText(def *ItemDef) string {
	t, ok := effectTexts[def.Effect]
	if !ok {
		return "特に効果はない。"
	}
	if def.Category == CategoryPot {
		return t
	}
	if def.Effect == EffectSenseEnemy && def.Strength == 0 {
		t = "フロアの敵の居場所が分かる。"
	}
	return usePrefixes[def.Category] + withStrength(t, def.Strength)
}

// withStrength puts n in t, if it has a place for it.
func withStrength(t string, n int) string {
	if strings.Contains(t, "%d") {
		return fmt.Sprintf(t, n)
	}
	return t
}

// info returns the lines that describe it: what it is, its numbers, and
// what it does, as far as the player knows.
func (g *Game) info(it *Item) []string {
	if it.Unmasked {
		return []string{
			it.Mimic.Name + "（道具に化けた魔物）",
			"使う、投げる、置くと、隣に現れて襲ってくる。",
			"次のフロアに持って行っても現れる。",
		}
	}
	def := it.Def
	lines := []string{fmt.Sprintf("%s（%s）", g.describe(it), def.Category)}
	if !g.identified[def.ID] {
		lines = append(lines, "正体はまだ分からない。使うか識別すると分かる。")
		return append(lines, g.shopInfo(it)...)
	}
	switch def.Category {
	case CategoryWeapon, CategoryShield:
		what := "攻撃力"
		if def.Category == CategoryShield {
			what = "防御力"
		}
		if it.Known {
			lines = append(lines, fmt.Sprintf("%s %d（基本 %d、修正値 %+d）", what, it.Strength(), def.Strength, it.Upgrade))
		} else {
			lines = append(lines, fmt.Sprintf("%s %d+?（修正値は装備するか識別すると分かる）", what, def.Strength))
		}
		lines = append(lines, fmt.Sprintf("印 %d/%d", len(it.Seals), def.Slots))
		if it.curseKnown() {
			lines = append(lines, "呪われている（装備すると外せない）")
		}
		for _, s := range it.Seals {
			sd := g.Dungeon.SealOn(s, def.Category)
			if sd == nil {
				continue
			}
			lines = append(lines, fmt.Sprintf("【%s】%s：%s", s, sd.Name, withStrength(sealTexts[sd.Effect], sd.Strength)))
		}
	case CategoryArrow:
		lines = append(lines, fmt.Sprintf("攻撃力 %d。%d 本。装備すると撃てる。", def.Strength, it.Count))
	case CategoryBracelet:
		lines = append(lines, effectText(def))
		if it.curseKnown() {
			lines = append(lines, "呪われている（装備すると外せない）")
		}
		switch {
		case def.Crack == 0:
			lines = append(lines, "ひびは入らない。")
		default:
			lines = append(lines, fmt.Sprintf("ひび %d/%d（効果を発揮したフロアを出るとき、ひびが入ることがある）", it.Cracks, g.Dungeon.Rules.CrackStages))
		}
	case CategoryStaff:
		lines = append(lines, effectText(def))
		if it.Known {
			lines = append(lines, fmt.Sprintf("残り %d 回。", it.Charges))
		} else {
			lines = append(lines, "残りの回数は分からない。")
		}
	case CategoryMoney:
		lines = append(lines,
			fmt.Sprintf("投げて当てると %d のダメージ（金額の 1/%d、防御は効かない）。", max(it.Amount/g.Dungeon.Rules.GoldThrow, 1), g.Dungeon.Rules.GoldThrow),
			"「財布に入れる」と所持金になる。")
	case CategoryPot:
		lines = append(lines, effectText(def))
		if it.Known {
			lines = append(lines, fmt.Sprintf("あと %d 個入る（%d 個入っている）。", it.Charges-len(it.Contents), len(it.Contents)))
		}
	default:
		lines = append(lines, effectText(def))
	}
	lines = append(lines, g.materialInfo(def)...)
	return append(lines, g.shopInfo(it)...)
}

// materialInfo says what seal def gives, as a material merged into a
// weapon or a shield (異種合成; none for others).
func (g *Game) materialInfo(def *ItemDef) []string {
	if def.Seal == "" || def.Category == CategoryWeapon || def.Category == CategoryShield {
		return nil
	}
	var lines []string
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		if s := g.Dungeon.SealOn(def.Seal, c); s != nil {
			lines = append(lines, fmt.Sprintf("%sに異種合成すると【%s】%s：%s", c, s.Glyph, s.Name,
				withStrength(sealTexts[s.Effect], s.Strength)))
		}
	}
	return lines
}

// showInfo shows the description of it.
func (g *Game) showInfo(it *Item) {
	lines := g.info(it)
	g.Prompt = &Prompt{Title: commandInfo, Choices: lines, kind: promptInfo}
}
