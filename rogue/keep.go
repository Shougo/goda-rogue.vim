package rogue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// keptDir is the directory, beside the replay file of the last run, of
// the replays of the runs before it (keepReplay).
const keptDir = "replays/auto"

// keepReplay keeps the replay file path of the last run before a new run
// replaces it: it moves it to keptDir, as "date-time-dungeon.replay" (when
// the run began), and removes the oldest there but the keep newest. So
// every run is kept, however it ended (over, suspended, a bug, the editor
// killed), for a while. A run without an action is not kept; keep 0 keeps
// none.
func keepReplay(path string, keep int) error {
	return keepReplayIn(path, filepath.Join(filepath.Dir(path), filepath.FromSlash(keptDir)), keep)
}

// keptBots is where the replays of the runs of the bots are kept, under
// the directory of the bot's replay file.
const keptBots = "bot"

// keepReplayIn is keepReplay, into the directory dir.
func keepReplayIn(path, dir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	rp, err := ReadReplay(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err == nil && len(rp.Actions) == 0:
		return nil
	}
	// A broken file is kept too: it may be what went wrong.
	name := "broken"
	if err == nil {
		name = rp.Started.Format("20060102-150405") + "-" + rp.Dungeon
	} else if fi, err := os.Stat(path); err == nil {
		name = fi.ModTime().Format("20060102-150405") + "-" + name
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dir, name+".replay")
	for n := 2; ; n++ {
		if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			break
		}
		dst = filepath.Join(dir, fmt.Sprintf("%s-%d.replay", name, n))
	}
	if err := os.Rename(path, dst); err != nil {
		return err
	}
	return pruneReplays(dir, keep)
}

// pruneReplays removes the oldest replays of dir (by their names) but the
// keep newest.
func pruneReplays(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".replay") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	var errs []error
	for _, name := range names[:max(len(names)-keep, 0)] {
		errs = append(errs, os.Remove(filepath.Join(dir, name)))
	}
	return errors.Join(errs...)
}
