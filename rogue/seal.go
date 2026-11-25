package rogue

import (
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
	SealNoHunger SealEffect = "ハラヘラズ"
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
)

var sealEffects = []SealEffect{SealAttack, SealDefense, SealCrit, SealSure, SealCounter, SealNoRust, SealNoHunger,
	SealThree, SealNoTheft, SealDodge, SealDodgeThrown}

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
