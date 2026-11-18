package rogue

import (
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// kept returns the names of the replays kept beside the replay file in
// dir.
func kept(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(keptDir)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestKeepReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last.replay")
	if err := keepReplay(path, 50); err != nil {
		t.Fatal(err)
	}

	// A run without an action: not kept.
	p, _ := recordedGame(t, dir, "empty")
	p.rec.Close()
	if err := keepReplay(path, 50); err != nil || len(kept(t, dir)) != 0 {
		t.Errorf("%v, kept %q", err, kept(t, dir))
	}

	// A run, kept as when it began; twice in a second, both.
	for range 2 {
		p, _ := recordedGame(t, dir, "run")
		play(t, p, rand.New(rand.NewPCG(1, 1)), 20)
		p.rec.Close()
		if err := keepReplay(path, 50); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("still there: %v", err)
		}
	}
	names := kept(t, dir)
	re := regexp.MustCompile(`^\d{8}-\d{6}-cave(-2)?\.replay$`)
	if len(names) != 2 || !re.MatchString(names[0]) || !re.MatchString(names[1]) {
		t.Errorf("kept %q", names)
	}
	rp, err := ReadReplay(filepath.Join(dir, filepath.FromSlash(keptDir), names[0]))
	if err != nil || rp.Seed != "run" {
		t.Errorf("%v, %+v", err, rp)
	}

	// A broken file: kept, it may be what went wrong.
	if err := os.WriteFile(path, []byte("not a replay"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := keepReplay(path, 50); err != nil || len(kept(t, dir)) != 3 {
		t.Errorf("%v, kept %q", err, kept(t, dir))
	}

	// keep 0: none.
	p, _ = recordedGame(t, dir, "none")
	play(t, p, rand.New(rand.NewPCG(1, 1)), 5)
	p.rec.Close()
	if err := keepReplay(path, 0); err != nil || len(kept(t, dir)) != 3 {
		t.Errorf("%v, kept %q", err, kept(t, dir))
	}
	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
}

func TestPruneReplays(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"20261118-1.replay", "20261116-1.replay", "20261117-1.replay", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneReplays(dir, 2); err != nil {
		t.Fatal(err)
	}
	names, _ := os.ReadDir(dir)
	var got []string
	for _, e := range names {
		got = append(got, e.Name())
	}
	if want := []string{"20261117-1.replay", "20261118-1.replay", "notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestStartKeeps: a new run keeps the replay of the one before, here one
// that ended with a bug (a panic recovered).
func TestStartKeeps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last.replay")
	p := New()
	// The run started last keeps its replay file open: MS-Windows cannot
	// remove the temporary directory with it.
	t.Cleanup(p.stop)
	start := p.Handlers()["start"]
	opts := map[string]any{"dungeon": "cave", "seed": "keep", "replay": path, "keep": 5}
	size := map[string]any{"width": 80, "height": 24}
	if _, err := start(context.Background(), nil, []any{opts, size}); err != nil {
		t.Fatal(err)
	}
	if err := p.do("wait", nil); err != nil {
		t.Fatal(err)
	}
	p.game.Monsters = append(p.game.Monsters, nil) // a bug
	if err := p.do("wait", nil); err == nil {
		t.Fatal("no bug")
	}
	if _, err := start(context.Background(), nil, []any{opts, size}); err != nil {
		t.Fatal(err)
	}
	if names := kept(t, dir); len(names) != 1 {
		t.Errorf("kept %q", names)
	}
}
