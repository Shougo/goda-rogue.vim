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
	// Strength is how much its effect does.
	Strength int
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
)

var sealEffects = []SealEffect{SealAttack, SealDefense, SealCrit, SealSure, SealCounter, SealNoRust, SealNoHunger,
	SealThree, SealNoTheft, SealDodge}

// No seal heals: not on a blow given, not on a blow taken. Such seals
// (弟 and 回 of Shiren 2 and Asuka, stacked) make trading blows pay, and a
// run is then hard until they are made and easy after: a cliff, not a
// curve (claude/rogue-seal-stacking-2026-11-20.md).

// sealsLabel shows the seals of it, if any: "【会必】".
func sealsLabel(it *Item) string {
	if len(it.Seals) == 0 {
		return ""
	}
	return "【" + strings.Join(it.Seals, "") + "】"
}

// sealStrength is the total strength of the seals of the effect e on the
// equipped weapon and shield (0 for none).
func (g *Game) sealStrength(e SealEffect) int {
	n := 0
	for _, s := range g.equippedSeals() {
		if s.Effect == e {
			n += s.Strength
		}
	}
	return n
}

// sealed says whether the equipped weapon or shield has a seal of the
// effect e.
func (g *Game) sealed(e SealEffect) bool {
	return slices.ContainsFunc(g.equippedSeals(), func(s *SealDef) bool { return s.Effect == e })
}

// equippedSeals are the seals of the equipped weapon and shield.
func (g *Game) equippedSeals() []*SealDef {
	var seals []*SealDef
	for _, c := range []Category{CategoryWeapon, CategoryShield} {
		if it := g.equipped(c); it != nil {
			for _, glyph := range it.Seals {
				if s := g.Dungeon.SealOn(glyph, c); s != nil {
					seals = append(seals, s)
				}
			}
		}
	}
	return seals
}
