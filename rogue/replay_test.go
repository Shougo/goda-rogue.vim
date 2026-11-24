package rogue

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// randomAction returns an action a player could do now, at random.
func randomAction(g *Game, rng *rand.Rand) (string, []string) {
	if p := g.Prompt; p != nil {
		if rng.IntN(8) == 0 {
			return "cancel", nil
		}
		if p.Input {
			return "name", []string{[]string{"", "混乱?"}[rng.IntN(2)]}
		}
		return "choose", []string{fmt.Sprint(rng.IntN(len(p.Choices)))}
	}
	dirs := slices.Sorted(func(yield func(string) bool) {
		for d := range Directions {
			if !yield(d) {
				return
			}
		}
	})
	dir := []string{dirs[rng.IntN(len(dirs))]}
	switch n := rng.IntN(40); {
	case n < 20:
		return "move", dir
	case n < 23:
		return "dash", dir
	case n < 27:
		return "attack", nil
	case n < 29:
		return "inventory", nil
	case n < 30:
		return "turn", nil
	case n < 31:
		return "turn", dir
	case n < 32:
		return "look", nil
	case n < 33:
		return "shoot", nil
	case n < 34:
		return "pickup", nil
	case n < 35:
		return "map", nil
	case n < 36:
		return "redraw", nil
	case n < 38:
		return "stairs", nil
	}
	return "wait", nil
}

// recordedGame starts a game of the seed whose replay is written to a file
// in dir, as the plugin does.
func recordedGame(t *testing.T, dir, seed string) (*Plugin, string) {
	t.Helper()
	path := filepath.Join(dir, "last.replay")
	g := NewGame(cave(), seed)
	rec, err := NewRecorder(path, g)
	if err != nil {
		t.Fatal(err)
	}
	p := &Plugin{game: g, rec: rec}
	// Closed at the end in any case: MS-Windows cannot remove the
	// temporary directory with a file open in it. (Closing again only
	// errs.)
	t.Cleanup(func() {
		if p.rec != nil {
			p.rec.Close()
		}
	})
	return p, path
}

// play does n random actions (fewer if the run ends), and returns them.
func play(t *testing.T, p *Plugin, rng *rand.Rand, n int) [][]string {
	t.Helper()
	var done [][]string
	for range n {
		if p.game.Over != "" {
			break
		}
		action, args := randomAction(p.game, rng)
		if err := p.do(action, args); err != nil {
			t.Fatalf("%s %q: %v", action, args, err)
		}
		done = append(done, append([]string{action}, args...))
		if p.game.Stairs == p.game.Player && rng.IntN(2) == 0 {
			if err := p.do("stairs", nil); err != nil {
				t.Fatal(err)
			}
			done = append(done, []string{"stairs"})
		}
	}
	return done
}

// timeless is the status line without the play time, which differs from
// a run to its replay.
var timeless = regexp.MustCompile(`時間 \S+`)

// state is what a replay must make the same.
func state(g *Game) string {
	var b strings.Builder
	fmt.Fprintln(&b, timeless.ReplaceAllString(g.Status(), ""))
	fmt.Fprintln(&b, g.Depth, g.Turn, g.HP, g.MaxHP, g.Player, g.Facing, g.Gold, g.Over, g.Kills, g.EXP)
	for _, it := range g.Inventory {
		fmt.Fprintln(&b, g.Entry(it))
	}
	for _, m := range g.Monsters {
		fmt.Fprintln(&b, m.Def.ID, m.Pos, m.HP)
	}
	for _, it := range g.Items {
		fmt.Fprintln(&b, it.Def.ID, it.Pos)
	}
	b.WriteString(strings.Join(g.Log, "\n"))
	return b.String()
}

func TestReplay(t *testing.T) {
	for i := range 20 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			dir := t.TempDir()
			p, path := recordedGame(t, dir, fmt.Sprint("replay", i))
			play(t, p, rand.New(rand.NewPCG(uint64(i), 1)), 1500)
			want := state(p.game)
			p.rec.Close()

			_, g, err := replayGame(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := state(g); got != want {
				t.Errorf("replayed:\n%s\nplayed:\n%s", got, want)
			}
		})
	}
}

func TestReplayFile(t *testing.T) {
	dir := t.TempDir()
	p, path := recordedGame(t, dir, "file")
	for _, a := range [][]string{{"wait"}, {"redraw"}, {"map"}, {"move", "left"}, {"inventory"}} {
		if err := p.do(a[0], a[1:]); err != nil {
			t.Fatal(err)
		}
	}
	p.rec.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	d := p.game.Dungeon
	head := []string{
		"# goda-rogue replay", fmt.Sprintf("format\t%d", ReplayFormat), "dungeon\tcave", "hash\t" + d.Hash, "seed\tfile",
	}
	if !slices.Equal(lines[:len(head)], head) {
		t.Errorf("header %q", lines[:len(head)])
	}
	i := slices.Index(lines, "---")
	if i < 0 || lines[i+1] != "# 0 floor 1" {
		t.Fatalf("lines %q", lines)
	}
	// Only what changes the game: not redraw nor map.
	var actions []string
	for _, l := range lines[i+1:] {
		if l != "" && !strings.HasPrefix(l, "#") {
			_, a, _ := strings.Cut(l, "\t")
			actions = append(actions, a)
		}
	}
	if want := []string{"wait", "move\tleft", "inventory"}; !slices.Equal(actions, want) {
		t.Errorf("actions %q, want %q", actions, want)
	}
}

// TestSuspend suspends a run, resumes it, and checks that it goes on as
// if it had not been suspended.
func TestSuspend(t *testing.T) {
	// A run that lasts long enough.
	var p *Plugin
	var path, seed string
	var rng *rand.Rand
	var actions [][]string
	for i := range 50 {
		seed = fmt.Sprint("suspend", i)
		p, path = recordedGame(t, t.TempDir(), seed)
		rng = rand.New(rand.NewPCG(uint64(i), 7))
		actions = play(t, p, rng, 100)
		if p.game.Over == "" {
			break
		}
		p.rec.Close()
	}
	if p.game.Over != "" {
		t.Fatal("every run over before suspending")
	}
	if err := p.suspend(); err != nil {
		t.Fatal(err)
	}
	if p.game != nil || p.rec != nil {
		t.Fatal("still playing after suspending")
	}
	last := lastRun(path)
	if last.State != "suspended" || last.Dungeon != "cave" || last.Depth == 0 {
		t.Fatalf("last run %+v", last)
	}

	// Resumed: the same game, and what follows goes on in the file.
	_, g, err := replayGame(path)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := AppendRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	p.game, p.rec = g, rec
	more := play(t, p, rng, 300)
	want := state(p.game)
	p.rec.Close()

	// A run without the suspension does the same.
	q, _ := recordedGame(t, t.TempDir(), seed)
	for _, a := range append(actions, more...) {
		if err := q.do(a[0], a[1:]); err != nil {
			t.Fatal(err)
		}
	}
	if got := state(q.game); got != want {
		t.Errorf("without suspending:\n%s\nresumed:\n%s", got, want)
	}
	// And the file replays it.
	_, g, err = replayGame(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := state(g); got != want {
		t.Errorf("replayed:\n%s\nresumed:\n%s", got, want)
	}
}

func TestLastRun(t *testing.T) {
	dir := t.TempDir()
	if got := lastRun(filepath.Join(dir, "none")); got.State != "none" {
		t.Errorf("no file: %+v", got)
	}

	// Over (waiting for Kron's wind, or falling before): it cannot be
	// resumed.
	p, path := recordedGame(t, dir, "over")
	for p.game.Over == "" {
		if err := p.do("wait", nil); err != nil {
			t.Fatal(err)
		}
	}
	p.rec.Close()
	if got := lastRun(path); got.State != "over" || got.Turn != p.game.Turn {
		t.Errorf("over: %+v", got)
	}

	// Changed by hand: what is not in the replay does not replay.
	q, path := recordedGame(t, t.TempDir(), "hand")
	q.game.Player = q.game.Stairs
	if err := q.do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	q.rec.Close()
	if got := lastRun(path); got.State != "broken" || !strings.Contains(got.Error, "does not replay the same") {
		t.Errorf("changed by hand: %+v", got)
	}
}

// TestLastRunTurnZero: a run left at turn 0 (only actions that cost no
// turn) can be resumed, and the Vim script gets its turn, 0.
func TestLastRunTurnZero(t *testing.T) {
	p, path := recordedGame(t, t.TempDir(), "zero")
	if err := p.do("turn", []string{"left"}); err != nil {
		t.Fatal(err)
	}
	p.rec.Close()
	got := lastRun(path)
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "suspended" || !strings.Contains(string(b), `"turn":0`) {
		t.Errorf("%s", b)
	}
}

func TestReplayMismatch(t *testing.T) {
	dir := t.TempDir()
	p, path := recordedGame(t, dir, "mismatch")
	play(t, p, rand.New(rand.NewPCG(3, 3)), 200)
	p.rec.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// An event changed: this version would not replay it the same.
	bad := strings.Replace(string(b), "# 0 floor 1", "# 0 floor 2", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := replayGame(path); !errors.Is(err, ErrReplayMismatch) {
		t.Errorf("changed event: %v", err)
	}

	// Another dungeon data.
	bad = regexp.MustCompile(`(?m)^hash\t.*$`).ReplaceAllString(string(b), "hash\t00000000")
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := replayGame(path); !errors.Is(err, ErrReplayMismatch) || !strings.Contains(err.Error(), "has changed") {
		t.Errorf("changed hash: %v", err)
	}

	// Not a replay.
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastRun(path); got.State != "broken" {
		t.Errorf("not a replay: %+v", got)
	}
}

func TestWatch(t *testing.T) {
	dir := t.TempDir()
	p, path := recordedGame(t, dir, "watch")
	play(t, p, rand.New(rand.NewPCG(5, 5)), 400)
	want := state(p.game)
	p.rec.Close()

	rp, _, err := replayGame(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := rp.NewReplayGame(p.game.Dungeon)
	if err != nil {
		t.Fatal(err)
	}
	w := &Plugin{game: g, watching: &watch{replay: rp}}
	// Halfway, however long the random run lasted.
	half := len(rp.Actions) / 2
	if half == 0 {
		t.Fatalf("a replay of %d actions", len(rp.Actions))
	}
	done, err := w.step(half)
	if err != nil || done || w.watching.next != half {
		t.Fatalf("step %d: %v, done %v, next %d", half, err, done, w.watching.next)
	}
	s := w.watchScreen(Size{80, 30})
	if last := s.Lines[len(s.Lines)-1]; last != fmt.Sprintf("リプレイ  %d/%d", half, len(rp.Actions)) {
		t.Errorf("progress %q", last)
	}
	if done, err := w.step(-1); err != nil || !done {
		t.Fatalf("step to the end: %v, done %v", err, done)
	}
	if got := state(w.game); got != want {
		t.Errorf("watched:\n%s\nplayed:\n%s", got, want)
	}
}

// TestSamePath: two names of one file are the same path (the short and
// the long names of Windows, here a link); a missing file is only the
// same string.
func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last.replay")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// (Not filepath.Join: it cleans the "." away.)
	sep := string(filepath.Separator)
	other := dir + sep + "." + sep + "last.replay"
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("no symbolic link:", err)
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{path, path, true},
		{path, other, true},
		{path, filepath.Join(link, "last.replay"), true},
		{path, filepath.Join(dir, "other.replay"), false},
		{filepath.Join(dir, "x"), dir + sep + "." + sep + "x", false},
	} {
		if got := samePath(tc.a, tc.b); got != tc.want {
			t.Errorf("samePath(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
