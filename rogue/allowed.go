package rogue

import "slices"

// Where an item is, for what may be done with it (allowed).
type itemPlace int

const (
	placeCarried itemPlace = iota
	placeFoot
	placePot
)

// freeOfShop are the commands that may be done with an item of the shop
// not paid for, by where it is: put down, equipped, taken off when
// carried (buying and selling), picked up from the floor, taken out of a
// pot of the shop.
var freeOfShop = map[itemPlace][]string{
	placeCarried: {commandDrop, commandEquip, commandUnequip, commandInfo},
	placeFoot:    {commandPickUp, commandPurse, commandSwap, commandInfo},
	placePot:     {commandTakeOut, commandInfo},
}

// mimicSafe are the commands that leave a mimic hidden, by where it is:
// reading about it; on the floor, picking it up and swapping (they test
// it themselves, pickUp and swap).
var mimicSafe = map[itemPlace][]string{
	placeCarried: {commandInfo},
	placeFoot:    {commandInfo, commandPickUp, commandSwap},
	placePot:     {commandInfo},
}

// allowed checks, before the command c is done with it (at where; in the
// pot pot, for inPot), what may stop it: a mimic shows itself, the shop
// wants its money, a sealed mouth cannot eat, drink nor read. It says
// whether to go on, and if not, whether the turn passes. Every command on
// an item comes here first.
func (g *Game) allowed(it, pot *Item, where itemPlace, c string) (ok, turn bool) {
	if it.Mimic != nil && !slices.Contains(mimicSafe[where], c) {
		// Whatever is done with a mimic, it shows itself.
		g.unmask(it, true)
		return false, true
	}
	if (it.ForSale || pot != nil && pot.ForSale) && !slices.Contains(freeOfShop[where], c) {
		g.say("店主「お代を払ってからにしてください。」")
		return false, false
	}
	if g.mouthBlocks(c) {
		return false, false
	}
	return true, false
}
