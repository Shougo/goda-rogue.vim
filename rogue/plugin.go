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
const ScriptVersion = 4

// Plugin serves the game.
type Plugin struct {
	mu   sync.Mutex
	game *Game
	// rec writes the replay file of the game, if it has one.
	rec *Recorder
	// watching is the replay or the bot being watched, if any (then game
	// is its run).
	watching *watch
	// asking says whether the game, over, is asking what to do next
	// (overPrompt).
	asking bool
}

// The choices at the end of a run (overPrompt), and what the Vim script
// does for them (Screen.Next).
var overChoices = []string{"同じダンジョンに再挑戦する", "同じシードでもう一度", "ダンジョンを選ぶ", "結果を見る"}

const (
	nextRetry    = "retry"
	nextDungeons = "dungeons"
)

// overAction is an action on a run that is over: the keys that would act
// in the dungeon ask what to do next, and the answer goes to the Vim
// script, which starts the next run. It says whether it took the action.
func (p *Plugin) overAction(action string, args []string, s *Screen) bool {
	switch action {
	case "redraw", "log", "map", "items":
		// Keep asking over a redraw.
		if !p.asking || action != "redraw" {
			p.asking = false
			return false
		}
	case "cancel":
		p.asking = false
	case "choose":
		if !p.asking || len(args) != 1 {
			p.asking = false
			break
		}
		p.asking = false
		switch args[0] {
		case "0":
			s.Next, s.Dungeon = nextRetry, p.game.Dungeon.ID
		case "1":
			// The same floors again, to play them another way.
			s.Next, s.Dungeon, s.Seed = nextRetry, p.game.Dungeon.ID, p.game.Seed
		case "2":
			s.Next = nextDungeons
		}
	default:
		p.asking = true
	}
	if p.asking {
		s.Prompt = &Prompt{Title: "冒険は終わった（" + p.game.Over + "）", Choices: overChoices}
	}
	return true
}

// watch is a replay being watched: its run is after next actions. Or it
// is a bot playing, to be watched (replay is nil): next is how many
// actions it made.
type watch struct {
	replay *Replay
	next   int
	bot    *Bot
}

// BotOptions are the options of a run a bot plays, to be watched.
type BotOptions struct {
	// Dungeon and Seed are as in StartOptions.
	Dungeon string `json:"dungeon"`
	Seed    string `json:"seed"`
	// Style is how the bot plays (BotStyles); empty for careful.
	Style string `json:"style"`
	// Replay is the path of the replay file to write the run to (made
	// anew); empty for none.
	Replay string `json:"replay"`
	// Keep is how many replays of the runs of bots before are kept, in
	// the directory keptBots beside the file; 0 for none.
	Keep int `json:"keep"`
}

// botNames are the styles of the bots, as the menu shows them.
var botNames = map[BotStyle]string{
	BotCareful:  "慎重な Bot（探索し、休み、危ないときは降りる）",
	BotReckless: "速攻の Bot（階段を見つけたらすぐ降りる）",
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
	// Keep is how many replays of the runs before are kept, when the
	// replay file is made anew (keepReplay); 0 for none.
	Keep int `json:"keep"`
}

// LastRun says what the replay file of the last run holds.
type LastRun struct {
	// State is "none" (no file), "playing" (the run being played),
	// "suspended" (it can be resumed: suspended, or left without
	// suspending), "over" or "broken" (it cannot be resumed: Error says
	// why).
	State string `json:"state"`
	Error string `json:"error,omitempty"`
	// (Not omitted when zero: a run left at turn 0 has turn 0, which
	// the Vim script shows.)
	Dungeon string `json:"dungeon"`
	Name    string `json:"name"`
	Depth   int    `json:"depth"`
	Turn    int    `json:"turn"`
	Time    string `json:"time"`
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

// samePath says whether the paths a and b name the same file: the same
// string, or two names of one file. On Windows the same path may come
// in a short (8.3) and a long form (expand() in Vim gives the long one
// once the directory exists), or with other slashes.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

// stop ends the game being played or watched.
func (p *Plugin) stop() {
	if p.rec != nil {
		p.rec.Close()
		p.rec = nil
	}
	p.game, p.watching, p.asking = nil, nil, false
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
	if w.bot != nil {
		return p.botStep(n)
	}
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

// botStep lets the bot watched make n more actions (to the end of its run
// for n < 0); it says whether the run is over. The answers to the prompts
// the bot opens go with the action that opened them: one step is one
// thing done.
func (p *Plugin) botStep(n int) (over bool, err error) {
	w, g := p.watching, p.game
	defer func() {
		if r := recover(); r != nil {
			over, err = true, fmt.Errorf("a bug of goda-rogue: %v (%s, seed %s, after %d actions of the %s bot)",
				r, g.Dungeon.ID, g.Seed, w.next, w.bot.Style)
			if p.rec != nil {
				err = fmt.Errorf("%w (its replay: %s)", err, p.rec.Path)
			}
			p.stop()
		}
	}()
	for ; n != 0 && g.Over == ""; n-- {
		for {
			action, args := w.bot.Next(g)
			if err := g.Do(action, args); err != nil {
				return true, err
			}
			events := g.takeEvents()
			if p.rec != nil && recorded(action) {
				if err := p.rec.Action(g.PlayTime(), action, args, events); err != nil {
					return true, fmt.Errorf("replay %s: %w", p.rec.Path, err)
				}
			}
			if g.Prompt == nil || g.Over != "" {
				break
			}
		}
		w.next++
	}
	return g.Over != "", nil
}

// watchScreen is the screen of the replay or of the bot watched: no
// prompt to answer, and how far it is under it.
func (p *Plugin) watchScreen(size Size) Screen {
	s := p.game.Render(Size{size.Width, size.Height - 1})
	s.Prompt = nil
	w := p.watching
	if w.bot != nil {
		s.Lines = append(s.Lines, fmt.Sprintf("Bot（%s）  %d 手", w.bot.Style, w.next))
		return s
	}
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
//	bot(options, size): start a run a bot plays, to be watched, and
//	return its Screen.
//	bots(): the bots there are: style, name.
//	step(n, size): watch n more actions of the replay or of the bot (-1:
//	to the end), and return {screen, done}.
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
				if err := keepReplay(opts.Replay, opts.Keep); err != nil {
					return nil, fmt.Errorf("keep the last replay: %w", err)
				}
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
				case "redraw", "map", "items", "log":
				default:
					return nil, fmt.Errorf("a replay or a bot is being watched: start a game with goda_rogue#start()")
				}
				switch action {
				case "map":
					p.game.ShowMap = !p.game.ShowMap
				case "items":
					p.game.ShowItems = !p.game.ShowItems
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
			if p.game.Over != "" {
				screen := p.game.Render(size)
				if p.overAction(action, args, &screen) {
					return sdk.Generic(screen)
				}
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
			if p.rec != nil && samePath(p.rec.Path, path) && p.game.Over == "" {
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
		"bot": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var opts BotOptions
			var size Size
			if err := sdk.DecodeArgs(params, &opts, &size); err != nil {
				return nil, err
			}
			if opts.Seed == "" {
				opts.Seed = fmt.Sprint(rand.N(100000000))
			}
			if opts.Dungeon == "" {
				opts.Dungeon = DefaultDungeon
			}
			if opts.Style == "" {
				opts.Style = BotCareful.String()
			}
			style, ok := BotStyles[opts.Style]
			if !ok {
				return nil, fmt.Errorf("bot: unknown style %q", opts.Style)
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			d, err := LoadBuiltin(opts.Dungeon)
			if err != nil {
				return nil, fmt.Errorf("dungeon %q: %w", opts.Dungeon, err)
			}
			// No record, and a replay file of its own: the run is the
			// bot's, not the player's.
			p.stop()
			g := NewGame(d, opts.Seed)
			if opts.Replay != "" {
				dir := filepath.Join(filepath.Dir(opts.Replay), keptBots)
				if err := keepReplayIn(opts.Replay, dir, opts.Keep); err != nil {
					return nil, fmt.Errorf("keep the last replay of a bot: %w", err)
				}
				rec, err := NewRecorder(opts.Replay, g)
				if err != nil {
					return nil, fmt.Errorf("replay: %w", err)
				}
				p.rec = rec
			}
			p.game = g
			p.watching = &watch{bot: NewBot(style, rand.Uint64())}
			return sdk.Generic(p.watchScreen(size))
		},
		"bots": func(ctx context.Context, h sdk.Host, params []any) (any, error) {
			var list []map[string]any
			for _, s := range []BotStyle{BotCareful, BotReckless} {
				list = append(list, map[string]any{"style": s.String(), "name": botNames[s]})
			}
			return sdk.Generic(list)
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
				return nil, fmt.Errorf("step: no replay nor bot is being watched")
			}
			done, err := p.step(n)
			if err != nil {
				return nil, err
			}
			return sdk.Generic(map[string]any{"screen": p.watchScreen(size), "done": done})
		},
	}
}
