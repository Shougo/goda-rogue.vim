package rogue

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCompareBot: the run of a bot compared with the same bot has no
// difference; compared with the careful bot, the reckless one went down
// where the other would have explored on, and the report says so.
func TestCompareBot(t *testing.T) {
	d := cave()
	// The floors change with the data: the first seed whose reckless
	// bot went down where the careful one would not have.
	var path, seed string
	var rp *Replay
	for i := 0; ; i++ {
		if i == 10 {
			t.Fatal("no seed where the bots differ")
		}
		seed = fmt.Sprintf("compare%d", i)
		path = filepath.Join(t.TempDir(), "run.replay")
		rp = botRun(t, d, seed, path)
		diffs, _, err := CompareBot(rp, d, BotCareful)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(diffs, func(df Difference) bool { return df.Player.Kind == "階段を降りる" }) {
			break
		}
	}

	diffs, n, err := CompareBot(rp, d, BotReckless)
	if err != nil || n < 50 || len(diffs) != 0 {
		t.Fatalf("with itself: %d differences in %d decisions (%v): %+v", len(diffs), n, err, diffs)
	}
	diffs, m, err := CompareBot(rp, d, BotCareful)
	if err != nil || m != n {
		t.Fatalf("with the careful bot: %d decisions, want %d (%v)", m, n, err)
	}
	if !slices.ContainsFunc(diffs, func(df Difference) bool {
		return df.Player.Kind == "階段を降りる" && df.Bot.Kind == "移動" && df.Depth >= 1 && df.MaxHP > 0
	}) {
		t.Errorf("no descent the careful bot would not have made: %+v", diffs)
	}
	lines, err := CompareReport(path, BotCareful)
	if err != nil || len(lines) < 6 ||
		lines[0] != "リプレイと Bot（careful）の比較  "+d.Name+"  シード "+seed ||
		!strings.HasPrefix(lines[1], "判断 ") || !slices.ContainsFunc(lines, func(l string) bool {
		return strings.Contains(l, "階段を降りる → 移動［")
	}) {
		t.Errorf("report (%v):\n%s", err, strings.Join(lines, "\n"))
	}

	// A replay of another version of the dungeon is refused.
	rp.Hash = "0"
	if _, _, err := CompareBot(rp, d, BotCareful); err == nil {
		t.Error("a replay of another dungeon is compared")
	}
}

// botRun plays the reckless bot down to 6F of d with seed, recorded in
// path, and returns the replay.
func botRun(t *testing.T, d *Dungeon, seed, path string) *Replay {
	t.Helper()
	g := NewGame(d, seed)
	rec, err := NewRecorder(path, g)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBot(BotReckless, 0)
	for n := 0; n < 20000 && g.Over == "" && g.Depth < 6; n++ {
		a, args := b.Next(g)
		if err := g.Do(a, args); err != nil {
			t.Fatal(err)
		}
		if events := g.takeEvents(); recorded(a) {
			if err := rec.Action(g.PlayTime(), a, args, events); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}
	rp, err := ReadReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	return rp
}
