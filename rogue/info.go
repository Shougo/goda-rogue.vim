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
	EffectStrength:     "ちからが %d 上がる。",
	EffectRegen:        "HP の回復が 2 倍速くなる（お腹も 2 倍速く減る）。",
	EffectSenseEnemy:   "%d マス以内の敵の居場所が分かる。",
	EffectSenseItem:    "フロアの道具の場所が分かる。",
	EffectEyeDrops:     "フロアの罠と、道具に化けたものが見える。目つぶしも治る。",
	EffectMaxFull:      "最大満腹度が %d 上がる（200 まで）。",
	EffectLessFull:     "最大満腹度が %d 下がる（50 未満にはならない）。",
	EffectRevive:       "持っていると（壺の中は除く）、倒れたときに一度だけ HP が全回復して生き返る（未識別でも、口封じでも）。飲んでも何も起こらない。",
	EffectFire:         "正面に炎を吐き、最初に当たった敵に %d 前後のダメージ（防御力は関係ない）。投げて当てると、その半分のダメージ。",
	EffectHeal:         "HP が %d 回復する。HP が満タンなら最大 HP が上がる。",
	EffectLight:        "フロア全体が明るくなり、地形と道具と敵が見える。",
	EffectDrain:        "フロアの水路の水がかれて、歩けるようになる。",
	EffectUncurse:      "持ち物 1 つの呪いを解く。ラッキーなら持ち物すべて。",
	EffectMoreSlots:    "選んだ武器か盾の印の枠を 1 つ増やす（10 まで）。",
	EffectEraseSeal:    "選んだ印を 1 つ消す（素の能力は消えない）。",
	EffectUpWeapon:     "装備している武器の修正値が %d 上がり、呪いが解ける。ラッキーならさらに 2 上がる。",
	EffectUpShield:     "装備している盾の修正値が %d 上がり、呪いが解ける。ラッキーならさらに 2 上がる。",
	EffectIdentify:     "持ち物を 1 つ識別する。ラッキーなら持ち物すべて。",
	EffectBlowAway:     "魔法弾が当たった敵を 10 マス吹き飛ばす。",
	EffectFood:         "満腹度が %d 回復する。",
	EffectGrilled:      "満腹度と HP が %d 回復する。腐らない。",
	EffectRotten:       "満腹度が %d 回復するが、混乱、鈍足、睡眠のどれかになる。",
	EffectKeep:         "入れた道具をしまっておける。「見る」で中の道具を使える。",
	EffectNoRust:       "装備している武器と盾がサビない。",
	EffectNoTheft:      "道具もお金も盗まれず、荒らされない。",
	EffectNoSleep:      "眠らされない。",
	EffectNoConfuse:    "混乱しない。",
	EffectNoCurse:      "持ち物が呪われない（呪いの罠、呪いをかける敵）。",
	EffectNoBlind:      "目つぶしにならない。",
	EffectNoSeal:       "口を封じられない。",
	EffectNoWeak:       "攻撃力や防御力を下げられない。",
	EffectHungry:       "お腹が 2 倍速く減るが、攻撃で与えるダメージが %d%% 増える。",
	EffectDrops:        "倒した敵が道具を落とす確率が %d%% 上がるが、ときどき持ち物を足元に落とす。",
	EffectSmallStomach: "敵の通常攻撃で受けるダメージが %d%% 減るが、最大満腹度が 25 下がる。",
	EffectCritTaken:    "敵の攻撃が %d%% の確率で痛恨の一撃（1.5 倍）になる。",
	EffectCall:         "フロアの敵がすべて寄ってきて、敵の湧く速さも 2 倍になる。",
	EffectLeap:         "毎ターン %d%% の確率で、フロアのどこかへ飛ぶ。",
	EffectBlast:        "毎ターン %d%% の確率で爆発する。自分の HP は半分になり、周りの敵は消え、仲間の HP は半分になり、隣の道具は消え、隣の壁は崩れる。",
	EffectClumsy:       "投げた物と撃った矢が必ず外れ、狙った敵の足元に落ちる。敵が罠の上にいれば、その罠が敵に働く。",
	EffectBlastGuard:   "爆発（敵の爆発、地雷）で受けるダメージが半分になる。",
	EffectWaterWalk:    "水路の上を歩ける。水の上で外すと、水に落ちてどこかへ流される。",
	EffectFloat:        "浮いて、水路の上を渡れ、罠を踏まない。",
	EffectWalls:        "壁の中を歩ける。壁の中では毎ターン HP が 1 減る（1 は残る）。",
	EffectCursePower:   "持っている呪われた物 1 つにつき、ちからが %d 上がる（壺の中は数えない）。",
	EffectNoStarve:     "満腹度を吸われない（餓、腹減りの罠、空腹の罠）。",
	EffectPierce:       "投げた物と撃った矢が、当たった敵を貫いて飛ぶ。",
	EffectHappy:        "敵を倒して得る経験値が %d%% 増える。",
	EffectStealth:      "部屋に出入りしても、眠っている敵が起きない（モンスターハウスは別）。",
	EffectBigStomach:   "最大満腹度が %d 上がる。",
	EffectBargain:      "店の商品が %d%% 安くなる（値段で正体を見分けにくくなる）。",
	EffectTrapper:      "罠がすべて見え、歩いて罠を踏まない。敵は罠を踏むようになる。",
	EffectPlain:        "道具を入れられるが、中の道具は使えず、割らないと取り出せない。",
	EffectBottomless:   "入れた道具は消える（入れるたびに容量が 1 つ減る）。割れると、その場所に落とし穴ができる。",
	EffectSynthesis: "武器と盾は、先に入れた同じ種類のものに印と修正値が合成される。" +
		"同じ腕輪を入れるとひびが直る。中のものは出せない。",
	EffectHealPot:     "道具を入れると HP が全部回復する。入れた道具はなくなり、壺の容量も 1 つ減る。",
	EffectAppraise:    "入れた道具が識別される（修正値や回数、呪いも分かる）。取り出せる。",
	EffectStronger:    "入れた武器と盾の修正値が上がる（ほかの道具はそのまま）。取り出せる。",
	EffectWeaker:      "入れた武器と盾の修正値が下がる（ほかの道具はそのまま）。取り出せる。",
	EffectCash:        "入れた道具がお金（店の買い取り値）になる。取り出せる。",
	EffectChange:      "入れた道具が別の道具に変わる（変わる先は専用の表から選ばれる）。取り出せる。",
	EffectTunnel:      "正面の壁を %d マス掘って通路にする（フロアの端、店、敵の手前まで）。",
	EffectSlash:       "部屋の敵（通路では隣の敵）に %d のダメージを与える。",
	EffectWipe:        "部屋の敵（通路では隣の敵）を倒す。店主には効かない。",
	EffectNoTraps:     "フロアの罠がすべて消える。",
	EffectSeer:        "そのフロアにいる間、フロアの敵の居場所が分かる。",
	EffectGather:      "フロアの道具（店の商品を除く）が自分の周りに集まる。",
	EffectLost:        "フロアのどこかへワープする。",
	EffectOnigiri:     "選んだ持ち物 1 つ（装備中の物を除く）がおにぎりになる。",
	EffectPotUp:       "選んだ壺の容量が 1 つ増える。",
	EffectSuck:        "選んだ壺の中身がすべて足元の周りに出てくる（どの壺でも）。",
	EffectBless:       "選んだ持ち物 1 つを祝福する。呪われた物は、呪いが解ける。",
	EffectGatherFoes:  "フロアの敵（店主を除く）を自分の周りに集める。",
	EffectTrouble:     "HP が全回復し、困っていることを 1 つ（当てはまる中からランダムに）解決する：敵 2 体以上に囲まれている（隣の敵をかなしばり）、満腹度が半分以下（満腹にする）、呪われた物を装備している（装備をすべて解呪）、悪い状態（治す）、ちからが下がっている（最大まで戻す）。",
	EffectPowerUp:     "%d ターンの間、攻撃で与えるダメージが 1.5 倍になる。",
	EffectPrevent:     "悪い状態（混乱、鈍足、睡眠、口封じ、目つぶし、弱り）が治り、そのフロアにいる間は防ぐ。",
	EffectBind:        "隣の敵（壁の中の敵も）をかなしばりにする。攻撃されると解ける。",
	EffectZowa:        "隣の敵が %d ターンの間、自分を攻撃しようとすると反対の向きを攻撃する（そこに敵がいれば同士打ち）。その敵は特技を使わなくなる。",
	EffectBigRoom:     "フロアが 1 つの大部屋になる。店主は階段の横へ（代金を払っていなければ階段の上へ）移る。",
	EffectMonsterRoom: "今いる部屋がモンスターハウスになる（眠った敵と道具が出る）。経験値や道具を稼げる。",
	EffectCurse:       "持ち物を %d つまで呪う（祝福された物は祝福が消える）。呪い力の腕輪と組み合わせられる。",
	EffectTraps:       "周りのマスに罠ができる（見えている）。罠使いの腕輪や、敵を罠にかけるのに使える。",
	EffectNoPickup:    "そのフロアにいる間、道具を拾えなくなる（お金も）。",
	EffectMute:        "口がふさがり、そのフロアにいる間、草を飲んだり巻物を読んだりできなくなる。",
	EffectSanctuary:   "読んでも何も起こらない。床に置くと貼りつき（もう拾えない）、その上にいる間は敵が攻撃してこない。",
	EffectBlank:       "読むと、知っている（識別した）巻物の名前を書いて、その巻物として読める。",
	EffectPlate:       "選んだ武器か盾にサビよけの印【錆】が付く（枠が空いていて、サビよけのない物）。",
}

// magicTexts say what the effect of a bolt does to its target ("%s").
var magicTexts = map[MagicEffect]string{
	MagicWarp:    "%sをどこかへワープさせる。",
	MagicHaste:   "%sを速くする。",
	MagicBlow:    "%sを 10 マス吹き飛ばす。",
	MagicConfuse: "%sを混乱させる。",
	MagicSlow:    "%sを遅くする。",
	MagicSleep:   "%sを眠らせる。",
	MagicSeal:    "%sを封印する（敵は特技が、自分は口が封じられる）。",
	MagicHalf:    "%sの HP を半分にする。",
	MagicQuarter: "%sの HP を 1/4 にする。",
	MagicLevel1:  "%sのレベルを 1 下げる。",
	MagicLevel3:  "%sのレベルを 3 下げる。",
	MagicBlind:   "%sの目を見えなくする。",
	MagicBind:    "%sをかなしばりにする（敵は攻撃を受けるまで動けない。自分と仲間は、HP が減るか 20 ターンで解ける）。",
	MagicLevelUp: "%sのレベルを 1 上げる。",
	MagicUnhurt:  "%sを 20 ターンの間無敵にする（ダメージを受けない）。",
	MagicPoison:  "%sに 5 のダメージを与え、自分ならちからを 1、敵なら攻撃力を半分に下げて遅くする。",
}

// magicTargets are whom the effect of a bolt takes, by the category of
// the item that has it.
var magicTargets = map[Category]string{
	CategoryGrass:  "飲んだ自分か、投げて当たった相手",
	CategoryScroll: "読むと、部屋の敵（通路では隣の敵）",
	CategoryStaff:  "振ると、魔法弾が当たった相手",
}

// sealTexts say what each seal does ("%d" is its strength).
var sealTexts = map[SealEffect]string{
	SealAttack:      "攻撃力 +%d。",
	SealDefense:     "防御力 +%d。",
	SealCrit:        "%d%% の確率で会心の一撃（2 倍のダメージ）。",
	SealSure:        "外れるはずの攻撃の %d%% が当たる。",
	SealCounter:     "%d%% の確率で受けたダメージを相手に返す。",
	SealNoRust:      "武器と盾がサビない（サビを吹きかける敵には、代わりに印を消される）。",
	SealNoHunger:    "お腹が減る速さが %d%% 遅くなる。",
	SealThree:       "正面に加えて、左右の斜め前にも %d%% のダメージで攻撃する。",
	SealNoTheft:     "%d%% の確率で盗みを防ぐ。",
	SealDodge:       "敵の攻撃が当たる確率が %d%% 下がる。",
	SealCritGuard:   "敵の痛恨の一撃で受けるダメージを %d%% 減らす。",
	SealDodgeThrown: "飛んできた道具（矢を含む。貫通の矢とはね返されたものは除く）を %d%% の確率でよけ、足元に落とす（足元の罠は発動する）。",
	SealSleepBlow:   "正面の敵に当てると、%d%% の確率で眠らせる。",
	SealConfuseBlow: "正面の敵に当てると、%d%% の確率で混乱させる。",
	SealSlowBlow:    "正面の敵に当てると、%d%% の確率で鈍足にする。",
	SealSealBlow:    "正面の敵に当てると、%d%% の確率で能力を封印する。",
	SealPoisonBlow:  "正面の敵に当てると、%d%% の確率で毒（ダメージと、攻撃力半分、鈍足）。",
	SealBindBlow:    "正面の敵に当てると、%d%% の確率でかなしばりにする。",
	SealBlowAway:    "正面の敵に当てると、%d%% の確率で吹き飛ばす。",
	SealDig:         "正面の壁を攻撃すると掘れる（フロアの端と店は掘れない）。",
	SealDesperate:   "HP が最大の 1/3 以下のとき、与えるダメージが %d%% 増える。",
	SealHappy:       "敵を倒して得る経験値が %d%% 増える。",
	SealDouble:      "%d%% の確率で、正面の敵をもう一度攻撃する。",
	SealGrow:        "装備して階段を降りると、%d%% の確率で修正値が +1（+20 まで）。",
	SealNoSleep:     "%d%% の確率で睡眠を防ぐ。",
	SealNoConfuse:   "%d%% の確率で混乱を防ぐ。",
	SealNoBlind:     "%d%% の確率で目つぶしを防ぐ。",
	SealNoWeak:      "%d%% の確率で弱り（攻撃力・防御力の低下）を防ぐ。",
	SealNoCurse:     "%d%% の確率で呪いを防ぐ。",
	SealNoStarve:    "%d%% の確率で、満腹度を吸われるのと腹減りの罠を防ぐ。",
	SealNoTrap:      "%d%% の確率で、歩いて踏んだ罠が発動しない。",
	SealBlastGuard:  "爆発で受けるダメージを %d%% 減らす。",
	SealReflect:     "敵が振った杖の魔法弾を、%d%% の確率ではね返す。",
	SealHeavy:       "お腹が減る速さが %d%% 速くなる。",
	SealFine:        "HP が満タンのとき、受けるダメージを %d%% 減らす。",
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
		if m, ok := magicTexts[MagicEffect(def.Effect)]; ok && castable(def.Category, MagicEffect(def.Effect)) {
			return fmt.Sprintf(m, magicTargets[def.Category])
		}
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
		lines = append(lines, fmt.Sprintf("印 %d/%d", len(it.Seals), it.slots()))
		if it.curseKnown() {
			lines = append(lines, "呪われている（装備すると外せない）")
		}
		if sd := g.Dungeon.SealOn(def.Seal, def.Category); sd != nil {
			merged := fmt.Sprintf("　別の%sに合成すると【%s】：%s", def.Category, sd.Glyph, withStrength(sealTexts[sd.Effect], sd.AsSeal))
			if sd.AsSeal == 0 {
				merged = "　別の" + string(def.Category) + "に合成しても、印にはならない。"
			}
			lines = append(lines,
				fmt.Sprintf("[%s]素の能力：%s", sd.Glyph, withStrength(sealTexts[sd.Effect], sd.Strength)), merged)
		}
		for _, s := range it.Seals {
			sd := g.Dungeon.SealOn(s, def.Category)
			if sd == nil {
				continue
			}
			lines = append(lines, fmt.Sprintf("【%s】%s：%s", s, sd.Name, withStrength(sealTexts[sd.Effect], sd.AsSeal)))
		}
	case CategoryArrow:
		lines = append(lines, fmt.Sprintf("攻撃力 %d。%d 本。装備すると撃てる。", def.Strength, it.Count))
	case CategoryBracelet:
		if sd := g.Dungeon.Seal(def.Seal); sd != nil && def.Effect == "" {
			lines = append(lines, usePrefixes[def.Category]+withStrength(sealTexts[sd.Effect], sd.Strength))
		} else {
			lines = append(lines, effectText(def))
		}
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
	if it.curseKnown() && !it.worn() {
		lines = append(lines, "呪われている（使えない）")
	}
	if it.blessKnown() {
		switch def.Category {
		case CategoryGrass, CategoryScroll, CategoryFood:
			lines = append(lines, "祝福されている（一度使ってもなくならない。悪い効果を一度防ぐ）")
		case CategoryStaff:
			lines = append(lines, "祝福されている（振っても回数が減らない。悪い効果を一度防ぐ）")
		default:
			lines = append(lines, "祝福されている（悪い効果を一度防ぐ）")
		}
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
				withStrength(sealTexts[s.Effect], s.AsSeal)))
		}
	}
	return lines
}

// showInfo shows the description of it.
func (g *Game) showInfo(it *Item) {
	lines := g.info(it)
	g.Prompt = &Prompt{Title: commandInfo, Choices: lines, kind: promptInfo}
}
