package rogue

import (
	"fmt"
	"strings"
)

// The glyphs of the map. All of them are full-width (East Asian Width F or
// W, never ambiguous), so a tile is two cells, and three bytes in UTF-8.
const (
	glyphPlayer  = "主"
	glyphWall    = "＃"
	glyphFloor   = "．"
	glyphUnknown = "　"
	glyphStairs  = "段"
	glyphBytes   = 3
)

// The highlight groups, defined by the Vim script.
const (
	hlPlayer = "GodaRoguePlayer"
	hlWall   = "GodaRogueWall"
	hlFloor  = "GodaRogueFloor"
	// hlSafe is the floor known to have no trap.
	hlSafe   = "GodaRogueFloorSafe"
	hlStairs = "GodaRogueStairs"
	hlRank   = "GodaRogueRank"
	hlAlly   = "GodaRogueAlly"
	// hlKeeper is a shopkeeper (not angry): the color of its rank,
	// reversed.
	hlKeeper = "GodaRogueKeeper"
	// hlFacing is the tile the player faces (and attacks), over its own
	// highlight.
	hlFacing = "GodaRogueFacing"
	// hlOldMessage is a message of an action before the last one.
	hlOldMessage = "GodaRogueOldMessage"
	// The HP bar of the top line: its filled part (hlHPLow when HP is
	// low), and the rest.
	hlHP      = "GodaRogueHP"
	hlHPLow   = "GodaRogueHPLow"
	hlHPEmpty = "GodaRogueHPEmpty"
	// hlTitle is the title of the result screen.
	hlTitle = "GodaRogueTitle"
)

// Size is the size of the window in cells.
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Span is a highlighted part of a line: its line and its byte columns
// [Col, End), from 1.
type Span struct {
	Line int `json:"line"`
	Col  int `json:"col"`
	End  int `json:"end"`
}

// Screen is what the Vim script shows: the lines of the buffer, the parts
// of them to highlight by group, and where the cursor goes (the player).
// With the mini-map shown, Minimap is it.
type Screen struct {
	Lines      []string          `json:"lines"`
	Highlights map[string][]Span `json:"highlights"`
	Cursor     [2]int            `json:"cursor"`
	Minimap    *Minimap          `json:"minimap,omitempty"`
	// Log is all the messages, for the "log" action.
	Log []string `json:"log,omitempty"`
	// Prompt is a choice to ask the player for.
	Prompt *Prompt `json:"prompt,omitempty"`
}

// Minimap is the floor, reduced: one half-width ASCII character a tile.
// Only the part the player has seen is in it, so that it covers as little
// of the map as it can.
type Minimap struct {
	Lines      []string          `json:"lines"`
	Highlights map[string][]Span `json:"highlights"`
}

// messageLines is how many of the last messages are shown.
const messageLines = 3

// statusLines are the lines around the map: the top line over it, the
// status line and the messages under it.
const statusLines = 2 + messageLines

// hpBarWidth is the width of the HP bar, in cells.
const hpBarWidth = 10

// Render returns the screen for a window of size: the top line, the part
// of the floor around the player that fits, then the status line and the
// messages. At the end of a run, the result comes under the top line, and
// the floor as it was at the end under it, to see what happened.
func (g *Game) Render(size Size) Screen {
	s := Screen{Highlights: map[string][]Span{}}
	s.Lines = append(s.Lines, g.topLine(s.Highlights))
	if g.Over != "" {
		g.result(&s)
	}
	g.renderMap(&s, max(size.Width/2, 1), max(size.Height-len(s.Lines)-1-messageLines, 1))
	s.Lines = append(s.Lines, g.Status())
	g.messages(&s)
	if g.ShowMap {
		s.Minimap = g.minimap()
	}
	s.Prompt = g.Prompt
	return s
}

// renderMap adds to s the part of the floor around the player that fits
// in viewW x viewH tiles.
func (g *Game) renderMap(s *Screen, viewW, viewH int) {
	l := g.Level
	x0, w := window(g.Player.X, l.W, viewW)
	y0, h := window(g.Player.Y, l.H, viewH)
	top := len(s.Lines)
	for row := range h {
		var b strings.Builder
		lnum := top + row + 1
		var last *Span
		lastGroup := ""
		for col := range w {
			p := Point{x0 + col, y0 + row}
			glyph, group := g.glyph(p)
			start := b.Len() + 1
			b.WriteString(glyph)
			if p == g.Player {
				s.Cursor = [2]int{lnum, start}
			}
			if group == "" {
				last = nil
				continue
			}
			// Join a span to the one before it of the same group.
			if last != nil && lastGroup == group && last.End == start {
				last.End = start + len(glyph)
				continue
			}
			s.Highlights[group] = append(s.Highlights[group], Span{lnum, start, start + len(glyph)})
			spans := s.Highlights[group]
			last, lastGroup = &spans[len(spans)-1], group
		}
		s.Lines = append(s.Lines, b.String())
	}
	if f := g.Player.Add(Directions[g.Facing]); f.X >= x0 && f.X < x0+w && f.Y >= y0 && f.Y < y0+h {
		col := (f.X-x0)*glyphBytes + 1
		s.Highlights[hlFacing] = []Span{{top + f.Y - y0 + 1, col, col + glyphBytes}}
	}
}

// result adds to s the result of the run.
func (g *Game) result(s *Screen) {
	title := "冒険の結果"
	if g.Cleared {
		title = "踏破！"
	}
	s.Lines = append(s.Lines, "")
	s.Highlights[hlTitle] = []Span{{len(s.Lines) + 1, 3, 3 + len(title)}}
	s.Lines = append(s.Lines,
		"  "+title,
		"",
		"  結果      "+g.Over,
		fmt.Sprintf("  到達      %dF", g.Depth),
		fmt.Sprintf("  レベル    Lv%d", g.PlayerLevel),
		fmt.Sprintf("  倒した敵  %d", g.Kills),
		fmt.Sprintf("  ターン    %d", g.Turn),
		fmt.Sprintf("  歩数      %d", g.Steps),
		"  時間      "+playTime(g.PlayTime()),
		"  シード    "+g.Seed,
		"",
		"  "+g.Summary(),
		"",
		"  新しい冒険は goda_rogue#start() で。",
		"  リプレイの保存は goda_rogue#save_replay() で。",
		"",
	)
}

// Summary is the result of the run in a line, to share: the dungeon, the
// floor and how it ended, the play time, the turns and the seed.
func (g *Game) Summary() string {
	debug := ""
	if g.Debugged {
		debug = "[デバッグ] "
	}
	return debug + fmt.Sprintf("%s %dF %s  %s  %dターン  シード %s",
		g.Dungeon.Name, g.Depth, g.Over, playTime(g.PlayTime()), g.Turn, g.Seed)
}

// messages adds the last messages to s: always messageLines lines, those
// of the actions before the last one dimmed.
func (g *Game) messages(s *Screen) {
	first := max(len(g.Log)-messageLines, 0)
	for i := range messageLines {
		n := first + i
		if n >= len(g.Log) {
			s.Lines = append(s.Lines, "")
			continue
		}
		msg := g.Log[n]
		s.Lines = append(s.Lines, msg)
		if n < g.actionStart && msg != "" {
			s.Highlights[hlOldMessage] = append(s.Highlights[hlOldMessage], Span{len(s.Lines), 1, len(msg) + 1})
		}
	}
}

// topLine returns the top line: what matters all the time (the floor, the
// level, HP with its bar). The bar is spaces with a background, so that it
// needs no characters of ambiguous width; its spans go to hl.
func (g *Game) topLine(hl map[string][]Span) string {
	head := fmt.Sprintf("%dF  Lv%d  HP %3d/%3d ", g.Depth, g.PlayerLevel, g.HP, g.MaxHP)
	filled := 0
	if g.HP > 0 {
		filled = max((g.HP*hpBarWidth+g.MaxHP-1)/g.MaxHP, 1)
	}
	group := hlHP
	if g.HP*4 <= g.MaxHP {
		group = hlHPLow
	}
	col := len(head) + 1
	if filled > 0 {
		hl[group] = append(hl[group], Span{1, col, col + filled})
	}
	if filled < hpBarWidth {
		hl[hlHPEmpty] = append(hl[hlHPEmpty], Span{1, col + filled, col + hpBarWidth})
	}
	return head + strings.Repeat(" ", hpBarWidth) +
		fmt.Sprintf("  満腹 %d%%  %dG", g.Fullness*100/g.Dungeon.Rules.Fullness, g.Gold)
}

// The characters of the mini-map. Walls are left out: the floors show the
// shapes of the rooms and corridors.
const (
	miniPlayer  = '@'
	miniFloor   = '.'
	miniUnknown = ' '
	miniStairs  = '>'
	miniItem    = '*'
	miniMonster = 'o'
)

// minimap returns the mini-map of the floor: what the player has seen, and
// the monsters it sees now.
func (g *Game) minimap() *Minimap {
	x0, y0, x1, y1 := g.seenBounds()
	m := &Minimap{Highlights: map[string][]Span{}}
	for y := y0; y <= y1; y++ {
		line := make([]byte, x1-x0+1)
		lnum := y - y0 + 1
		for x := x0; x <= x1; x++ {
			c, group := g.miniGlyph(Point{x, y})
			line[x-x0] = c
			if group == "" {
				continue
			}
			col := x - x0 + 1
			spans := m.Highlights[group]
			if n := len(spans); n > 0 && spans[n-1].Line == lnum && spans[n-1].End == col {
				spans[n-1].End++
				continue
			}
			m.Highlights[group] = append(spans, Span{lnum, col, col + 1})
		}
		m.Lines = append(m.Lines, string(line))
	}
	return m
}

// seenBounds returns the smallest rectangle around the floor the player
// has seen (walls left out, as on the mini-map), and what a bracelet
// shows.
func (g *Game) seenBounds() (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = g.Player.X, g.Player.Y, g.Player.X, g.Player.Y
	for i, seen := range g.Seen {
		if !seen || !g.Level.Tiles[i].Walkable() {
			continue
		}
		x, y := i%g.Level.W, i/g.Level.W
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	// What a bracelet shows, too.
	var sensed []Point
	for _, m := range g.Monsters {
		sensed = append(sensed, m.Pos)
	}
	for _, it := range g.Items {
		sensed = append(sensed, it.Pos)
	}
	for _, p := range sensed {
		if g.senses(p) {
			x0, y0, x1, y1 = min(x0, p.X), min(y0, p.Y), max(x1, p.X), max(y1, p.Y)
		}
	}
	return x0, y0, x1, y1
}

func (g *Game) miniGlyph(p Point) (byte, string) {
	if p == g.Player {
		return miniPlayer, hlPlayer
	}
	if !g.IsSeen(p) && !g.senses(p) {
		return miniUnknown, ""
	}
	if m := g.MonsterAt(p); m != nil && (m.Ally || g.Visible(p) || g.wears(EffectSenseEnemy)) {
		if m.Ally {
			return miniMonster, hlAlly + string(rune('0'+m.Rank))
		}
		if m.Keeper && !m.Angry {
			return miniMonster, hlKeeper + string(rune('0'+m.Rank))
		}
		return miniMonster, hlRank + string(rune('0'+m.Rank))
	}
	if _, ok := g.ItemAt(p); ok {
		return miniItem, ""
	}
	if p == g.Stairs {
		return miniStairs, hlStairs
	}
	if g.foundTrapAt(p) != nil {
		return miniTrap, hlTrap
	}
	if g.Level.At(p).Walkable() {
		if g.IsSafe(p) {
			return miniFloor, hlSafe
		}
		return miniFloor, hlFloor
	}
	return miniUnknown, ""
}

// window returns the first of n tiles of size total to show around pos:
// centered on it, and kept in the floor.
func window(pos, total, n int) (start, length int) {
	if n >= total {
		return 0, total
	}
	start = min(max(pos-n/2, 0), total-n)
	return start, n
}

// glyph returns what p shows, and its highlight group ("" for none).
func (g *Game) glyph(p Point) (string, string) {
	if p == g.Player {
		return glyphPlayer, hlPlayer
	}
	if !g.IsSeen(p) && !g.senses(p) {
		return glyphUnknown, ""
	}
	if m := g.MonsterAt(p); m != nil && (m.Ally || g.Visible(p) || g.wears(EffectSenseEnemy)) {
		if m.Ally {
			return m.Glyph, hlAlly + string(rune('0'+m.Rank))
		}
		if m.Keeper && !m.Angry {
			return m.Glyph, hlKeeper + string(rune('0'+m.Rank))
		}
		return m.Glyph, hlRank + string(rune('0'+m.Rank))
	}
	if it, ok := g.ItemAt(p); ok {
		return it.Glyph, ""
	}
	if p == g.Stairs {
		return glyphStairs, hlStairs
	}
	if g.foundTrapAt(p) != nil {
		return glyphTrap, hlTrap
	}
	if g.Level.At(p).Walkable() {
		if g.IsSafe(p) {
			return glyphFloor, hlSafe
		}
		return glyphFloor, hlFloor
	}
	return glyphWall, hlWall
}

// senses says whether a bracelet shows what is at p on a tile the player
// has not seen: a monster, or an item.
func (g *Game) senses(p Point) bool {
	if g.wears(EffectSenseEnemy) && g.MonsterAt(p) != nil {
		return true
	}
	if g.wears(EffectSenseItem) {
		_, ok := g.ItemAt(p)
		return ok
	}
	return false
}
