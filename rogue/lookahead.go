package rogue

import (
	"math/rand/v2"
	"slices"
)

// Look-ahead (after AlphaDefect's search at the hard moments only): when
// the reckoning of the chance of dying tells the answers to a danger no
// more apart (all hopeless), the bot (with LookAhead) tries each answer it knows
// for sure on copies of the game (Clone), the rest of the randomness
// drawn anew each time, plays on by its rules for some turns, and takes
// the answer that lives on best. The copies know what the player does
// not (the kinds not identified, monsters not seen): the bot does not
// try the items it is unsure of, but the monsters it does not see still
// act in the copies as they would. A first step, to measure.

const (
	// lookTries is how many copies an answer is tried on.
	lookTries = 6
	// lookTurns is how many turns a copy is played on.
	lookTurns = 20
	// lookAnswers is how many answers are tried at most, the cheapest.
	lookAnswers = 8
)

// lookAhead returns the answer that lives on best in the copies, when
// the one picked (pick, nil for none) is hopeless; with false, the pick
// stands.
func (b *Bot) lookAhead(g *Game, answers []botAnswer, pick *botAnswer) (*botAnswer, bool) {
	if !b.LookAhead || b.rollout || g.pcg == nil || pick != nil && pick.chance < hopeless {
		return nil, false
	}
	var cands []*botAnswer
	for i := range answers {
		if !answers[i].unsure {
			cands = append(cands, &answers[i])
		}
	}
	slices.SortStableFunc(cands, func(x, y *botAnswer) int {
		switch {
		case x.cost < y.cost:
			return -1
		case x.cost > y.cost:
			return 1
		}
		return 0
	})
	cands = cands[:min(len(cands), lookAnswers)]
	// Fighting on (no answer) is a candidate too.
	cands = append(cands, nil)
	var best *botAnswer
	bestScore := -1.0
	for _, a := range cands {
		score := 0.0
		for k := range lookTries {
			score += b.tryAnswer(g, a, uint64(k))
		}
		if score > bestScore {
			best, bestScore = a, score
		}
	}
	return best, true
}

// tryAnswer plays the answer a (nil: fighting on) on a copy of g, the
// randomness from k, then the rules for lookTurns turns, and scores how
// it went: 1 and a little more for the HP left alive, less than 0.5 dead,
// more the later.
func (b *Bot) tryAnswer(g *Game, a *botAnswer, k uint64) float64 {
	c := g.Clone()
	pcg := rand.NewPCG(uint64(g.Turn)*7919+k, k)
	c.pcg, c.rng = pcg, rand.New(pcg)
	r := NewBot(b.Style, k)
	r.rollout = true
	start := c.Turn
	if a != nil {
		ca := translateAnswer(g, c, a)
		for range 3 {
			action, args := r.take(c, ca)
			if action == "" || c.Do(action, args) != nil {
				break
			}
			for c.Prompt != nil && c.Over == "" {
				pa, pargs := r.Next(c)
				if c.Do(pa, pargs) != nil {
					break
				}
			}
			if action != "turn" {
				break
			}
		}
	}
	for n := 0; c.Over == "" && c.Turn-start < lookTurns && n < 4*lookTurns; n++ {
		action, args := r.Next(c)
		if c.Do(action, args) != nil {
			break
		}
	}
	if c.Over != "" && !c.Cleared {
		return 0.5 * float64(c.Turn-start) / lookTurns
	}
	return 1 + 0.5*float64(c.HP)/float64(max(c.MaxHP, 1))
}

// translateAnswer is a, an answer in g, in its copy c: the same items and
// monsters, by their places in the lists.
func translateAnswer(g, c *Game, a *botAnswer) *botAnswer {
	t := *a
	item := func(it *Item) *Item {
		if i := slices.Index(g.Inventory, it); i >= 0 {
			return c.Inventory[i]
		}
		return nil
	}
	if a.item != nil {
		t.item = item(a.item)
	}
	if a.fuel != nil {
		t.fuel = item(a.fuel)
	}
	if a.target != nil {
		if i := slices.Index(g.Monsters, a.target); i >= 0 {
			t.target = c.Monsters[i]
		}
	}
	return &t
}
