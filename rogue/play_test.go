package rogue

import (
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// playRuns is how many runs TestRandomPlay plays: more with -play.runs, to
// look for bugs longer than the tests usually take (go test ./rogue -run
// RandomPlay -play.runs 1000). By default 24, or 8 with the race detector.
var playRuns = flag.Int("play.runs", 0, "the runs of TestRandomPlay (0: the default)")

// player plays a run with random actions. It favors what a player does
// (walking, answering the prompts), heads for the stairs now and then, to
// go down, and may be given items (the debug mode), to use them all.
type player struct {
	g   *Game
	rng *rand.Rand
	// stairs is the chance in 1000 of an action to head for the stairs;
	// give that of being given an item.
	stairs, give int
	// going is how many more actions it heads for the stairs; attack
	// says whether it attacks what stood in its way.
	going  int
	attack bool
}

// playerActions are the actions of a player, weighted.
var playerActions = []struct {
	action string
	weight int
}{
	{"move", 30}, {"dash", 6}, {"attack", 10}, {"turn", 4}, {"wait", 6},
	{"inventory", 12}, {"pickup", 4}, {"shoot", 3}, {"look", 2}, {"stairs", 1},
	{"map", 1}, {"redraw", 1},
}

// next returns the next action, and its arguments.
func (p *player) next() (string, []string) {
	g, rng := p.g, p.rng
	if g.Prompt != nil && rng.IntN(10) < 8 {
		if rng.IntN(8) == 0 {
			return "cancel", nil
		}
		return "choose", []string{fmt.Sprint(rng.IntN(len(g.Prompt.Choices)))}
	}
	if rng.IntN(1000) < p.give && len(g.Inventory) < g.Dungeon.Rules.Capacity {
		items := g.Dungeon.Items
		return "debug", []string{debugItem, items[rng.IntN(len(items))].ID}
	}
	if p.going == 0 && rng.IntN(1000) < p.stairs {
		p.going = 200
	}
	if p.going > 0 {
		p.going--
		if action, args := p.toStairs(); action != "" {
			return action, args
		}
		p.going = 0
	}
	n := rng.IntN(100)
	for _, a := range playerActions {
		if n -= a.weight; n < 0 {
			switch a.action {
			case "move", "dash":
				return a.action, []string{clockwise[rng.IntN(len(clockwise))]}
			case "turn":
				if rng.IntN(2) == 0 {
					return a.action, []string{clockwise[rng.IntN(len(clockwise))]}
				}
			}
			return a.action, nil
		}
	}
	return "wait", nil
}

// toStairs returns the next action toward the stairs (and down them), or
// "" if they cannot be reached.
func (p *player) toStairs() (string, []string) {
	g := p.g
	if p.attack {
		p.attack = false
		return "attack", nil
	}
	if g.Player == g.Stairs {
		p.going = 0
		return "stairs", nil
	}
	// The distances to the stairs.
	dist := map[Point]int{g.Stairs: 0}
	queue := []Point{g.Stairs}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, d := range Directions {
			n := q.Add(d)
			if _, ok := dist[n]; ok || !g.Level.At(n).Walkable() || !g.clearFrom(q, d) {
				continue
			}
			dist[n] = dist[q] + 1
			queue = append(queue, n)
		}
	}
	here, ok := dist[g.Player]
	if !ok {
		return "", nil
	}
	for _, name := range clockwise {
		n := g.Player.Add(Directions[name])
		if dn, ok := dist[n]; !ok || dn >= here || !g.clearFrom(g.Player, Directions[name]) {
			continue
		}
		if m := g.MonsterAt(n); m != nil && !m.Ally {
			if m.hostile() {
				p.attack = true
				return "turn", []string{name}
			}
			continue
		}
		return "move", []string{name}
	}
	return "", nil
}

// TestRandomPlay plays runs with random actions, recording their replays,
// and checks after each action that the game is sound (checkGame), and
// that the replays replay the same.
func TestRandomPlay(t *testing.T) {
	d := cave()
	runs := *playRuns
	if runs == 0 {
		runs = 24
		if raceDetector {
			runs = 8
		}
	}
	for run := range runs {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			t.Parallel()
			g := NewGame(d, fmt.Sprint("play", run))
			// Half the runs are given items, and reach the deep floors
			// (the player cannot fall); the others play as they go.
			p := &player{g: g, rng: rand.New(rand.NewPCG(uint64(run), 0)), stairs: 3}
			if run%2 == 1 {
				g.DebugMode, g.Invincible = true, true
				p.stairs, p.give = 10, 30
			}
			path := filepath.Join(t.TempDir(), "run.replay")
			rec, err := NewRecorder(path, g)
			if err != nil {
				t.Fatal(err)
			}
			var log []string
			for range 4000 {
				if g.Over != "" {
					break
				}
				action, args := p.next()
				log = append(log, strings.Join(append([]string{action}, args...), " "))
				if err := g.Do(action, args); err != nil {
					t.Fatalf("%s: %v", log[len(log)-1], err)
				}
				if recorded(action) {
					if err := rec.Action(0, action, args, g.takeEvents()); err != nil {
						t.Fatal(err)
					}
				}
				if err := checkGame(g); err != nil {
					t.Fatalf("after %d actions (the last: %s), %dF turn %d: %v",
						len(log), strings.Join(log[max(len(log)-8, 0):], ", "), g.Depth, g.Turn, err)
				}
			}
			if err := rec.Close(); err != nil {
				t.Fatal(err)
			}
			// The invincible runs carry on to the bottom, so that they
			// reach it.
			if run%2 == 1 && !g.Cleared && g.Over == "" {
				for g.Depth < d.Floors {
					if err := g.Do("debug", []string{debugFloor, fmt.Sprint(g.Depth + 1)}); err != nil {
						t.Fatal(err)
					}
					if err := checkGame(g); err != nil {
						t.Fatalf("down to the bottom, %dF: %v", g.Depth, err)
					}
				}
			}
			_ = g.Render(Size{80, 24})

			rp, err := ReadReplay(path)
			if err != nil {
				t.Fatal(err)
			}
			if run%2 == 1 {
				// The debug runs: only what was recorded.
				return
			}
			got, err := rp.Play(d, len(rp.Actions))
			if err != nil {
				t.Fatal(err)
			}
			if a, b := fingerprint(got), fingerprint(g); a != b {
				t.Errorf("the replay ends in\n%s\nnot\n%s", a, b)
			}
		})
	}
}

// fingerprint is the state of g to compare: the screen, and what it does
// not show. It sets the play time (on the screen), which replaying does not
// keep.
func fingerprint(g *Game) string {
	start := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	g.Played, g.Started, g.Ended = 0, start, start
	var b strings.Builder
	fmt.Fprintf(&b, "%dF turn %d HP %d/%d EXP %d gold %d fullness %d over %q\n",
		g.Depth, g.Turn, g.HP, g.MaxHP, g.EXP, g.Gold, g.Fullness, g.Over)
	for _, m := range g.Monsters {
		fmt.Fprintf(&b, "%s %v %d\n", m.Def.ID, m.Pos, m.HP)
	}
	for _, it := range slices.Concat(g.Items, g.Inventory) {
		fmt.Fprintf(&b, "%s %v %d %d %d\n", it.Def.ID, it.Pos, it.Upgrade, it.Charges, it.Count)
	}
	b.WriteString(strings.Join(g.Render(Size{80, 24}).Lines, "\n"))
	return b.String()
}

// checkGame checks that g is sound, and returns what is wrong.
func checkGame(g *Game) error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	rules, lv := g.Dungeon.Rules, g.Level

	// The run.
	if g.Depth < 1 || g.Depth > g.Dungeon.Floors {
		fail("on %dF of %d", g.Depth, g.Dungeon.Floors)
	}
	won := g.Cleared || strings.Contains(g.Over, "風")
	if g.HP < 0 || g.HP > g.MaxHP || (g.HP == 0) != (g.Over != "" && !won) {
		fail("HP %d/%d, over %q", g.HP, g.MaxHP, g.Over)
	}
	if g.Cleared && g.Over == "" {
		fail("cleared, not over")
	}
	if g.Fullness < 0 || g.Fullness > rules.Fullness {
		fail("fullness %d of %d", g.Fullness, rules.Fullness)
	}
	if g.Gold < 0 || g.PlayerLevel < 1 || g.Strength < 0 || g.EXP < 0 {
		fail("gold %d, level %d, strength %d, EXP %d", g.Gold, g.PlayerLevel, g.Strength, g.EXP)
	}
	if n := lv.W * lv.H; len(g.Seen) != n || len(g.Safe) != n {
		fail("%d seen, %d safe tiles of %d", len(g.Seen), len(g.Safe), n)
	}
	if len(g.Log) > maxLog || g.actionStart > len(g.Log) {
		fail("%d messages, the action's from %d", len(g.Log), g.actionStart)
	}
	if g.Prompt != nil && len(g.Prompt.Choices) == 0 {
		fail("the prompt %q has no choice", g.Prompt.Title)
	}
	if !lv.At(g.Stairs).Walkable() {
		fail("the stairs on %v", g.Stairs)
	}

	// The creatures: one a tile, on the floor, alive.
	if !lv.At(g.Player).Walkable() {
		fail("the player on %v", g.Player)
	}
	tiles := map[Point]bool{g.Player: true}
	for _, m := range g.Monsters {
		if tiles[m.Pos] || !lv.At(m.Pos).Walkable() {
			fail("%s on %v", m.Name, m.Pos)
		}
		tiles[m.Pos] = true
		if (m.HP <= 0 && g.Over == "") || m.HP > m.MaxHP {
			fail("%s HP %d/%d", m.Name, m.HP, m.MaxHP)
		}
		if m.Rank < 1 || m.Rank > maxRank {
			fail("%s of rank %d", m.Name, m.Rank)
		}
	}

	// The items: each in one place; on the floor one a tile, not on the
	// stairs.
	where := map[*Item]string{}
	put := func(it *Item, at string) {
		if w, ok := where[it]; ok {
			fail("%s both %s and %s", it.Def.ID, w, at)
		}
		where[it] = at
	}
	floor := map[Point]*Item{}
	for _, it := range g.Items {
		put(it, fmt.Sprintf("on %v", it.Pos))
		if o := floor[it.Pos]; o != nil {
			fail("%s and %s on %v", o.Def.ID, it.Def.ID, it.Pos)
		}
		floor[it.Pos] = it
		if !lv.At(it.Pos).Walkable() || it.Pos == g.Stairs {
			fail("%s on %v (the stairs %v)", it.Def.ID, it.Pos, g.Stairs)
		}
		if it.Equipped {
			fail("%s equipped on the floor", it.Def.ID)
		}
		if it.ForSale && !g.inShop(it.Pos) {
			fail("%s for sale out of the shop, on %v", it.Def.ID, it.Pos)
		}
	}
	if len(g.Inventory) > rules.Capacity {
		fail("%d items carried of %d", len(g.Inventory), rules.Capacity)
	}
	worn := map[Category]*Item{}
	for _, it := range g.Inventory {
		put(it, "carried")
		if it.Equipped {
			if !it.equipment() {
				fail("%s equipped", it.Def.ID)
			}
			if o := worn[it.Def.Category]; o != nil {
				fail("%s and %s equipped", o.Def.ID, it.Def.ID)
			}
			worn[it.Def.Category] = it
		}
	}
	if g.debt() > 0 && !g.keepersCalm() {
		fail("a debt of %dG to no shopkeeper", g.debt())
	}
	var pots func(items []*Item, in string)
	pots = func(items []*Item, in string) {
		for _, pot := range items {
			if pot.Def.Category != CategoryPot {
				continue
			}
			if len(pot.Contents) > pot.Charges {
				fail("%s %s holds %d of %d", pot.Def.ID, in, len(pot.Contents), pot.Charges)
			}
			for _, it := range pot.Contents {
				put(it, "in "+pot.Def.ID)
				if it.Equipped || it.Def.Category == CategoryPot {
					fail("%s in %s (equipped %v)", it.Def.ID, pot.Def.ID, it.Equipped)
				}
				if it.ForSale && !pot.ForSale {
					fail("%s for sale in %s, not", it.Def.ID, pot.Def.ID)
				}
			}
		}
	}
	pots(g.Items, "on the floor")
	pots(g.Inventory, "carried")
	for it, w := range where {
		if it.Def.Category == CategoryArrow && it.Count < 1 {
			fail("%d %s %s", it.Count, it.Def.ID, w)
		}
		if it.Charges < 0 || it.Cracks < 0 {
			fail("%s %s: %d charges, %d cracks", it.Def.ID, w, it.Charges, it.Cracks)
		}
		if it.Def.Category == CategoryMoney && (it.Amount <= 0 || w == "carried") {
			fail("%dG %s", it.Amount, w)
		}
	}

	// The traps: one a tile, on the floor.
	traps := map[Point]bool{}
	for _, t := range g.Traps {
		if traps[t.Pos] || !lv.At(t.Pos).Walkable() || t.Pos == g.Stairs {
			fail("a trap on %v", t.Pos)
		}
		traps[t.Pos] = true
	}

	// The shop: its shopkeepers on the floor, in it while calm.
	if s := g.Shop; s != nil {
		for _, m := range s.Keepers {
			if !slices.Contains(g.Monsters, m) {
				continue
			}
			if !m.Keeper || (!m.Angry && !g.inShop(m.Pos) && m.Pos != m.door) {
				fail("the shopkeeper on %v (angry %v)", m.Pos, m.Angry)
			}
		}
	}
	return errors.Join(errs...)
}

// TestCheckGame: checkGame finds what is wrong.
func TestCheckGame(t *testing.T) {
	g := NewGame(cave(), "check")
	if err := checkGame(g); err != nil {
		t.Fatal(err)
	}
	for name, spoil := range map[string]func(g *Game){
		"HP":     func(g *Game) { g.HP = g.MaxHP + 1 },
		"dead":   func(g *Game) { g.HP = 0 },
		"stairs": func(g *Game) { g.Items = append(g.Items, &Item{Def: g.Dungeon.Items[0], Pos: g.Stairs}) },
		"twice":  func(g *Game) { g.Inventory = append(g.Inventory, g.Inventory[0]) },
		"monster": func(g *Game) {
			g.Monsters = append(g.Monsters, &Monster{Def: kind("nezumi1"), Pos: g.Player, HP: 1, MaxHP: 1, Rank: 1})
		},
		"capacity": func(g *Game) {
			for len(g.Inventory) <= g.Dungeon.Rules.Capacity {
				g.Inventory = append(g.Inventory, &Item{Def: g.Dungeon.Items[0]})
			}
		},
	} {
		g := NewGame(cave(), "check")
		g.Inventory = append(g.Inventory, &Item{Def: g.Dungeon.Items[0]})
		spoil(g)
		if checkGame(g) == nil {
			t.Errorf("%s: nothing wrong", name)
		}
	}
}
