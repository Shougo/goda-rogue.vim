package rogue

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// update writes the golden screens anew: go test ./rogue -run Golden
// -update.
var update = flag.Bool("update", false, "write the golden files of the tests anew")

// frozen returns the dungeon of testdata/dungeons/frozen: a copy of the
// built-in one that stays as it is, so that tuning the built-in one does
// not change the golden screens.
func frozen(t *testing.T) *Dungeon {
	t.Helper()
	d, err := LoadDungeon(os.DirFS(filepath.Join("testdata", "dungeons", "frozen")), "frozen")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestGolden checks the screens of runs of the frozen dungeon against
// those in testdata/golden: a change of them shows as a diff.
func TestGolden(t *testing.T) {
	for _, c := range []struct {
		name string
		size Size
		// play is how many random actions are done; deep goes down that
		// many floors first.
		play, deep int
		// minimap shows the mini-map, items the items carried; end
		// ends the run, if the player has not fallen (it can, but for
		// invincible).
		minimap, items, end, invincible bool
	}{
		{name: "start", size: Size{80, 24}},
		{name: "small", size: Size{40, 12}},
		{name: "play", size: Size{80, 24}, play: 300, minimap: true, items: true, invincible: true},
		{name: "deep", size: Size{100, 30}, play: 200, deep: 9, invincible: true},
		{name: "result", size: Size{80, 40}, play: 300, end: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := NewGame(frozen(t), "golden")
			g.DebugMode, g.Invincible = true, c.invincible
			for range c.deep {
				if err := g.Do("debug", []string{debugFloor, fmt.Sprint(g.Depth + 1)}); err != nil {
					t.Fatal(err)
				}
			}
			p := &player{g: g, rng: rand.New(rand.NewPCG(1, 2)), stairs: 3}
			for range c.play {
				if g.Over != "" {
					break
				}
				action, args := p.next()
				if err := g.Do(action, args); err != nil {
					t.Fatal(err)
				}
			}
			g.Prompt = nil
			g.ShowMap, g.ShowItems = c.minimap, c.items
			if c.end && g.Over == "" {
				g.end("テストを終えた", false)
			}
			// The play time, which the result shows.
			start := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			g.Played, g.Started, g.Ended = 83*time.Second, start, start
			golden(t, filepath.Join("testdata", "golden", c.name+".txt"), screenText(g.Render(c.size)))
		})
	}
}

// screenText is the screen as text: its lines, then where the cursor is,
// the highlights, the mini-map and the items. The lines end with "|", for the spaces
// at their ends.
func screenText(s Screen) string {
	var b strings.Builder
	for _, l := range s.Lines {
		b.WriteString(l + "|\n")
	}
	fmt.Fprintf(&b, "--- cursor %d %d\n", s.Cursor[0], s.Cursor[1])
	highlights(&b, s.Highlights)
	if m := s.Minimap; m != nil {
		b.WriteString("--- minimap\n")
		for _, l := range m.Lines {
			b.WriteString(l + "|\n")
		}
		highlights(&b, m.Highlights)
	}
	if l := s.Items; l != nil {
		fmt.Fprintf(&b, "--- items %s\n", l.Title)
		for _, line := range l.Lines {
			b.WriteString(line + "|\n")
		}
	}
	return b.String()
}

// highlights writes the spans of each group, the groups sorted.
func highlights(b *strings.Builder, h map[string][]Span) {
	groups := make([]string, 0, len(h))
	for g := range h {
		groups = append(groups, g)
	}
	slices.Sort(groups)
	for _, g := range groups {
		fmt.Fprintf(b, "--- %s\n", g)
		for _, s := range h[g] {
			fmt.Fprintf(b, "%d %d-%d\n", s.Line, s.Col, s.End)
		}
	}
}

// golden checks got against the file path, or writes it with -update.
func golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./rogue -run Golden -update writes it)", err)
	}
	if got == string(want) {
		return
	}
	// The first line that differs.
	g, w := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := range max(len(g), len(w)) {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			t.Fatalf("%s:%d differs (go test ./rogue -run Golden -update to accept it):\n got %q\nwant %q", path, i+1, gl, wl)
		}
	}
}
