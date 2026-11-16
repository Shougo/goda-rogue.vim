package rogue

import (
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
	hlStairs = "GodaRogueStairs"
	hlRank   = "GodaRogueRank"
	hlAlly   = "GodaRogueAlly"
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
type Screen struct {
	Lines      []string          `json:"lines"`
	Highlights map[string][]Span `json:"highlights"`
	Cursor     [2]int            `json:"cursor"`
}

// statusLines are the lines under the map: the status and the message.
const statusLines = 2

// Render returns the screen for a window of size: the part of the floor
// around the player that fits, then the status line and the message.
func (g *Game) Render(size Size) Screen {
	l := g.Level
	viewW := max(size.Width/2, 1)
	viewH := max(size.Height-statusLines, 1)
	x0, w := window(g.Player.X, l.W, viewW)
	y0, h := window(g.Player.Y, l.H, viewH)

	s := Screen{Highlights: map[string][]Span{}}
	for row := range h {
		var b strings.Builder
		lnum := row + 1
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
	s.Lines = append(s.Lines, g.Status(), g.Message)
	return s
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
	if !g.IsSeen(p) {
		return glyphUnknown, ""
	}
	if m := g.MonsterAt(p); m != nil && (m.Ally || g.Visible(p)) {
		if m.Ally {
			return m.Glyph, hlAlly + string(rune('0'+m.Rank))
		}
		return m.Glyph, hlRank + string(rune('0'+m.Rank))
	}
	if it, ok := g.ItemAt(p); ok {
		return it.Glyph, ""
	}
	if p == g.Stairs {
		return glyphStairs, hlStairs
	}
	if g.Level.At(p).Walkable() {
		return glyphFloor, hlFloor
	}
	return glyphWall, hlWall
}
