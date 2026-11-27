package rogue

import (
	"slices"
	"strings"
)

// SealDef is a seal (印) of weapons and shields.
type SealDef struct {
	// Glyph is the kanji of the seal.
	Glyph  string
	Name   string
	Effect SealEffect
	// Strength is how much its effect does as the ability a weapon or a
	// shield is born with (items.txt 印: its 素の能力), and AsSeal as a
	// seal merged into another (seals.txt 印の強さ; weaker, so that the
	// one born with it is worth using as it is).
	Strength, AsSeal int
	// For is the category it goes on, a weapon or a shield (付く先; ""
	// for either, when seals.txt does not say). A seal goes on one of
	// them, so that the two never add up the same effect.
	For Category
	// Target is the attribute of the monsters a seal of slaying (特攻)
	// goes for (seals.txt 対象, monsters.txt 属性); "" for the others.
	Target string
}

// SealEffect is what a seal does while its weapon or shield is equipped.
type SealEffect string

// The effects of seals.
const (
	SealAttack   SealEffect = "攻撃"
	SealDefense  SealEffect = "防御"
	SealCrit     SealEffect = "会心"
	SealSure     SealEffect = "必中"
	SealCounter  SealEffect = "反撃"
	SealNoRust   SealEffect = "サビよけ"
	SealNoHunger SealEffect = "節食"
	// SealThree: a blow also strikes the two tiles beside the one the
	// player faces, ahead of it.
	SealThree SealEffect = "三方向"
	// SealNoTheft: nothing is stolen from the player.
	SealNoTheft SealEffect = "盗みよけ"
	// SealDodge: the blows of the monsters hit so many percent less
	// often.
	SealDodge SealEffect = "見切り"
	// SealDodgeThrown: the player dodges an item thrown or shot at it
	// (not a piercing arrow, nor what a mirror sends back) by a chance of
	// its %; the item falls at its feet (dodgeThrown).
	SealDodgeThrown SealEffect = "身躱し"
	// SealCritGuard: a critical blow on the player (痛恨) does its %
	// less damage.
	SealCritGuard SealEffect = "防恨"

	// The seals of weapons that came with the weapons of 2026-11-28.
	// A blow that hits (the one ahead, not the side blows of 三方向 nor
	// the second of 連撃) brings, by a chance of its %, the state of the
	// bolt of the same name on the monster (sealBolts).
	SealSleepBlow   SealEffect = "睡眠"
	SealConfuseBlow SealEffect = "混乱"
	SealSlowBlow    SealEffect = "鈍足"
	SealSealBlow    SealEffect = "封印"
	SealPoisonBlow  SealEffect = "毒"
	SealBindBlow    SealEffect = "かなしばり"
	SealBlowAway    SealEffect = "吹き飛ばし"
	// SealDig: a blow at the rock ahead digs it into a corridor (not the
	// edge of the floor, nor a shop).
	SealDig SealEffect = "掘る"
	// SealDesperate: with its HP at a third of the max or less, the
	// player's blows do its % more.
	SealDesperate SealEffect = "捨て身"
	// SealHappy: the EXP of the defeats is its % more.
	SealHappy SealEffect = "しあわせ"
	// SealDouble: by a chance of its %, the player strikes the one ahead
	// a second time in the turn (Shougo: by a chance, not always).
	SealDouble SealEffect = "連撃"
	// SealGrow: going down the stairs with it equipped, by a chance of
	// its %, the upgrade value of its weapon or shield goes up by one,
	// up to growCap (Shougo, 2026-11-28).
	SealGrow SealEffect = "成長"

	// The seals of shields of 2026-11-28. A ward (Shougo: 100% born with
	// it, 50% as a seal) keeps a thing off by a chance of its %.
	SealNoSleep   SealEffect = "睡眠よけ"
	SealNoConfuse SealEffect = "混乱よけ"
	SealNoBlind   SealEffect = "目つぶしよけ"
	SealNoWeak    SealEffect = "弱りよけ"
	SealNoCurse   SealEffect = "呪いよけ"
	SealNoStarve  SealEffect = "空腹よけ"
	SealNoTrap    SealEffect = "罠よけ"
	// SealBlastGuard: an explosion does its % less damage to the player.
	SealBlastGuard SealEffect = "耐爆"
	// SealReflect: by a chance of its %, the bolt of a staff waved at the
	// player by a monster goes back to it.
	SealReflect SealEffect = "反射"
	// SealHeavy: the player gets hungry its % faster (a heavy shield).
	SealHeavy SealEffect = "腹減り"
	// SealFine: with its HP full, the player takes its % less damage
	// (絶好調; Shougo: only while full, so the first blow after resting).
	SealFine SealEffect = "絶好調"
	// SealSlay: the blows of the player do its % more to the monsters of
	// its Target (特攻).
	SealSlay SealEffect = "特攻"
	// SealReach: with no one next to it ahead, a blow reaches the monster
	// two tiles ahead, for its % of the damage (長槍).
	SealReach SealEffect = "間合い"
	// SealFireGuard: fire (the breath of 龍, 火炎草 thrown at it) does its
	// % less damage to the player (龍鱗の盾).
	SealFireGuard SealEffect = "耐火"
	// SealNoSealing: a ward of the sealing of the player (口封じ), from
	// any source: a bolt, a monster, a trap, an item (Shougo, 2026-11-28;
	// a seal taken away is another thing).
	SealNoSealing SealEffect = "封印よけ"
)

var sealEffects = []SealEffect{SealAttack, SealDefense, SealCrit, SealSure, SealCounter, SealNoRust, SealNoHunger,
	SealThree, SealNoTheft, SealDodge, SealDodgeThrown, SealCritGuard,
	SealSleepBlow, SealConfuseBlow, SealSlowBlow, SealSealBlow, SealPoisonBlow, SealBindBlow, SealBlowAway,
	SealDig, SealDesperate, SealHappy, SealDouble, SealGrow,
	SealNoSleep, SealNoConfuse, SealNoBlind, SealNoWeak, SealNoCurse, SealNoStarve, SealNoTrap,
	SealBlastGuard, SealReflect, SealHeavy, SealFine, SealSlay, SealReach, SealFireGuard, SealNoSealing}

// sealBolts are the seals of weapons that bring the effect of a bolt with
// a blow, in a fixed order.
var sealBolts = []struct {
	seal SealEffect
	bolt MagicEffect
}{
	{SealSleepBlow, MagicSleep}, {SealConfuseBlow, MagicConfuse}, {SealSlowBlow, MagicSlow},
	{SealSealBlow, MagicSeal}, {SealPoisonBlow, MagicPoison}, {SealBindBlow, MagicBind},
	{SealBlowAway, MagicBlow},
}

// growCap is the upgrade value up to which 成長 raises an item (Shougo,
// 2026-11-28: for now).
const growCap = 20

// No seal heals: not on a blow given, not on a blow taken. Such seals
// (弟 and 回 of Shiren 2 and Asuka, stacked) make trading blows pay, and a
// run is then hard until they are made and easy after: a cliff, not a
// curve (claude/rogue-seal-stacking-2026-11-20.md).

// sealsLabel shows the innate ability and the seals of it, if any:
// "[必]【会会】".
func sealsLabel(it *Item) string {
	s := ""
	if it.Def.Seal != "" && it.upgradable() {
		s = "[" + it.Def.Seal + "]"
	}
	if len(it.Seals) > 0 {
		s += "【" + strings.Join(it.Seals, "") + "】"
	}
	return s
}

// Innate abilities and seals (Shougo, 2026-11-24). The ability a weapon
// or a shield is born with (items.txt 印) is not a seal: it takes no
// slot, it stays when the item is the base of a synthesis, and it works
// in full while the item is equipped. Merged into another, it becomes a
// seal of that one, weaker (seals.txt 印の強さ): which weapon to wear is
// then a choice, and many strong items found do not make a run easy.
// The same seal may be there more than once: an attack or a defense
// adds up; the others are chances (or shares) that add up as chances
// do, 1 - (1 - a)(1 - b): closer to the innate ability with each, never
// past it (two seals of 必中 at 50% leave a quarter of the misses).

// anAbility is the innate ability or a seal of an equipped item.
type anAbility struct {
	def    *SealDef
	innate bool
}

// strength is how much it does.
func (a anAbility) strength() int {
	if a.innate {
		return a.def.Strength
	}
	return a.def.AsSeal
}

// sealStrength is the total strength of the abilities of the effect e of
// the equipped weapon and shield (0 for none): for the effects that add
// up (攻撃, 防御).
func (g *Game) sealStrength(e SealEffect) int {
	n := 0
	for _, a := range g.equippedAbilities() {
		if a.def.Effect == e {
			n += a.strength()
		}
	}
	return n
}

// sealShare is the share (0 to 1) that the abilities of the effect e of
// the equipped weapon and shield make up together, their strengths as
// percents of chances that add up as chances do.
func (g *Game) sealShare(e SealEffect) float64 {
	not := 1.0
	for _, a := range g.equippedAbilities() {
		if a.def.Effect == e {
			not *= 1 - float64(min(a.strength(), 100))/100
		}
	}
	return 1 - not
}

// sealChance says, by the share of the abilities of the effect e, whether
// it works this time.
func (g *Game) sealChance(e SealEffect) bool {
	share := g.sealShare(e)
	return share >= 1 || share > 0 && g.rng.Float64() < share
}

// slayShare is how much more (a share) the blows of the player do to m
// by the seals of slaying of its attributes, adding up as chances do.
func (g *Game) slayShare(m *Monster) float64 {
	not := 1.0
	for _, a := range g.equippedAbilities() {
		if a.def.Effect == SealSlay && slices.Contains(m.Def.Attrs, a.def.Target) {
			not *= 1 - float64(min(a.strength(), 100))/100
		}
	}
	return 1 - not
}

// itemShare is the share of the abilities of the effect e of the item it
// alone (its innate ability and its seals), as sealShare.
func (g *Game) itemShare(it *Item, e SealEffect) float64 {
	not := 1.0
	c := it.Def.Category
	if s := g.Dungeon.SealOn(it.Def.Seal, c); s != nil && s.Effect == e {
		not *= 1 - float64(min(s.Strength, 100))/100
	}
	for _, glyph := range it.Seals {
		if s := g.Dungeon.SealOn(glyph, c); s != nil && s.Effect == e {
			not *= 1 - float64(min(s.AsSeal, 100))/100
		}
	}
	return 1 - not
}

// grow is 成長 going down the stairs: the equipped weapon and shield
// with it go up by one, each by its own chance, up to growCap.
func (g *Game) grow() {
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		it := g.equipped(c)
		if it == nil || it.Upgrade >= growCap {
			continue
		}
		if share := g.itemShare(it, SealGrow); share > 0 && g.rng.Float64() < share {
			it.Upgrade++
			g.say(g.Name(it) + "が成長した！")
		}
	}
}

// equippedAbilities are the innate abilities and the seals of the
// equipped weapon and shield, and the ability of the equipped bracelet.
func (g *Game) equippedAbilities() []anAbility {
	var as []anAbility
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		it := g.equipped(c)
		if it == nil {
			continue
		}
		if s := g.Dungeon.SealOn(it.Def.Seal, c); s != nil {
			as = append(as, anAbility{s, true})
		}
		for _, glyph := range it.Seals {
			if s := g.Dungeon.SealOn(glyph, c); s != nil {
				as = append(as, anAbility{s, false})
			}
		}
	}
	// A bracelet with a seal (会心の腕輪...) has its ability at the
	// strength it is born with, as a weapon or a shield.
	if b := g.equipped(CategoryBracelet); b != nil {
		if s := g.Dungeon.Seal(b.Def.Seal); s != nil {
			as = append(as, anAbility{s, true})
		}
	}
	return as
}
