// Package rogue is goda-rogue.vim: a mystery-dungeon roguelike in Vim and
// Neovim, whose map is drawn with full-width characters (enemies are
// kanji, their rank is their color).
//
// The Go side owns the whole game; the Vim script sends the actions and
// shows the screens Render returns.
package rogue

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/Shougo/goda.vim/sdk"
)

// PluginName is the name of the plugin: goda#request('rogue', ...).
const PluginName = "rogue"

// ScriptVersion is the version of the Vim script (autoload/goda_rogue.vim)
// this binary goes with (sdk.ScriptVersioner). Bump it with s:version there
// when they no longer work together.
const ScriptVersion = 1

// Plugin serves the game.
type Plugin struct {
	mu   sync.Mutex
	game *Game
}

// New returns the plugin, with no game yet.
func New() *Plugin { return &Plugin{} }

// Name implements sdk.Plugin.
func (*Plugin) Name() string { return PluginName }

// ScriptVersion implements sdk.ScriptVersioner.
func (*Plugin) ScriptVersion() int { return ScriptVersion }

// StartOptions are the options of a new game.
type StartOptions struct {
	// Seed makes the floors: the same seed, the same floors. Empty for a
	// random one.
	Seed string `json:"seed"`
}

// Handlers implements sdk.Plugin.
//
//	start(options, size): start a new game, and return its Screen.
//	do(action, args, size): run an action of the game, and return the
//	Screen after it.
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
			p.game = NewGame(opts.Seed)
			return sdk.Generic(p.game.Render(size))
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
			if err := p.game.Do(action, args); err != nil {
				return nil, err
			}
			screen := p.game.Render(size)
			if action == "log" {
				// The whole log, for the Vim script to show in a buffer.
				screen.Log = slices.Clone(p.game.Log)
			}
			return sdk.Generic(screen)
		},
	}
}
