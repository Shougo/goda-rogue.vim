package rogue

import (
	"fmt"
	"strconv"
)

// The debug mode: actions to test a dungeon or the game quickly (go to a
// floor, give an item, put a monster, ...). It is enabled by the rule
// デバッグ of the dungeon (for the makers of edit dungeons) or by the
// option debug of a new game (g:goda_rogue#debug, for developers). A run
// that used it is marked, on its status line, its result and its replay
// (the debug actions are actions like the others: a replay replays them).

// The debug commands: the first argument of the action "debug".
const (
	debugFloor      = "floor"
	debugItem       = "item"
	debugMonster    = "monster"
	debugReveal     = "reveal"
	debugIdentify   = "identify"
	debugInvincible = "invincible"
	debugGold       = "gold"
)

// debugMenu is the prompt of the action "debug" without arguments: the
// commands, as the player chooses them.
var debugMenu = []struct{ label, command string }{
	{"階を移動する", debugFloor},
	{"道具を出す", debugItem},
	{"敵を出す", debugMonster},
	{"フロアを見渡す", debugReveal},
	{"すべて識別する", debugIdentify},
	{"不死身を切り替える", debugInvincible},
	{"1000G 得る", debugGold},
}

// The prompts of the debug mode: the menu, and the floor, item or monster
// to choose.
const (
	promptDebug        = "debug"
	promptDebugFloor   = "debug floor"
	promptDebugItem    = "debug item"
	promptDebugMonster = "debug monster"
)

// debugAllowed says whether the run may use the debug mode.
func (g *Game) debugAllowed() bool { return g.DebugMode || g.Dungeon.Rules.Debug }

// debug runs the debug command args; without one, it asks which. It says
// whether the turn passes (never: the debug mode costs no turn).
func (g *Game) debug(args []string) error {
	if !g.debugAllowed() {
		return fmt.Errorf("debug: not enabled (the rule デバッグ of the dungeon, or g:goda_rogue#debug)")
	}
	if len(args) == 0 {
		p := &Prompt{Title: "デバッグ", kind: promptDebug}
		for _, c := range debugMenu {
			p.Choices = append(p.Choices, c.label)
		}
		g.Prompt = p
		return nil
	}
	arg := func(i int) (int, error) {
		if len(args) <= i {
			return 0, fmt.Errorf("debug %s: a number expected", args[0])
		}
		n, err := strconv.Atoi(args[i])
		if err != nil {
			return 0, fmt.Errorf("debug %s: %w", args[0], err)
		}
		return n, nil
	}
	name := func() (string, error) {
		if len(args) < 2 {
			return "", fmt.Errorf("debug %s: an ID expected", args[0])
		}
		return args[1], nil
	}
	switch args[0] {
	case debugFloor:
		n, err := arg(1)
		if err != nil {
			return err
		}
		if n < 1 || n > g.Dungeon.Floors {
			return fmt.Errorf("debug floor: %d is not a floor of 1 to %d", n, g.Dungeon.Floors)
		}
		g.debugUsed(args)
		g.Depth = n - 1
		g.descend()
	case debugItem:
		id, err := name()
		if err != nil {
			return err
		}
		def := g.Dungeon.Item(id)
		if def == nil {
			return fmt.Errorf("debug item: no item %q", id)
		}
		g.debugUsed(args)
		g.giveItem(def)
	case debugMonster:
		id, err := name()
		if err != nil {
			return err
		}
		def := g.Dungeon.Monster(id)
		if def == nil {
			return fmt.Errorf("debug monster: no monster %q", id)
		}
		g.debugUsed(args)
		m := g.spawnNear(def)
		if m == nil {
			g.say("敵を置く場所がない。")
			return nil
		}
		// Not a spawned one: it gives EXP.
		m.Spawned = false
		g.say(m.Name + "が現れた。")
	case debugReveal:
		g.debugUsed(args)
		g.light()
		g.say("フロアを見渡した。")
	case debugIdentify:
		g.debugUsed(args)
		for _, def := range g.Dungeon.Items {
			g.identified[def.ID] = true
		}
		for _, it := range g.Inventory {
			it.Known = true
		}
		g.say("すべての道具を識別した。")
	case debugInvincible:
		g.debugUsed(args)
		g.Invincible = !g.Invincible
		if g.Invincible {
			g.say("不死身になった。")
		} else {
			g.say("不死身ではなくなった。")
		}
	case debugGold:
		g.debugUsed(args)
		g.Gold += 1000
		g.say("1000G 得た。")
	default:
		return fmt.Errorf("debug: unknown command %q", args[0])
	}
	return nil
}

// debugUsed marks the run as one that used the debug mode.
func (g *Game) debugUsed(args []string) {
	g.Debugged = true
	g.event("debug %v", args)
}

// giveItem gives the player an item of def (it lands under the player
// when the inventory is full).
func (g *Game) giveItem(def *ItemDef) {
	it := g.newItemOf(g.rng, def, g.Player)
	// (Neither cursed nor blessed: what was asked for.)
	it.Known, it.Cursed, it.Blessed = true, false, false
	it.Upgrade = max(it.Upgrade, 0)
	if def.Category == CategoryMoney {
		g.Gold += it.Amount
		g.say(fmt.Sprintf("%dG 得た。", it.Amount))
		return
	}
	if len(g.Inventory) >= g.Dungeon.Rules.Capacity {
		g.land(it, g.Player)
		g.say(g.Name(it) + "が足元に出た。")
		return
	}
	g.Inventory = append(g.Inventory, it)
	g.sortInventory()
	g.say(g.Name(it) + "を得た。")
}

// chooseDebug answers a prompt of the debug mode with the choice n.
func (g *Game) chooseDebug(p *Prompt, n int) error {
	switch p.kind {
	case promptDebug:
		switch c := debugMenu[n].command; c {
		case debugFloor:
			q := &Prompt{Title: "何階へ？", kind: promptDebugFloor}
			for f := 1; f <= g.Dungeon.Floors; f++ {
				q.Choices = append(q.Choices, fmt.Sprintf("%dF", f))
			}
			g.Prompt = q
		case debugItem:
			q := &Prompt{Title: "どの道具？", kind: promptDebugItem}
			for _, def := range g.Dungeon.Items {
				q.Choices = append(q.Choices, def.Glyph+" "+def.Name)
			}
			g.Prompt = q
		case debugMonster:
			q := &Prompt{Title: "どの敵？", kind: promptDebugMonster}
			for _, def := range g.Dungeon.Monsters {
				q.Choices = append(q.Choices, fmt.Sprintf("%s %s（ランク%d）", def.Glyph, def.Name, def.Rank))
			}
			g.Prompt = q
		default:
			return g.debug([]string{c})
		}
	case promptDebugFloor:
		return g.debug([]string{debugFloor, strconv.Itoa(n + 1)})
	case promptDebugItem:
		return g.debug([]string{debugItem, g.Dungeon.Items[n].ID})
	case promptDebugMonster:
		return g.debug([]string{debugMonster, g.Dungeon.Monsters[n].ID})
	}
	return nil
}

// spared is the player falling while invincible (the debug mode): it
// stands up again with full HP and fullness. It says whether it did.
func (g *Game) spared() bool {
	if !g.Invincible {
		return false
	}
	g.HP, g.Fullness = g.MaxHP, g.fullnessCap()
	g.say("不死身なので倒れなかった。")
	return true
}
