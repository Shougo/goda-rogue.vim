package rogue

import (
	"fmt"
	"sort"
	"strings"
)

// Comparing a replay with a bot: at every decision of the player, what
// would the bot have done there? Where both do a thing of the same kind
// (a step, whatever its direction; a blow) nothing is said; where they
// differ (the player threw a grass and the bot would have fought, the
// player went down and the bot would have explored on) the decision is
// listed, with the state it was taken in. It shows where the bot plays
// another way than a player, to make it play better.

// Difference is a decision of the player where the bot would have done
// a thing of another kind.
type Difference struct {
	// Depth, Turn, HP and MaxHP are the state before the decision;
	// Enemies how many enemies the player saw, Adjacent how many were
	// next to it, Carried how many items it carried.
	Depth, Turn, HP, MaxHP     int
	Enemies, Adjacent, Carried int
	// Player and Bot are what each did or would do: its kind (move,
	// attack, a command of an item) and, of an item, which.
	Player, Bot decision
	// Rule is the rule of the bot that chose (botRules).
	Rule string
	// Times is how many decisions in a row were alike (the same floor,
	// kinds and rule), from Turn to Last.
	Times, Last int
}

// decision is a thing done: its kind, and what with.
type decision struct {
	Kind, With string
}

func (d decision) String() string {
	if d.With == "" {
		return d.Kind
	}
	return d.With + "を" + d.Kind
}

// actionKinds are the kinds of the actions that are not commands of
// items.
var actionKinds = map[string]string{
	"move": "移動", "dash": "移動", "attack": "攻撃", "shoot": "矢を撃つ", "stairs": "階段を降りる",
	"wait": "足踏み", "pickup": "拾う", "turn": "向きを変える",
}

// CompareBot plays the replay rp in the dungeon d, and returns the
// decisions where a bot of style would have done a thing of another kind
// than the player, and how many decisions there were. A decision is an
// action that takes a turn, with the answers to the prompts it opens;
// those that take none (looking, turning, a prompt cancelled) are not
// compared.
func CompareBot(rp *Replay, d *Dungeon, style BotStyle) ([]Difference, int, error) {
	g, err := rp.NewReplayGame(d)
	if err != nil {
		return nil, 0, err
	}
	b := NewBot(style, 0)
	var diffs []Difference
	n := 0
	for i := 0; i < len(rp.Actions) && g.Over == ""; {
		if rp.Actions[i].Action == replaySuspend || g.Prompt != nil {
			// (A prompt left open by the floor: not a decision.)
			if err := g.replayAction(rp.Actions[i]); err != nil {
				return nil, 0, err
			}
			i++
			continue
		}
		df := Difference{Depth: g.Depth, Turn: g.Turn, HP: g.HP, MaxHP: g.MaxHP,
			Enemies: len(g.visibleEnemies()), Carried: len(g.Inventory), Times: 1, Last: g.Turn}
		for _, m := range g.visibleEnemies() {
			if chebyshev(m.Pos, g.Player) <= 1 {
				df.Adjacent++
			}
		}
		// What the bot would do, before the player does its thing.
		action, _ := b.Next(g)
		df.Bot, df.Rule = botDecision(g, b, action), b.rule
		b.plan = nil

		// What the player did: the action, and the answers to its
		// prompts.
		turn, depth := g.Turn, g.Depth
		a := rp.Actions[i]
		df.Player = decision{Kind: actionKinds[a.Action]}
		if df.Player.Kind == "" {
			df.Player.Kind = a.Action
		}
		for {
			var label, item string
			if p := g.Prompt; p != nil && a.Action == "choose" && len(a.Args) == 1 {
				var c int
				if _, err := fmt.Sscan(a.Args[0], &c); err == nil && c >= 0 && c < len(p.Choices) {
					label = p.Choices[c]
					if p.kind == promptInventory && c < len(p.items) && p.items[c] >= 0 && p.items[c] < len(g.Inventory) {
						item = g.Name(g.Inventory[p.items[c]])
					}
				}
			}
			if err := g.replayAction(a); err != nil {
				return nil, 0, err
			}
			i++
			switch {
			case item != "":
				df.Player.With = item
			case label != "":
				df.Player.Kind = label
			}
			if g.Prompt == nil || g.Over != "" || i >= len(rp.Actions) || g.Turn != turn || g.Depth != depth {
				break
			}
			a = rp.Actions[i]
		}
		if g.Turn == turn && g.Depth == depth {
			// No turn passed: looking, turning, a prompt cancelled.
			continue
		}
		n++
		if df.Player.Kind == df.Bot.Kind && (df.Player.With == df.Bot.With || df.Player.With == "" || df.Bot.With == "") {
			continue
		}
		if k := len(diffs); k > 0 && diffs[k-1].Depth == df.Depth && diffs[k-1].Player == df.Player &&
			diffs[k-1].Bot == df.Bot && diffs[k-1].Rule == df.Rule && df.Turn-diffs[k-1].Last <= 2 {
			diffs[k-1].Times++
			diffs[k-1].Last = df.Turn
			continue
		}
		diffs = append(diffs, df)
	}
	return diffs, n, nil
}

// botDecision returns what the bot decided with action: the action, or
// the command of its plan, with its item.
func botDecision(g *Game, b *Bot, action string) decision {
	switch action {
	case "inventory":
		if len(b.plan) == 2 && b.plan[0].item != nil {
			return decision{Kind: b.plan[1].label, With: g.Name(b.plan[0].item)}
		}
	case "foot":
		if len(b.plan) == 1 {
			with := "足元の物"
			if it := g.foot(); it != nil {
				with = "足元の" + g.Name(it)
			}
			return decision{Kind: b.plan[0].label, With: with}
		}
	}
	if k := actionKinds[action]; k != "" {
		return decision{Kind: k}
	}
	return decision{Kind: action}
}

// compareLines returns the report of the differences diffs of a replay
// of n decisions: how many, which kinds how often, and each one.
func compareLines(rp *Replay, name string, style BotStyle, diffs []Difference, n int) []string {
	total := 0
	kinds := map[string]int{}
	for _, d := range diffs {
		total += d.Times
		kinds[fmt.Sprintf("%s → %s［%s］", d.Player.Kind, d.Bot.Kind, d.Rule)] += d.Times
	}
	lines := []string{
		fmt.Sprintf("リプレイと Bot（%s）の比較  %s  シード %s", style, name, rp.Seed),
		fmt.Sprintf("判断 %d 回のうち、Bot なら違う種類の行動をした場面 %d 回（%d 件）。移動の向きの違いは数えない。", n, total, len(diffs)),
		"",
		"■ 内訳（プレイヤーの行動 → Bot の行動［Bot のルール］）",
	}
	var ks []string
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if kinds[ks[i]] != kinds[ks[j]] {
			return kinds[ks[i]] > kinds[ks[j]]
		}
		return ks[i] < ks[j]
	})
	for _, k := range ks {
		lines = append(lines, fmt.Sprintf("%6d  %s", kinds[k], k))
	}
	lines = append(lines, "", "■ 一覧（階、ターン、HP、見えている敵（うち隣）、持ち物）")
	for _, d := range diffs {
		at := fmt.Sprintf("T%d", d.Turn)
		if d.Times > 1 {
			at = fmt.Sprintf("T%d-%d ×%d", d.Turn, d.Last, d.Times)
		}
		lines = append(lines, strings.TrimRight(fmt.Sprintf("%2dF  %-16s HP %3d/%-3d  敵 %d(%d)  持 %2d  プレイヤー: %s  /  Bot: %s［%s］",
			d.Depth, at, d.HP, d.MaxHP, d.Enemies, d.Adjacent, d.Carried, d.Player, d.Bot, d.Rule), " "))
	}
	return lines
}

// CompareReport reads the replay file path, and returns the report of
// its comparison with a bot of style.
func CompareReport(path string, style BotStyle) ([]string, error) {
	rp, err := ReadReplay(path)
	if err != nil {
		return nil, err
	}
	d, err := LoadBuiltin(rp.Dungeon)
	if err != nil {
		return nil, fmt.Errorf("dungeon %q: %w", rp.Dungeon, err)
	}
	diffs, n, err := CompareBot(rp, d, style)
	if err != nil {
		return nil, err
	}
	return compareLines(rp, d.Name, style, diffs, n), nil
}
