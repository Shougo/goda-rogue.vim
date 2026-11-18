// Package rogue is goda-rogue.vim: a mystery-dungeon roguelike in Vim and
// Neovim, whose map is drawn with full-width characters (enemies are
// kanji, their rank is their color).
//
// The Go side owns the whole game; the Vim script sends the actions and
// shows the screens Render returns.
package rogue

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Shougo/goda.vim/sdk"
)

// PluginName is the name of the plugin: goda#request('rogue', ...).
const PluginName = "rogue"

// ScriptVersion is the version of the Vim script (autoload/goda_rogue.vim)
// this binary goes with (sdk.ScriptVersioner). Bump it with s:version there
// when they no longer work together.
const ScriptVersion = 2

// Plugin serves the game.
type Plugin struct {
	mu   sync.Mutex
	game *Game
	// rec writes the replay file of the game, if it has one.
	rec *Recorder
	// watching is the replay being watched, if any (then game is its run).
	watching *watch
}

// watch is a replay being watched: its run is after next actions.
type watch struct {
	replay *Replay
	next   int
}

// New returns the plugin, with no game yet.
func New() *Plugin { return &Plugin{} }

// Name implements sdk.Plugin.
func (*Plugin) Name() string { return PluginName }

// ScriptVersion implements sdk.ScriptVersioner.
func (*Plugin) ScriptVersion() int { return ScriptVersion }

// DefaultDungeon is the dungeon of goda_rogue#start() without one.
const DefaultDungeon = "cave"

// StartOptions are the options of a new game.
type StartOptions struct {
	// Dungeon is the ID of the dungeon. Empty for the built-in one
	// (DefaultDungeon).
	Dungeon string `json:"dungeon"`
	// Seed makes the floors: the same seed, the same floors. Empty for a
	// random one.
	Seed string `json:"seed"`
	// Replay is the path of the replay file to write the run to (made
	// anew); empty for none.
	Replay string `json:"replay"`
	// Debug enables the debug mode (debug.go).
	Debug bool `json:"debug"`
}

// LastRun says what the replay file of the last run holds.
type LastRun struct {
	// State is "none" (no file), "playing" (the run being played),
	// "suspended" (it can be resumed: suspended, or left without
	// suspending), "over" or "broken" (it cannot be resumed: Error says
	// why).
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
	Dungeon string `json:"dungeon,omitempty"`
	Name    string `json:"name,omitempty"`
	Depth   int    `json:"depth,omitempty"`
	Turn    int    `json:"turn,omitempty"`
	Time    string `json:"time,omitempty"`
}

// lastRun reads the replay file path.
func lastRun(path string) LastRun {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return LastRun{State: "none"}
	}
	rp, err := ReadReplay(path)
	if err != nil {
		return LastRun{State: "broken", Error: err.Error()}
	}
	d, err := LoadBuiltin(rp.Dungeon)
	if err != nil {
		return LastRun{State: "broken", Error: fmt.Sprintf("dungeon %q: %v", rp.Dungeon, err)}
	}
	suspended, g, err := rp.Suspended(d)
	if err != nil {
		return LastRun{State: "broken", Error: err.Error(), Dungeon: d.ID, Name: d.Name}
	}
	r := LastRun{State: "over", Dungeon: d.ID, Name: d.Name, Depth: g.Depth, Turn: g.Turn, Time: playTime(g.PlayTime())}
	if suspended {
		r.State = "suspended"
	}
	return r
}

// replayGame replays the replay file path to its end, and returns it and
// its game.
func replayGame(path string) (*Replay, *Game, error) {
	rp, err := ReadReplay(path)
	if err != nil {
		return nil, nil, err
	}
	d, err := LoadBuiltin(rp.Dungeon)
	if err != nil {
		return nil, nil, fmt.Errorf("dungeon %q: %w", rp.Dungeon, err)
	}
	g, err := rp.Play(d, len(rp.Actions))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return rp, g, nil
}

// stop ends the game being played or watched.
func (p *Plugin) stop() {
	if p.rec != nil {
		p.rec.Close()
		p.rec = nil
	}
	p.game, p.watching = nil, nil
}

// do runs an action of the game, and writes it to the replay file.
func (p *Plugin) do(action string, args []string) (err error) {
	g := p.game
	defer func() {
		// A bug: keep the replay, which reproduces it.
		if r := recover(); r != nil {
			err = fmt.Errorf("a bug of goda-rogue: %v", r)
			if p.rec != nil {
				err = fmt.Errorf("%w (its replay: %s)", err, p.rec.Path)
			}
			p.stop()
		}
	}()
	over := g.Over != ""
	if err := g.Do(action, args); err != nil {
		return err
	}
	events := g.takeEvents()
	if p.rec == nil || over || !recorded(action) {
		return nil
	}
	if err := p.rec.Action(g.PlayTime(), action, args, events); err != nil {
		return fmt.Errorf("replay %s: %w", p.rec.Path, err)
	}
	if g.Over != "" {
		// The run has just ended: its record.
		path := filepath.Join(filepath.Dir(p.rec.Path), recordsFile)
		if err := appendRecord(path, recordOf(g)); err != nil {
			return fmt.Errorf("records %s: %w", path, err)
		}
	}
	return nil
}

// suspend writes that the run is suspended, and ends it.
func (p *Plugin) suspend() error {
	if p.rec == nil {
		return fmt.Errorf("suspend: the run has no replay file")
	}
	if p.game.Over != "" {
		return fmt.Errorf("suspend: the run is over")
	}
	err := p.rec.Action(p.game.PlayTime(), replaySuspend, nil, nil)
	p.stop()
	return err
}

// step watches n more actions of the replay (all of them for n < 0); it
// says whether the replay is over.
func (p *Plugin) step(n int) (bool, error) {
	w := p.watching
	for ; n != 0 && w.next < len(w.replay.Actions); n-- {
		a := w.replay.Actions[w.next]
		w.next++
		if a.Action == replaySuspend {
			continue
		}
		if err := p.game.replayAction(a); err != nil {
			return true, err
		}
	}
	return w.next >= len(w.replay.Actions), nil
}

// watchScreen is the screen of the replay watched: no prompt to answer,
// and how far it is under it.
func (p *Plugin) watchScreen(size Size) Screen {
	s := p.game.Render(Size{size.Width, size.Height - 1})
	s.Prompt = nil
	w := p.watching
	s.Lines = append(s.Lines, fmt.Sprintf("リプレイ  %d/%d", w.next, len(w.replay.Actions)))
	return s
}

// Handlers implements sdk.Plugin.
//
//	start(options, size): start a new game, and return its Screen.
//	dungeons(): the dungeons there are: id, name, description, floors.
//	do(action, args, size): run an action of the game, and return the
//	Screen after it. "suspend" suspends the run, and returns nothing.
//	last(path): what the replay file path of the last run holds
//	(LastRun).
//	resume(path, size): resume the suspended run of the replay file
//	path, and return its Screen.
//	watch(path, size): watch the replay file path from its start, and
//	return the Screen.
//	step(n, size): watch n more actions of the replay (-1: to its end),
//	and return {screen, done}.
//	records(path): the lines that show the records of the file path.
func (p *Plugin) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{
		"start": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var opts StartOptions
			var size Size
			if err := sdk.DecodeArgs(params, &opts, &size); err != nil {
				return nil, err
			}
			if opts.Seed == "" {
				opts.Seed = fmt.Sprint(rand.N(100000000))
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			if opts.Dungeon == "" {
				opts.Dungeon = DefaultDungeon
			}
			d, err := LoadBuiltin(opts.Dungeon)
			if err != nil {
				return nil, fmt.Errorf("dungeon %q: %w", opts.Dungeon, err)
			}
			p.stop()
			g := NewGame(d, opts.Seed)
			g.DebugMode = opts.Debug
			if opts.Replay != "" {
				rec, err := NewRecorder(opts.Replay, g)
				if err != nil {
					return nil, fmt.Errorf("replay: %w", err)
				}
				p.rec = rec
			}
			p.game = g
			return sdk.Generic(g.Render(size))
		},
		"dungeons": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var list []map[string]any
			for _, id := range BuiltinDungeons() {
				d, err := LoadBuiltin(id)
				if err != nil {
					return nil, fmt.Errorf("dungeon %q: %w", id, err)
				}
				list = append(list, map[string]any{
					"id": d.ID, "name": d.Name, "description": d.Description, "floors": d.Floors,
				})
			}
			return sdk.Generic(list)
		},
		"do": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var action string
			var args []string
			var size Size
			if err := sdk.DecodeArgs(params, &action, &args, &size); err != nil {
				return nil, err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.game == nil {
				return nil, fmt.Errorf("no game: start one with goda_rogue#start()")
			}
			if p.watching != nil {
				switch action {
				case "redraw", "map", "log":
				default:
					return nil, fmt.Errorf("a replay is being watched: start a game with goda_rogue#start()")
				}
				if action == "map" {
					p.game.ShowMap = !p.game.ShowMap
				}
				screen := p.watchScreen(size)
				if action == "log" {
					screen.Log = slices.Clone(p.game.Log)
				}
				return sdk.Generic(screen)
			}
			if action == replaySuspend {
				return nil, p.suspend()
			}
			if err := p.do(action, args); err != nil {
				return nil, err
			}
			if p.game == nil {
				return nil, fmt.Errorf("no game")
			}
			screen := p.game.Render(size)
			if action == "log" {
				// The whole log, for the Vim script to show in a buffer.
				screen.Log = slices.Clone(p.game.Log)
			}
			return sdk.Generic(screen)
		},
		"last": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var path string
			if err := sdk.DecodeArgs(params, &path); err != nil {
				return nil, err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.rec != nil && p.rec.Path == path && p.game.Over == "" {
				// The run being played: starting another is asked
				// for, not resuming it.
				return sdk.Generic(LastRun{State: "playing"})
			}
			return sdk.Generic(lastRun(path))
		},
		"resume": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var path string
			var size Size
			if err := sdk.DecodeArgs(params, &path, &size); err != nil {
				return nil, err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			p.stop()
			_, g, err := replayGame(path)
			if err != nil {
				return nil, err
			}
			if g.Over != "" {
				return nil, fmt.Errorf("resume: the run of %s is over", path)
			}
			rec, err := AppendRecorder(path)
			if err != nil {
				return nil, fmt.Errorf("replay: %w", err)
			}
			// The play time goes on from where it was suspended.
			g.Started = time.Now()
			g.beginAction()
			g.say("冒険を再開した。")
			p.game, p.rec = g, rec
			return sdk.Generic(g.Render(size))
		},
		"watch": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var path string
			var size Size
			if err := sdk.DecodeArgs(params, &path, &size); err != nil {
				return nil, err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			// Replayed whole first: it tells whether this version replays
			// it the same, before watching it.
			rp, _, err := replayGame(path)
			if err != nil {
				return nil, err
			}
			d, err := LoadBuiltin(rp.Dungeon)
			if err != nil {
				return nil, err
			}
			g, err := rp.NewReplayGame(d)
			if err != nil {
				return nil, err
			}
			p.stop()
			p.game, p.watching = g, &watch{replay: rp}
			return sdk.Generic(p.watchScreen(size))
		},
		"records": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var path string
			if err := sdk.DecodeArgs(params, &path); err != nil {
				return nil, err
			}
			rs, err := readRecords(path)
			if err != nil {
				return nil, err
			}
			names := map[string]string{}
			for _, id := range BuiltinDungeons() {
				if d, err := LoadBuiltin(id); err == nil {
					names[id] = d.Name
				}
			}
			return sdk.Generic(recordLines(rs, names))
		},
		"step": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var n int
			var size Size
			if err := sdk.DecodeArgs(params, &n, &size); err != nil {
				return nil, err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.watching == nil {
				return nil, fmt.Errorf("step: no replay is being watched")
			}
			done, err := p.step(n)
			if err != nil {
				return nil, err
			}
			return sdk.Generic(map[string]any{"screen": p.watchScreen(size), "done": done})
		},
	}
}
