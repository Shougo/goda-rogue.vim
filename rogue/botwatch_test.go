package rogue

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// request calls the handler name of p with params, and returns the screen
// it answers (of a "step", its screen).
func request(t *testing.T, p *Plugin, name string, params ...any) map[string]any {
	t.Helper()
	r, err := p.Handlers()[name](context.Background(), nil, params)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	m, _ := r.(map[string]any)
	if s, ok := m["screen"].(map[string]any); ok {
		s["done"] = m["done"]
		return s
	}
	return m
}

// last returns the last line of the screen s.
func last(s map[string]any) string {
	lines := s["lines"].([]any)
	return lines[len(lines)-1].(string)
}

// TestWatchBot: a bot plays a run, watched step by step to its end; no
// action of the player goes to it.
func TestWatchBot(t *testing.T) {
	p := New()
	t.Cleanup(p.stop)
	path := filepath.Join(t.TempDir(), "bot.replay")
	size := map[string]any{"width": 80, "height": 24}
	s := request(t, p, "bot", map[string]any{"dungeon": "cave", "seed": "watch", "style": "reckless", "replay": path}, size)
	if last(s) != "Bot（reckless）  0 手" || s["prompt"] != nil {
		t.Fatalf("last line %q, prompt %v", last(s), s["prompt"])
	}
	s = request(t, p, "step", 5, size)
	if last(s) != "Bot（reckless）  5 手" || s["done"] != false || p.game.Prompt != nil {
		t.Fatalf("last line %q, done %v, prompt %+v", last(s), s["done"], p.game.Prompt)
	}
	if _, err := p.Handlers()["do"](context.Background(), nil, []any{"wait", []any{}, size}); err == nil || !strings.Contains(err.Error(), "being watched") {
		t.Errorf("an action while watching: %v", err)
	}
	s = request(t, p, "step", -1, size)
	if s["done"] != true || p.game.Over == "" {
		t.Fatalf("done %v, over %q", s["done"], p.game.Over)
	}
	// Stepping on does nothing.
	n := p.watching.next
	if s = request(t, p, "step", 3, size); p.watching.next != n || s["done"] != true {
		t.Errorf("stepped on to %d", p.watching.next)
	}

	// Its replay plays the same run, with no record of it.
	over, turn := p.game.Over, p.game.Turn
	p.stop()
	if _, g, err := replayGame(path); err != nil || g.Over != over || g.Turn != turn {
		t.Errorf("replay: %v, over %q turn %d, want %q %d", err, g.Over, g.Turn, over, turn)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), recordsFile)); err == nil {
		t.Error("the run of a bot has a record")
	}

	if _, err := p.Handlers()["bot"](context.Background(), nil, []any{map[string]any{"style": "wise"}, size}); err == nil {
		t.Error("an unknown style")
	}
}

// TestOverPrompt: over, an action in the dungeon asks what comes next;
// the answer tells the Vim script.
func TestOverPrompt(t *testing.T) {
	p := New()
	size := map[string]any{"width": 80, "height": 24}
	request(t, p, "start", map[string]any{"dungeon": "cave", "seed": "over"}, size)
	p.game.Over = "力尽きた"
	do := func(action string, args ...any) map[string]any {
		t.Helper()
		if args == nil {
			args = []any{}
		}
		return request(t, p, "do", action, args, size)
	}
	if s := do("map"); s["prompt"] != nil {
		t.Errorf("the map asks: %v", s["prompt"])
	}
	s := do("wait")
	pr, _ := s["prompt"].(map[string]any)
	if pr == nil || pr["title"] != "冒険は終わった（力尽きた）" || len(pr["choices"].([]any)) != 3 {
		t.Fatalf("prompt %v", s["prompt"])
	}
	if s = do("redraw"); s["prompt"] == nil {
		t.Error("a redraw drops the prompt")
	}
	if s = do("choose", "2"); s["prompt"] != nil || s["next"] != nil {
		t.Errorf("the result: prompt %v, next %v", s["prompt"], s["next"])
	}
	// Not asked: a choice is nothing.
	if s = do("choose", "0"); s["next"] != nil {
		t.Errorf("next %v without a prompt", s["next"])
	}
	do("attack")
	if s = do("cancel"); s["prompt"] != nil || s["next"] != nil {
		t.Errorf("cancelled: prompt %v, next %v", s["prompt"], s["next"])
	}
	do("attack")
	if s = do("choose", "0"); s["next"] != "retry" || s["dungeon"] != "cave" {
		t.Errorf("retry: next %v, dungeon %v", s["next"], s["dungeon"])
	}
	do("attack")
	if s = do("choose", "1"); s["next"] != "dungeons" {
		t.Errorf("dungeons: next %v", s["next"])
	}
}
