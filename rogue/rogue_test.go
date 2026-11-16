package rogue

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// reachable returns the walkable tiles reachable from p.
func reachable(l *Level, p Point) map[Point]bool {
	seen := map[Point]bool{p: true}
	queue := []Point{p}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := q.Add(d)
			if l.At(n).Walkable() && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

func TestGenerateConnected(t *testing.T) {
	for seed := range 200 {
		g := NewGame(fmt.Sprint(seed))
		l := g.Level
		if len(l.Rooms) != gridCols*gridRows {
			t.Fatalf("seed %d: %d rooms", seed, len(l.Rooms))
		}
		from := reachable(l, g.Player)
		for i, tile := range l.Tiles {
			p := Point{i % l.W, i / l.W}
			if tile.Walkable() && !from[p] {
				t.Fatalf("seed %d: %v not reachable from the player %v", seed, p, g.Player)
			}
			// The edges of the floor stay rock.
			if tile.Walkable() && (p.X == 0 || p.Y == 0 || p.X == l.W-1 || p.Y == l.H-1) {
				t.Fatalf("seed %d: %v on the edge is walkable", seed, p)
			}
		}
		if !from[g.Stairs] {
			t.Fatalf("seed %d: the stairs %v are not reachable", seed, g.Stairs)
		}
		for _, m := range g.Monsters {
			if !l.At(m.Pos).Walkable() || m.Pos == g.Player {
				t.Fatalf("seed %d: monster at %v", seed, m.Pos)
			}
		}
	}
}

// TestSeed: the same seed makes the same floors, whatever the player did
// on the floors before.
func TestSeed(t *testing.T) {
	a, b := NewGame("vim"), NewGame("vim")
	if !slices.Equal(a.Level.Tiles, b.Level.Tiles) || a.Player != b.Player || a.Stairs != b.Stairs {
		t.Fatal("the same seed made different first floors")
	}
	for range 30 {
		for _, d := range []string{"right", "down", "left", "up"} {
			if err := a.Do("move", []string{d}); err != nil {
				t.Fatal(err)
			}
		}
	}
	a.descend()
	b.descend()
	if !slices.Equal(a.Level.Tiles, b.Level.Tiles) || a.Stairs != b.Stairs {
		t.Fatal("the same seed made different second floors")
	}
	if c := NewGame("emacs"); slices.Equal(a.Level.Tiles, c.Level.Tiles) {
		t.Fatal("different seeds made the same floor")
	}
}

func TestRender(t *testing.T) {
	g := NewGame("render")
	for _, size := range []Size{{80, 24}, {200, 60}, {7, 3}} {
		s := g.Render(size)
		// The map is between the top line, and the status line and the
		// messages.
		if len(s.Lines) != min(max(size.Height-statusLines, 1), MapHeight)+statusLines {
			t.Errorf("%v: %d lines", size, len(s.Lines))
		}
		maps := s.Lines[1 : len(s.Lines)-1-messageLines]
		wantW := min(max(size.Width/2, 1), MapWidth)
		wantH := min(max(size.Height-statusLines, 1), MapHeight)
		if len(maps) != wantH {
			t.Errorf("%v: %d lines, want %d", size, len(maps), wantH)
		}
		for i, line := range maps {
			if n := utf8.RuneCountInString(line); n != wantW || len(line) != wantW*glyphBytes {
				t.Errorf("%v: line %d has %d tiles (%d bytes), want %d", size, i+1, n, len(line), wantW)
			}
		}
		c := s.Cursor
		if c[0] < 2 || c[0] > len(maps)+1 || !strings.HasPrefix(maps[c[0]-2][c[1]-1:], glyphPlayer) {
			t.Errorf("%v: the cursor %v is not on the player", size, c)
		}
		for group, spans := range s.Highlights {
			if group == hlHP || group == hlHPLow || group == hlHPEmpty || group == hlOldMessage {
				continue
			}
			for _, sp := range spans {
				line := maps[sp.Line-2]
				if sp.Col < 1 || sp.End > len(line)+1 || (sp.Col-1)%glyphBytes != 0 || (sp.End-sp.Col)%glyphBytes != 0 {
					t.Errorf("%v: %s span %v off the glyphs", size, group, sp)
				}
			}
		}
		if got := s.Highlights[hlPlayer]; len(got) != 1 || got[0].Line != c[0] || got[0].Col != c[1] {
			t.Errorf("%v: player highlight %v, cursor %v", size, got, c)
		}
	}
}

// find returns a game of some seed for which ok holds.
func find(t *testing.T, ok func(*Game) bool) *Game {
	t.Helper()
	for seed := range 1000 {
		g := NewGame(fmt.Sprint(seed))
		if ok(g) {
			return g
		}
	}
	t.Fatal("no such game")
	return nil
}

func TestMoveIntoRock(t *testing.T) {
	g := find(t, func(g *Game) bool { return !g.Level.At(g.Player.Add(Point{0, -1})).Walkable() })
	g.Monsters = nil
	before := g.Player
	if err := g.Do("move", []string{"up"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != before || g.Turn != 0 || g.Facing != "up" {
		t.Errorf("moved into rock: %v, turn %d, facing %s", g.Player, g.Turn, g.Facing)
	}
}

func TestTurnCostsNothing(t *testing.T) {
	g := NewGame("turn")
	if err := g.Do("turn", []string{"upleft"}); err != nil {
		t.Fatal(err)
	}
	if g.Turn != 0 || g.Facing != "upleft" {
		t.Errorf("turn %d, facing %s", g.Turn, g.Facing)
	}
	if err := g.Do("turn", []string{"nowhere"}); err == nil {
		t.Error("an unknown direction was accepted")
	}
}

func TestSwapWithAlly(t *testing.T) {
	g := NewGame("ally")
	var ally *Monster
	for _, m := range g.Monsters {
		if m.Ally {
			ally = m
		}
	}
	if ally == nil {
		t.Fatal("no ally")
	}
	d := Point{ally.Pos.X - g.Player.X, ally.Pos.Y - g.Player.Y}
	var name string
	for n, p := range Directions {
		if p == d {
			name = n
		}
	}
	from, to := g.Player, ally.Pos
	if !g.canStep(d) {
		t.Skip("the ally is around a corner")
	}
	if err := g.Do("move", []string{name}); err != nil {
		t.Fatal(err)
	}
	if g.Player != to || ally.Pos != from {
		t.Errorf("player %v, ally %v; want %v, %v", g.Player, ally.Pos, to, from)
	}
}

func TestStairs(t *testing.T) {
	g := NewGame("stairs")
	if err := g.Do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 1 {
		t.Fatal("went down without stairs")
	}
	g.Player = g.Stairs
	if err := g.Do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	if g.Depth != 2 {
		t.Errorf("depth %d", g.Depth)
	}
}

func TestDash(t *testing.T) {
	g := NewGame("dash")
	g.Monsters = nil
	g.Items = nil
	g.Stairs = Point{-1, -1}
	i := g.Level.RoomAt(g.Player)
	r := g.Level.Rooms[i]
	g.Player = Point{r.X, r.Y}
	if err := g.Do("dash", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	// Stops at the wall, or where it leaves the room by a doorway.
	if g.Player.X != r.X+r.W-1 && g.Level.RoomAt(g.Player) == i {
		t.Errorf("dash stopped at %v in the room %+v", g.Player, r)
	}
}

// TestCorridorsAwayFromWalls: next to the wall of a room, a corridor is
// only right outside a doorway, so rooms do not look merged.
func TestCorridorsAwayFromWalls(t *testing.T) {
	for seed := range 200 {
		l := NewGame(fmt.Sprint(seed)).Level
		for _, r := range l.Rooms {
			if r.Dummy {
				continue
			}
			// The tiles one outside the wall ring.
			for y := r.Y - 2; y <= r.Y+r.H+1; y++ {
				for x := r.X - 2; x <= r.X+r.W+1; x++ {
					p := Point{x, y}
					if r.Around(p) || l.At(p) != TileCorridor {
						continue
					}
					// The wall tile next to it, toward the room.
					w := Point{min(max(x, r.X-1), r.X+r.W), min(max(y, r.Y-1), r.Y+r.H)}
					corner := (x < r.X-1 || x > r.X+r.W) && (y < r.Y-1 || y > r.Y+r.H)
					if corner || l.At(w) != TileCorridor {
						t.Fatalf("seed %d: corridor %v along the wall of %+v", seed, p, r)
					}
				}
			}
		}
	}
}

func TestSafe(t *testing.T) {
	g := NewGame("safe")
	start := g.Player
	if !g.IsSafe(start) {
		t.Fatal("the start tile is not safe")
	}
	for _, d := range []string{"left", "right", "up", "down", "upleft", "upright", "downleft", "downright"} {
		if err := g.Do("move", []string{d}); err != nil {
			t.Fatal(err)
		}
		if g.Player != start {
			break
		}
	}
	if g.Player == start {
		t.Skip("could not move")
	}
	if !g.IsSafe(g.Player) || !g.IsSafe(start) {
		t.Error("walked tiles are not safe")
	}
	n := 0
	for _, s := range g.Safe {
		if s {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d safe tiles, want 2", n)
	}
	if _, group := g.glyph(start); group != hlSafe {
		t.Errorf("the start tile shows as %s", group)
	}
	g.descend()
	if n := len(slices.DeleteFunc(slices.Clone(g.Safe), func(s bool) bool { return !s })); n != 1 {
		t.Errorf("%d safe tiles on a new floor, want 1", n)
	}
}

func TestAttack(t *testing.T) {
	g := find(t, func(g *Game) bool { return g.Level.At(g.Player.Add(Point{1, 0})).Walkable() })
	var ally *Monster
	for _, m := range g.Monsters {
		if m.Ally {
			ally = m
		}
	}
	g.Monsters = nil
	g.Items = nil
	right := g.Player.Add(Point{1, 0})
	// The enemy strikes back; this test is about attacking.
	g.HP, g.MaxHP = 9999, 9999

	if err := g.Do("turn", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if g.Turn != 1 || g.Message != "空振りした。" {
		t.Errorf("attacking the air: turn %d, %q", g.Turn, g.Message)
	}

	ally.Pos = right
	g.Monsters = []*Monster{ally}
	if err := g.Do("attack", nil); err != nil {
		t.Fatal(err)
	}
	if g.Turn != 1 || ally.HP != ally.MaxHP {
		t.Errorf("attacked the ally: turn %d, HP %d", g.Turn, ally.HP)
	}

	m := newMonster(monsterKinds[0], 4)
	m.Pos = right
	g.Monsters = []*Monster{m}
	// Moving toward it only faces it.
	g.Facing = "left"
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if g.Player == right || g.Facing != "right" || m.HP != m.MaxHP {
		t.Errorf("moved into the enemy: %v, facing %s, HP %d", g.Player, g.Facing, m.HP)
	}
	for n := 0; len(g.Monsters) > 0; n++ {
		if n > 100 {
			t.Fatal("the enemy never dies")
		}
		hp := m.HP
		if err := g.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
		if m.HP > hp || (m.HP == hp && !strings.Contains(g.Message, "にかわされた。")) {
			t.Fatalf("HP %d -> %d: %q", hp, m.HP, g.Message)
		}
	}
	if !strings.Contains(g.Message, "鼠を倒した。") {
		t.Errorf("message %q", g.Message)
	}
}

// TestAttackRNG: combat does not change the floors of a seed.
func TestAttackRNG(t *testing.T) {
	a, b := NewGame("rng"), NewGame("rng")
	for range 20 {
		if err := a.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
	}
	a.descend()
	b.descend()
	if !slices.Equal(a.Level.Tiles, b.Level.Tiles) {
		t.Fatal("attacks changed the next floor")
	}
}

func TestFacingHighlight(t *testing.T) {
	g := NewGame("facing")
	for dir, d := range Directions {
		g.Facing = dir
		s := g.Render(Size{200, 60})
		f := g.Player.Add(d)
		spans := s.Highlights[hlFacing]
		want := []Span{{f.Y + 2, f.X*glyphBytes + 1, f.X*glyphBytes + 1 + glyphBytes}}
		if !slices.Equal(spans, want) {
			t.Errorf("%s: %v, want %v", dir, spans, want)
		}
	}
}

func TestMinimap(t *testing.T) {
	g := NewGame("minimap")
	if s := g.Render(Size{80, 24}); s.Minimap != nil {
		t.Fatal("the mini-map is shown without asking")
	}
	if err := g.Do("map", nil); err != nil {
		t.Fatal(err)
	}
	if g.Turn != 0 {
		t.Errorf("the map cost a turn")
	}
	m := g.Render(Size{80, 24}).Minimap
	if m == nil {
		t.Fatal("no mini-map")
	}
	// Only what has been seen: the first room, and the tiles around the
	// player.
	x0, y0, x1, y1 := g.seenBounds()
	r := g.Level.Rooms[g.Level.RoomAt(g.Player)]
	// (Doorways are in its walls.)
	if !r.Around(Point{x0, y0}) || !r.Around(Point{x1, y1}) || x0 > r.X || x1 < r.X+r.W-1 {
		t.Errorf("bounds %d,%d-%d,%d for the room %+v", x0, y0, x1, y1, r)
	}
	w, h := x1-x0+1, y1-y0+1
	if len(m.Lines) != h {
		t.Fatalf("%d lines, want %d", len(m.Lines), h)
	}
	for i, line := range m.Lines {
		if len(line) != w || utf8.RuneCountInString(line) != w {
			t.Fatalf("line %d: %q", i+1, line)
		}
	}
	px, py := g.Player.X-x0, g.Player.Y-y0
	if c := m.Lines[py][px]; c != miniPlayer {
		t.Errorf("%q at the player", c)
	}
	for group, spans := range m.Highlights {
		for _, sp := range spans {
			if sp.Col < 1 || sp.End > w+1 || sp.End <= sp.Col || sp.Line > h {
				t.Errorf("%s span %v", group, sp)
			}
		}
	}
	if got := m.Highlights[hlPlayer]; len(got) != 1 || got[0] != (Span{py + 1, px + 1, px + 2}) {
		t.Errorf("player span %v", got)
	}
	// The whole floor, when all of it has been seen.
	for i := range g.Seen {
		g.Seen[i] = true
	}
	if m := g.minimap(); len(m.Lines) > MapHeight || len(m.Lines[0]) > MapWidth || len(m.Lines) < MapHeight/2 {
		t.Errorf("%d lines of %d", len(m.Lines), len(m.Lines[0]))
	}
	if err := g.Do("map", nil); err != nil {
		t.Fatal(err)
	}
	if g.Render(Size{80, 24}).Minimap != nil {
		t.Error("the mini-map is still shown")
	}
}

func TestFaceEnemy(t *testing.T) {
	g := find(t, func(g *Game) bool {
		for _, d := range []Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			if !g.Level.At(g.Player.Add(d)).Walkable() {
				return false
			}
		}
		return true
	})
	g.Monsters = nil
	g.Facing = "up"
	if err := g.Do("turn", nil); err != nil {
		t.Fatal(err)
	}
	if g.Facing != "up" || g.Message != "周りに敵はいない。" {
		t.Errorf("no enemy: facing %s, %q", g.Facing, g.Message)
	}

	for _, d := range []Point{{-1, 0}, {1, 0}} {
		m := newMonster(monsterKinds[0], 1)
		m.Pos = g.Player.Add(d)
		g.Monsters = append(g.Monsters, m)
	}
	ally := newMonster(monsterKinds[2], 1)
	ally.Ally = true
	ally.Pos = g.Player.Add(Point{0, 1})
	g.Monsters = append(g.Monsters, ally)

	// Clockwise from up: right, then left (not the ally below), and
	// right again. No turn passes.
	for _, want := range []string{"right", "left", "right"} {
		if err := g.Do("turn", nil); err != nil {
			t.Fatal(err)
		}
		if g.Facing != want {
			t.Errorf("facing %s, want %s", g.Facing, want)
		}
	}
	if g.Turn != 0 {
		t.Errorf("turn %d", g.Turn)
	}
}

// roomGame returns a game whose player stands at the top left corner of a
// room at least w x h large, with nothing else on the floor.
func roomGame(t *testing.T, w, h int) (*Game, Room) {
	t.Helper()
	g := find(t, func(g *Game) bool {
		r := g.Level.Rooms[g.Level.RoomAt(g.Player)]
		return r.W >= w && r.H >= h
	})
	r := g.Level.Rooms[g.Level.RoomAt(g.Player)]
	g.Player = Point{r.X, r.Y}
	g.Monsters, g.Items = nil, nil
	g.Stairs = Point{-1, -1}
	return g, r
}

func place(g *Game, k monsterKind, rank int, p Point) *Monster {
	m := newMonster(k, rank)
	m.Pos = p
	g.Monsters = append(g.Monsters, m)
	return m
}

func wait(t *testing.T, g *Game, n int) {
	t.Helper()
	for range n {
		if err := g.Do("wait", nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMonsterChases(t *testing.T) {
	for _, tc := range []struct {
		kind  monsterKind
		turns int
		steps int
	}{
		{monsterKinds[0], 1, 1}, // normal
		{monsterKinds[2], 1, 2}, // double speed
		{monsterKinds[3], 2, 1}, // slow
	} {
		g, r := roomGame(t, 8, 3)
		m := place(g, tc.kind, 1, Point{r.X + 7, r.Y})
		before := chebyshev(m.Pos, g.Player)
		wait(t, g, tc.turns)
		if got := chebyshev(m.Pos, g.Player); got != before-tc.steps {
			t.Errorf("%s: distance %d -> %d in %d turns, want -%d", tc.kind.Name, before, got, tc.turns, tc.steps)
		}
	}
}

func TestMonsterAttacks(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	g.HP, g.MaxHP = 9999, 9999
	place(g, monsterKinds[0], 1, Point{r.X + 1, r.Y})
	wait(t, g, 20)
	if g.HP >= 9999-20/regenTurns {
		t.Errorf("HP %d: the monster next to the player never hit it", g.HP)
	}
}

func TestPlayerDies(t *testing.T) {
	g, r := roomGame(t, 4, 3)
	g.HP = 1
	g.MaxHP = 1
	place(g, monsterKinds[0], 4, Point{r.X + 1, r.Y})
	for n := 0; g.Over == ""; n++ {
		if n > 100 {
			t.Fatal("the player never dies")
		}
		wait(t, g, 1)
	}
	if g.HP != 0 || !strings.Contains(g.Over, "鼠") {
		t.Errorf("HP %d, over: %q", g.HP, g.Over)
	}
	pos, turn := g.Player, g.Turn
	if err := g.Do("move", []string{"down"}); err != nil {
		t.Fatal(err)
	}
	if g.Player != pos || g.Turn != turn || g.Message != "冒険は終わった。" {
		t.Errorf("moved after the end: %v, turn %d, %q", g.Player, g.Turn, g.Message)
	}
}

func TestDozing(t *testing.T) {
	// A room with a doorway on its left wall, and the player on it.
	g := find(t, func(g *Game) bool {
		for _, r := range g.Level.Rooms {
			if !r.Dummy && r.W >= 5 && g.Level.At(Point{r.X - 1, r.Y}) == TileCorridor {
				return true
			}
		}
		return false
	})
	var r Room
	for _, room := range g.Level.Rooms {
		if !room.Dummy && room.W >= 5 && g.Level.At(Point{room.X - 1, room.Y}) == TileCorridor {
			r = room
			break
		}
	}
	g.Monsters, g.Items = nil, nil
	g.Stairs = Point{-1, -1}
	g.HP, g.MaxHP = 9999, 9999
	g.Player = Point{r.X - 1, r.Y}
	m := place(g, monsterKinds[0], 1, Point{r.X + r.W - 1, r.Y + r.H - 1})
	m.Dozing = true
	pos := m.Pos
	wait(t, g, 3)
	if m.Pos != pos || !m.Dozing {
		t.Fatalf("a dozing monster moved: %v", m.Pos)
	}
	if err := g.Do("move", []string{"right"}); err != nil {
		t.Fatal(err)
	}
	if m.Dozing {
		t.Error("entering the room did not wake it")
	}
}

func TestAlly(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	ally := place(g, monsterKinds[0], 1, Point{r.X + 5, r.Y})
	ally.Ally = true
	wait(t, g, 1)
	if d := chebyshev(ally.Pos, g.Player); d != 4 {
		t.Errorf("ally at %d, want 4: it follows the player", d)
	}

	// An enemy next to the ally, not to the player: they fight.
	ally.Pos = Point{r.X + 3, r.Y + 2}
	ally.HP, ally.MaxHP = 9999, 9999
	enemy := place(g, monsterKinds[0], 1, Point{r.X + 4, r.Y + 2})
	for n := 0; slices.Contains(g.Monsters, enemy); n++ {
		if n > 100 {
			t.Fatal("the ally never defeats the enemy")
		}
		// (The ally does not come closer: it stays to fight.)
		wait(t, g, 1)
	}
}

func TestTopLine(t *testing.T) {
	g := NewGame("top")
	for _, tc := range []struct {
		hp           int
		group        string
		filled, rest int
	}{
		{15, hlHP, hpBarWidth, 0},
		{8, hlHP, 6, 4},
		{3, hlHPLow, 2, 8},
		{0, "", 0, hpBarWidth},
	} {
		g.HP, g.MaxHP = tc.hp, 15
		s := g.Render(Size{80, 24})
		top := s.Lines[0]
		if !strings.HasPrefix(top, fmt.Sprintf("1F  Lv1  HP %3d/ 15 ", tc.hp)) {
			t.Errorf("top line %q", top)
		}
		width := func(group string) int {
			n := 0
			for _, sp := range s.Highlights[group] {
				if sp.Line == 1 {
					n += sp.End - sp.Col
				}
			}
			return n
		}
		if tc.group != "" && width(tc.group) != tc.filled {
			t.Errorf("HP %d: %s %d cells, want %d", tc.hp, tc.group, width(tc.group), tc.filled)
		}
		if width(hlHPEmpty) != tc.rest {
			t.Errorf("HP %d: empty %d cells, want %d", tc.hp, width(hlHPEmpty), tc.rest)
		}
	}
}

func TestSpawn(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	g.HP, g.MaxHP = 9999, 9999
	wait(t, g, spawnTurns-1)
	if len(g.Monsters) != 0 {
		t.Fatalf("%d monsters before %d turns", len(g.Monsters), spawnTurns)
	}
	wait(t, g, 1)
	if len(g.Monsters) != 1 {
		t.Fatalf("%d monsters after %d turns", len(g.Monsters), spawnTurns)
	}
	if m := g.Monsters[0]; !m.Spawned || chebyshev(m.Pos, g.Player) <= 4 {
		t.Errorf("spawned %+v, the player at %v", m, g.Player)
	}
}

func TestWind(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.HP, g.MaxHP = 9999, 9999
	m := place(g, monsterKinds[0], 1, Point{r.X + 7, r.Y + 2})
	m.Dozing = true

	g.FloorTurns = windTurns[0] - 1
	wait(t, g, 1)
	if g.Wind != 1 || !strings.Contains(g.Message, "風が吹いてきた") {
		t.Errorf("first wind: %d, %q", g.Wind, g.Message)
	}
	if !strings.Contains(g.Status(), fmt.Sprintf("次の風まで %d", windTurns[1]-windTurns[0])) {
		t.Errorf("status %q", g.Status())
	}

	g.FloorTurns = windTurns[1] - 1
	n := len(g.Monsters)
	wait(t, g, 1)
	if g.Wind != 2 || len(g.Monsters) != n+1 || m.Rank != 2 {
		t.Errorf("second wind: %d, %d monsters, rank %d", g.Wind, len(g.Monsters), m.Rank)
	}
	hunter := g.Monsters[len(g.Monsters)-1]
	if !hunter.Hunter || !hunter.Spawned || hunter.Rank != 3 {
		t.Errorf("the strong monster %+v", hunter)
	}

	g.FloorTurns = windTurns[2] - 1
	wait(t, g, 1)
	if g.Wind != 3 || !strings.Contains(g.Over, "風") {
		t.Errorf("third wind: %d, %q", g.Wind, g.Over)
	}

	g.Over = ""
	g.descend()
	if g.Wind != 0 || g.FloorTurns != 0 {
		t.Errorf("a new floor: wind %d, %d turns", g.Wind, g.FloorTurns)
	}
}

func TestMessages(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	g.Log, g.actionStart = nil, 0
	g.beginAction()
	g.say("一。")
	g.say("二。")
	g.beginAction()
	g.say("三。")
	s := g.Render(Size{80, 24})
	got := s.Lines[len(s.Lines)-messageLines:]
	if !slices.Equal(got, []string{"一。", "二。", "三。"}) {
		t.Errorf("messages %q", got)
	}
	// The first two are of the action before: dimmed.
	n := len(s.Lines)
	want := []Span{{n - 2, 1, len("一。") + 1}, {n - 1, 1, len("二。") + 1}}
	if !slices.Equal(s.Highlights[hlOldMessage], want) {
		t.Errorf("old messages %v, want %v", s.Highlights[hlOldMessage], want)
	}
	if g.Message != "三。" {
		t.Errorf("message %q", g.Message)
	}
	for range maxLog + 10 {
		g.say("。")
	}
	if len(g.Log) != maxLog {
		t.Errorf("log of %d", len(g.Log))
	}
}

func TestLevelUp(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.HP, g.MaxHP = 9999, 9999
	ally := place(g, monsterKinds[0], 1, Point{r.X, r.Y + 1})
	ally.Ally = true
	g.Facing = "right"

	// A spawned monster gives nothing.
	m := place(g, monsterKinds[0], 1, Point{r.X + 1, r.Y})
	m.Spawned, m.HP = true, 1
	g.rng = rand.New(rand.NewPCG(1, 1))
	for slices.Contains(g.Monsters, m) {
		if err := g.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
	}
	if g.EXP != 0 || g.Kills != 1 {
		t.Errorf("a spawned monster gave %d EXP (%d kills)", g.EXP, g.Kills)
	}

	// The player and the ally get the whole EXP each, and level up.
	m = place(g, monsterKinds[0], 1, Point{r.X + 1, r.Y})
	m.HP, m.EXP = 1, levelEXP[1]
	for slices.Contains(g.Monsters, m) {
		if err := g.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
	}
	if g.PlayerLevel != 3 || g.MaxHP != 9999+2*levelHP || !strings.Contains(g.Message, "主はレベル3に上がった。") {
		t.Errorf("player: Lv%d, max HP %d, %q", g.PlayerLevel, g.MaxHP, g.Message)
	}
	if ally.Level != 3 || ally.Gained != levelEXP[1] || ally.Attack != rankStats[1].Attack+2*allyLevelAtk {
		t.Errorf("ally: Lv%d, EXP %d, attack %d", ally.Level, ally.Gained, ally.Attack)
	}
	if got := playerAttack(3, playerStrength); got <= playerAttack(1, playerStrength) {
		t.Errorf("attack at Lv3 %d", got)
	}
}

func TestRankUpKeepsEXP(t *testing.T) {
	m := newMonster(monsterKinds[0], 1)
	rankUp(m)
	if m.Rank != 2 || m.EXP != rankEXP[1] || m.Attack != rankStats[2].Attack {
		t.Errorf("%+v", m)
	}
}

func TestClear(t *testing.T) {
	g := NewGame("clear")
	for g.Depth < finalFloor {
		g.descend()
	}
	g.Player = g.Stairs
	if err := g.Do("stairs", nil); err != nil {
		t.Fatal(err)
	}
	if !g.Cleared || !strings.Contains(g.Over, "踏破") {
		t.Fatalf("cleared %v, %q", g.Cleared, g.Over)
	}
	s := g.Render(Size{80, 24})
	if !slices.Contains(s.Lines, "  踏破！") || !slices.Contains(s.Lines, fmt.Sprintf("  到達      %dF", finalFloor)) {
		t.Errorf("result %q", s.Lines)
	}
}

// TestRandomPlay plays runs with random actions, and checks that nothing
// breaks: no two creatures on one tile, everyone on a walkable tile, HP in
// bounds, and the run ending only for a reason.
func TestRandomPlay(t *testing.T) {
	actions := []string{"move", "move", "move", "dash", "attack", "turn", "wait", "stairs"}
	for seed := range 20 {
		g := NewGame(fmt.Sprint("play", seed))
		rng := rand.New(rand.NewPCG(uint64(seed), 0))
		for range 3000 {
			if g.Over != "" {
				break
			}
			action := actions[rng.IntN(len(actions))]
			var args []string
			if action == "move" || action == "dash" || (action == "turn" && rng.IntN(2) == 0) {
				args = []string{clockwise[rng.IntN(len(clockwise))]}
			}
			if action == "stairs" && rng.IntN(4) == 0 {
				g.Player = g.Stairs
			}
			if err := g.Do(action, args); err != nil {
				t.Fatal(err)
			}
			tiles := map[Point]bool{g.Player: true}
			for _, m := range g.Monsters {
				if tiles[m.Pos] || !g.Level.At(m.Pos).Walkable() {
					t.Fatalf("seed %d: %s on %v", seed, m.Name, m.Pos)
				}
				tiles[m.Pos] = true
				if m.HP <= 0 || m.HP > m.MaxHP {
					t.Fatalf("seed %d: %s HP %d/%d", seed, m.Name, m.HP, m.MaxHP)
				}
			}
			if g.HP < 0 || g.HP > g.MaxHP || (g.HP == 0) != (g.Over != "" && !g.Cleared && !strings.Contains(g.Over, "風")) {
				t.Fatalf("seed %d: HP %d/%d, over %q", seed, g.HP, g.MaxHP, g.Over)
			}
		}
		_ = g.Render(Size{80, 24})
	}
}
