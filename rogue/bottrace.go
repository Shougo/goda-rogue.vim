package rogue

import (
	"fmt"
	"strings"
)

// traceAnswers tells Trace the answers to a danger the bot weighed: the
// chance of dying fighting on, and of each answer its cost and chance,
// the one picked marked.
func (b *Bot) traceAnswers(g *Game, fight float64, answers []botAnswer, pick *botAnswer) {
	var parts []string
	for i := range answers {
		a := &answers[i]
		what := a.command
		if a.item != nil {
			what = g.Name(a.item) + " " + a.command
		}
		if what == "" {
			what = "階段へ逃げる"
		}
		mark := ""
		if a == pick {
			mark = "*"
		}
		parts = append(parts, fmt.Sprintf("%s%s(%.0f, %.0f%%)", mark, what, a.cost, 100*a.chance))
	}
	b.Trace(fmt.Sprintf("  危険：戦うと %.0f%%。%s", 100*fight, strings.Join(parts, " ")))
}

// traceAction tells Trace the action the bot chose, and why.
func (b *Bot) traceAction(g *Game, action string, args []string) {
	var near []string
	for _, m := range b.adjacent(g) {
		near = append(near, fmt.Sprintf("%s(HP%d)", g.seenName(m), m.HP))
	}
	b.Trace(fmt.Sprintf("%dF T%d HP%d/%d 満腹%d %s %s [%s] 隣%v", g.Depth, g.Turn, g.HP, g.MaxHP, g.Fullness,
		action, strings.Join(args, " "), b.rule, near))
}

// TraceBot plays a run of d with the seed by the bot b, its choices to
// trace, and returns how it ended.
func TraceBot(d *Dungeon, seed string, b *Bot, maxActions int, trace func(string)) string {
	b.Trace = trace
	g := NewGame(d, seed)
	for n := 0; n < maxActions && g.Over == ""; n++ {
		action, args := b.Next(g)
		if g.Prompt == nil {
			b.traceAction(g, action, args)
		}
		if err := g.Do(action, args); err != nil {
			return err.Error()
		}
	}
	return g.Over
}
