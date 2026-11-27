package rogue

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// debugGame is a game in the debug mode, with no monster.
func debugGame(t *testing.T) *Game {
	t.Helper()
	g, _ := roomGame(t, 4, 3)
	g.DebugMode = true
	g.Monsters = nil
	return g
}

func TestDebugDisabled(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	if err := g.Do("debug", []string{"gold"}); err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("debug without the mode: %v", err)
	}
	if g.Gold != 0 || g.Debugged {
		t.Errorf("gold %d, debugged %v", g.Gold, g.Debugged)
	}
}

func TestDebugRule(t *testing.T) {
	m := copyFS(t)
	m["rules.txt"].Data = append(m["rules.txt"].Data, "デバッグ\t可\n"...)
	d, err := LoadDungeon(m, DefaultDungeon)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Rules.Debug || cave().Rules.Debug {
		t.Fatalf("debug rule: %v, the cave's %v", d.Rules.Debug, cave().Rules.Debug)
	}
	g := NewGame(d, "rule")
	if err := g.Do("debug", []string{"gold"}); err != nil || g.Gold != 1000 {
		t.Errorf("debug by the rule: %v, gold %d", err, g.Gold)
	}
}

func TestDebugCommands(t *testing.T) {
	g := debugGame(t)
	turn := g.Turn
	do := func(args ...string) {
		t.Helper()
		if err := g.Do("debug", args); err != nil {
			t.Fatalf("debug %q: %v", args, err)
		}
	}

	do("item", "katana")
	if len(g.Inventory) != 1 || g.Inventory[0].Def.ID != "katana" || !g.Debugged {
		t.Fatalf("inventory %v, debugged %v", g.Inventory, g.Debugged)
	}
	if !strings.HasSuffix(g.Status(), "  デバッグ") {
		t.Errorf("status %q", g.Status())
	}

	do("monster", "nezumi1")
	if len(g.Monsters) != 1 || g.Monsters[0].Def.ID != "nezumi1" || g.Monsters[0].Spawned {
		t.Errorf("monsters %v", g.Monsters)
	}

	do("identify")
	for _, def := range g.Dungeon.Items {
		if !g.identified[def.ID] {
			t.Errorf("%s not identified", def.ID)
		}
	}

	do("reveal")
	if !g.Lit {
		t.Error("the floor is not lit")
	}

	do("floor", "7")
	if g.Depth != 7 {
		t.Errorf("depth %d", g.Depth)
	}
	if g.Turn != turn {
		t.Errorf("turns %d: the debug mode costs no turn", g.Turn-turn)
	}

	// Invincible: falling leaves the player standing.
	do("invincible")
	g.Monsters = nil
	g.HP, g.Fullness = 1, 0
	wait(t, g, 3)
	if g.Over != "" || g.HP == 0 {
		t.Fatalf("over %q, HP %d", g.Over, g.HP)
	}
	do("invincible")
	if g.Invincible {
		t.Error("still invincible")
	}

	for _, args := range [][]string{{"floor", "0"}, {"floor", "999"}, {"item", "nothing"}, {"monster", "nothing"}, {"fly"}} {
		if err := g.Do("debug", args); err == nil {
			t.Errorf("debug %q was accepted", args)
		}
	}
	if !strings.HasPrefix(g.Summary(), "[デバッグ] ") {
		t.Errorf("summary %q", g.Summary())
	}
}

func TestDebugMenu(t *testing.T) {
	g := debugGame(t)
	if err := g.Do("debug", nil); err != nil {
		t.Fatal(err)
	}
	chooseLabel(t, g, "道具を出す")
	if g.Prompt == nil || !g.Prompt.Input || !slices.Contains(g.Prompt.Complete, "薬草") {
		t.Fatalf("prompt %+v", g.Prompt)
	}
	// A name not known asks again; a name or an ID gives it.
	if err := g.Do("name", []string{"ないもの"}); err != nil || g.Prompt == nil {
		t.Fatalf("asked again: %v %+v", err, g.Prompt)
	}
	if err := g.Do("name", []string{"薬草"}); err != nil {
		t.Fatal(err)
	}
	if len(g.Inventory) != 1 || g.Inventory[0].Def.ID != "yakusou" {
		t.Fatalf("inventory %v", g.Inventory)
	}
	if err := g.Do("debug", nil); err != nil {
		t.Fatal(err)
	}
	chooseLabel(t, g, "階を移動する")
	chooseLabel(t, g, "3F")
	if g.Depth != 3 {
		t.Errorf("depth %d", g.Depth)
	}
	if err := g.Do("debug", nil); err != nil {
		t.Fatal(err)
	}
	chooseLabel(t, g, "敵を出す")
	if err := g.Do("name", []string{"nezumi1"}); err != nil {
		t.Fatal(err)
	}
	m := g.Monsters[slices.IndexFunc(g.Monsters, func(m *Monster) bool { return m.Def.ID == "nezumi1" && !m.Ally })]

	// Frozen, the monsters do nothing.
	if err := g.Do("debug", []string{"freeze"}); err != nil || !g.Frozen || !strings.Contains(g.Status(), "停止") {
		t.Fatalf("freeze: %v %v", err, g.Frozen)
	}
	pos, n := m.Pos, len(g.Monsters)
	for range 30 {
		g.Do("wait", nil)
	}
	if m.Pos != pos || len(g.Monsters) != n {
		t.Errorf("moved %v -> %v, monsters %d -> %d", pos, m.Pos, n, len(g.Monsters))
	}
	g.Do("debug", []string{"freeze"})
	if g.Frozen {
		t.Error("still frozen")
	}
}

// TestDebugReplay replays a run that used the debug mode: the replay
// knows it may, and replays the debug actions.
func TestDebugReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last.replay")
	g := NewGame(cave(), "debug")
	g.DebugMode = true
	rec, err := NewRecorder(path, g)
	if err != nil {
		t.Fatal(err)
	}
	p := &Plugin{game: g, rec: rec}
	for _, a := range [][]string{{"debug", "floor", "5"}, {"debug", "item", "katana"}, {"wait"}, {"debug"}, {"choose", "6"}} {
		if err := p.do(a[0], a[1:]); err != nil {
			t.Fatal(err)
		}
	}
	rec.Close()
	want := state(g)
	_, h, err := replayGame(path)
	if err != nil {
		t.Fatal(err)
	}
	if !h.DebugMode || !h.Debugged || h.Gold != 1000 {
		t.Errorf("debug mode %v, debugged %v, gold %d", h.DebugMode, h.Debugged, h.Gold)
	}
	if got := state(h); got != want {
		t.Errorf("replayed:\n%s\nplayed:\n%s", got, want)
	}
}
