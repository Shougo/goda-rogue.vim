package rogue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stateText is the text of the state file name of testdata/states.
func stateText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "states", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestLoadState: a state file sets the floor, the player, its items, the
// monsters and the traps.
func TestLoadState(t *testing.T) {
	g := NewGame(cave(), "state")
	g.DebugMode = true
	if err := g.Do("debug", []string{"state", stateText(t, "water.txt")}); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 12 || g.Player != (Point{2, 2}) || g.Stairs != (Point{10, 3}) {
		t.Fatalf("depth %d, player %v, stairs %v", g.Depth, g.Player, g.Stairs)
	}
	if g.Level.At(Point{5, 2}) != TileWater || g.Level.At(Point{12, 3}) != TileCorridor || len(g.Level.Rooms) != 1 {
		t.Errorf("terrain: rooms %v", g.Level.Rooms)
	}
	if g.PlayerLevel != 5 || g.HP != 40 || g.MaxHP != 50 || g.Strength != 9 || g.Fullness != 80 || g.Gold != 300 {
		t.Errorf("player: level %d, HP %d/%d, strength %d, fullness %d, gold %d", g.PlayerLevel, g.HP, g.MaxHP, g.Strength, g.Fullness, g.Gold)
	}
	sword := g.equipped(CategoryWeapon)
	if sword == nil || sword.Upgrade != 3 || strings.Join(sword.Seals, ",") != "会,錆" || len(g.Inventory) != 2 {
		t.Fatalf("inventory %v", g.Inventory)
	}
	if len(g.Items) != 1 || !g.Items[0].Cursed || len(g.Monsters) != 1 || !g.Monsters[0].Dozing || g.Monsters[0].HP != 5 {
		t.Errorf("items %v, monsters %v", g.Items, g.Monsters)
	}
	if len(g.Traps) != 1 || !g.Traps[0].Found || !g.identified["otogiri"] || g.identified["shikibetsu"] {
		t.Errorf("traps %v", g.Traps)
	}
	if !g.Debugged {
		t.Error("not marked")
	}
}

// TestDumpState: a scene written and read again is the same.
func TestDumpState(t *testing.T) {
	g := NewGame(cave(), "dump")
	g.DebugMode = true
	for range 40 {
		g.Do("move", []string{"right"})
	}
	text := g.DumpState()
	h := NewGame(cave(), "other")
	h.DebugMode = true
	if err := h.Do("debug", []string{"state", text}); err != nil {
		t.Fatal(err)
	}
	if again := h.DumpState(); strings.SplitN(again, "\n", 2)[1] != strings.SplitN(text, "\n", 2)[1] {
		t.Errorf("dumped again:\n%s\nfirst:\n%s", again, text)
	}
}

// TestStateErrors: a wrong state file says where.
func TestStateErrors(t *testing.T) {
	g := NewGame(cave(), "state")
	g.DebugMode = true
	for _, text := range []string{"ダンジョン\tdeep", "階\t999", "持ち物\tnothing", "敵\t1,1 nothing", "主人公\t0,0\n地形\n###\n#.#\n###", "地形\n#X#", "持ち物\tyakusou 祝福 呪い"} {
		if err := g.Do("debug", []string{"state", text}); err == nil {
			t.Errorf("%q was accepted", text)
		}
	}
}

// TestReplayState: a replay keeps the text of a state file (tabs and
// line breaks quoted), and replays it.
func TestReplayState(t *testing.T) {
	for _, s := range []string{"plain", "a\tb\nc", `"quoted`} {
		if got, err := unquoteArg(quoteArg(s)); err != nil || got != s {
			t.Errorf("%q -> %q (%v)", s, got, err)
		}
	}
	if quoteArg("plain") != "plain" {
		t.Error("plain quoted")
	}
	// Written and read again, then replayed.
	g := NewGame(cave(), "replay")
	g.DebugMode = true
	path := filepath.Join(t.TempDir(), "state.replay")
	rec, err := NewRecorder(path, g)
	if err != nil {
		t.Fatal(err)
	}
	text := stateText(t, "water.txt")
	if err := g.Do("debug", []string{"state", text}); err != nil {
		t.Fatal(err)
	}
	if err := rec.Action(0, "debug", []string{"state", text}, g.takeEvents()); err != nil {
		t.Fatal(err)
	}
	rec.Close()
	rp, err := ReadReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rp.Actions) != 1 || rp.Actions[0].Args[1] != text {
		t.Fatalf("actions %+v", rp.Actions)
	}
	h, err := rp.Play(cave(), len(rp.Actions))
	if err != nil {
		t.Fatal(err)
	}
	if h.Depth != 12 || h.Player != (Point{2, 2}) {
		t.Errorf("replayed: depth %d, player %v", h.Depth, h.Player)
	}
}
