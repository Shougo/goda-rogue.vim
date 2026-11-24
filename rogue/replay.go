package rogue

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// A run is all in its replay file: the dungeon (and the hash of its
// data), the seed, and every action of the player, appended as the run
// goes (a crash loses nothing). Replaying the actions makes the same run:
// all the randomness comes from the seed. The same file is the suspended
// run, resumed by replaying it, and the replay to watch or to share.
//
// The file is text:
//
//	# goda-rogue replay
//	format	1
//	dungeon	cave
//	hash	1a2b3c4d
//	seed	12345
//	started	2026-11-18T08:30:00+09:00
//	revision	abcdef0
//	debug	1
//	---
//	# 0 floor 1
//	1520	move	right
//	3075	attack
//	# 12 hurt 3 nezumi1
//	9000	suspend
//
// debug is there when the run may use the debug mode (debug.go).
// An action is its play time in milliseconds, its name and its arguments,
// separated by tabs. "suspend" marks where the run was suspended. The
// lines "# turn event ..." after an action are what happened in it
// (Game.event): ignored by replaying, they are checked against what
// replaying makes, and can be read without replaying (analysis).

// ReplayFormat is the version of the format of the replay files.
const ReplayFormat = 2

// replaySuspend is the line of the action that suspends a run.
const replaySuspend = "suspend"

// Replay is a replay file.
type Replay struct {
	Format  int
	Dungeon string
	Hash    string
	Seed    string
	// Started is when the run began; Revision is the revision of the
	// binary that played it, if known: both only for information.
	Started  time.Time
	Revision string
	// Debug says whether the run could use the debug mode.
	Debug bool
	// Events are those before the first action (the first floor).
	Events  []string
	Actions []ReplayAction
}

// ReplayAction is an action of a replay.
type ReplayAction struct {
	// Time is the play time when it was done.
	Time   time.Duration
	Action string
	Args   []string
	// Events are what happened in it.
	Events []string
	// Line is its line in the file.
	Line int
}

// recorded says whether the action goes in the replay: those that change
// the game (looking and choosing too), not those that only show it.
func recorded(action string) bool {
	switch action {
	case "redraw", "log", "map", "items", "catalog":
		return false
	}
	return true
}

// revision is the VCS revision of the binary, if it was built with one.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}

// ReadReplay reads the replay file path.
func ReadReplay(path string) (*Replay, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rp, err := ParseReplay(f)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	return rp, nil
}

// ParseReplay reads a replay. Its errors begin with the line number.
func ParseReplay(r io.Reader) (*Replay, error) {
	rp := &Replay{}
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 1<<20)
	n := 0
	header := true
	for sc.Scan() {
		n++
		line := sc.Text()
		if header {
			switch {
			case line == "---":
				header = false
			case line == "" || strings.HasPrefix(line, "#"):
			default:
				key, value, _ := strings.Cut(line, "\t")
				switch key {
				case "format":
					f, err := strconv.Atoi(value)
					if err != nil {
						return nil, fmt.Errorf("%d: format: %w", n, err)
					}
					rp.Format = f
				case "dungeon":
					rp.Dungeon = value
				case "hash":
					rp.Hash = value
				case "seed":
					rp.Seed = value
				case "started":
					// Only for information: a bad time is no error.
					rp.Started, _ = time.Parse(time.RFC3339, value)
				case "revision":
					rp.Revision = value
				case "debug":
					rp.Debug = value == "1"
				}
			}
			continue
		}
		if e, ok := strings.CutPrefix(line, "# "); ok {
			if len(rp.Actions) == 0 {
				rp.Events = append(rp.Events, e)
			} else {
				a := &rp.Actions[len(rp.Actions)-1]
				a.Events = append(a.Events, e)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("%d: an action expected: %q", n, line)
		}
		ms, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%d: the time of an action: %w", n, err)
		}
		rp.Actions = append(rp.Actions, ReplayAction{
			Time: time.Duration(ms) * time.Millisecond, Action: fields[1], Args: fields[2:], Line: n,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%d: %w", n, err)
	}
	if header {
		return nil, fmt.Errorf("%d: no \"---\" after the header", n)
	}
	if rp.Format != ReplayFormat {
		return nil, fmt.Errorf("1: format %d, want %d", rp.Format, ReplayFormat)
	}
	if rp.Dungeon == "" || rp.Seed == "" {
		return nil, fmt.Errorf("1: no dungeon or seed in the header")
	}
	return rp, nil
}

// Suspended says whether the run of the replay was suspended (or cut
// short), not over: whether it can be resumed. Only replaying it tells
// whether it is over, so it returns the game replayed too.
func (rp *Replay) Suspended(d *Dungeon) (bool, *Game, error) {
	g, err := rp.Play(d, len(rp.Actions))
	if err != nil {
		return false, nil, err
	}
	return g.Over == "", g, nil
}

// ErrReplayMismatch says that a replay does not replay the same with this
// version of the game or of the dungeon.
var ErrReplayMismatch = errors.New("the replay does not replay the same with this version")

// NewReplayGame starts the run of the replay: the game before its first
// action.
func (rp *Replay) NewReplayGame(d *Dungeon) (*Game, error) {
	if d.ID != rp.Dungeon {
		return nil, fmt.Errorf("the replay is of the dungeon %q, not %q", rp.Dungeon, d.ID)
	}
	if d.Hash != rp.Hash {
		return nil, fmt.Errorf("%w: the dungeon %q has changed (hash %s, the replay's %s)",
			ErrReplayMismatch, d.ID, d.Hash, rp.Hash)
	}
	g := NewGame(d, rp.Seed)
	g.DebugMode = rp.Debug
	if err := checkEvents(0, rp.Events, g.takeEvents()); err != nil {
		return nil, err
	}
	return g, nil
}

// Play replays the first n actions of rp in the dungeon d, checking that
// they make the same events, and returns the game after them.
func (rp *Replay) Play(d *Dungeon, n int) (*Game, error) {
	g, err := rp.NewReplayGame(d)
	if err != nil {
		return nil, err
	}
	for _, a := range rp.Actions[:n] {
		if err := g.replayAction(a); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// replayAction does the action a of a replay, and checks its events.
func (g *Game) replayAction(a ReplayAction) error {
	if a.Action == replaySuspend {
		return nil
	}
	if err := g.Do(a.Action, a.Args); err != nil {
		return fmt.Errorf("%w: line %d: %v", ErrReplayMismatch, a.Line, err)
	}
	g.Played, g.Started = a.Time, time.Now()
	if g.Over != "" {
		g.Ended = g.Started
	}
	return checkEvents(a.Line, a.Events, g.takeEvents())
}

// checkEvents says whether the events recorded at the line are those
// replaying made.
func checkEvents(line int, recorded, made []string) error {
	if strings.Join(recorded, "\n") == strings.Join(made, "\n") {
		return nil
	}
	return fmt.Errorf("%w: line %d: events %q, replaying makes %q", ErrReplayMismatch, line, recorded, made)
}

// Recorder writes the replay file of a run as it goes.
type Recorder struct {
	Path string
	f    *os.File
}

// NewRecorder starts the replay file path (made anew) of the game g, just
// started.
func NewRecorder(path string, g *Game) (*Recorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	r := &Recorder{Path: path, f: f}
	var b strings.Builder
	fmt.Fprintf(&b, "# goda-rogue replay\n")
	fmt.Fprintf(&b, "format\t%d\n", ReplayFormat)
	fmt.Fprintf(&b, "dungeon\t%s\n", g.Dungeon.ID)
	fmt.Fprintf(&b, "hash\t%s\n", g.Dungeon.Hash)
	fmt.Fprintf(&b, "seed\t%s\n", g.Seed)
	fmt.Fprintf(&b, "started\t%s\n", g.Started.Format(time.RFC3339))
	if rev := revision(); rev != "" {
		fmt.Fprintf(&b, "revision\t%s\n", rev)
	}
	if g.DebugMode {
		b.WriteString("debug\t1\n")
	}
	b.WriteString("---\n")
	writeEvents(&b, g.takeEvents())
	if err := r.write(b.String()); err != nil {
		f.Close()
		return nil, err
	}
	return r, nil
}

// AppendRecorder goes on writing the replay file path (a resumed run).
func AppendRecorder(path string) (*Recorder, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return nil, err
	}
	return &Recorder{Path: path, f: f}, nil
}

// Action writes the action done at the play time t, and its events.
func (r *Recorder) Action(t time.Duration, action string, args []string, events []string) error {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(t.Milliseconds(), 10))
	for _, s := range append([]string{action}, args...) {
		b.WriteString("\t" + s)
	}
	b.WriteString("\n")
	writeEvents(&b, events)
	return r.write(b.String())
}

// Close closes the file.
func (r *Recorder) Close() error { return r.f.Close() }

func (r *Recorder) write(s string) error {
	_, err := r.f.WriteString(s)
	return err
}

func writeEvents(b *strings.Builder, events []string) {
	for _, e := range events {
		b.WriteString("# " + e + "\n")
	}
}
