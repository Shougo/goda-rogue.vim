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
)

var sealEffects = []SealEffect{SealAttack, SealDefense, SealCrit, SealSure, SealCounter, SealNoRust, SealNoHunger}

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
				if s := g.Dungeon.Seal(glyph); s != nil {
					seals = append(seals, s)
				}
			}
		}
	}
	return seals
}
