package rogue

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// State files (Shougo, 2026-11-27, as the dojo of Shiren 6): a scene of
// the debug mode written as text, to set up at once (debug state, or
// goda_rogue#load_state()) and to share. The current scene is written
// the same way (DumpState, goda_rogue#dump_state()). It does not depend
// on a seed: it stays good when the data of the dungeon change.
//
//	# a comment
//	ダンジョン	cave
//	階	12
//	主人公	5,2
//	レベル	10
//	HP	80/80
//	ちから	8/8
//	満腹度	100/100
//	お金	1000
//	識別	yakusou otogiri	(or すべて)
//	持ち物	doutanuki +3 印=会,錆 装備
//	道具	7,3 otogiri 呪い
//	敵	5,3 unagi2 眠り HP=20
//	罠	7,4 onigiriwana 見える
//	地形
//	##########
//	#....~~..#
//	#....>...++++
//	##########
//
// Coordinates are x,y on the floor, from 0. The terrain (地形, to the end
// or a blank line) is one character a tile: # rock, . the floor of a
// room, + a corridor, ~ water, > the stairs (a room's floor); the rooms
// are the rectangles around the floor and water that touch. With it, the
// floor is that, with only what the file lists; without it, the floor 階
// is made as usual (by the seed), and what the file lists is added.
//
// The attributes of an item: +N or -N (upgrade value), 印=会,錆 (seals),
// 枠+N (more slots), 装備, 祝福, 呪い, 識別 (its upgrade and charges
// known), 回数=N or 容量=N (charges), 数=N (arrows), 金額=N (money),
// 中身=id;id (in a pot), 売り物, 貼りつき, ミミック=id (a mimic hidden as
// it). Of a monster: 眠り, 仲間, HP=N. Of a trap: 見える.

// The terrain characters of a state file.
const (
	stateRock     = '#'
	stateRoom     = '.'
	stateCorridor = '+'
	stateWater    = '~'
	stateStairs   = '>'
)

// debugState is the debug command that sets the scene of a state file.
const debugState = "state"

// stateLine is a line of a state file: its key and its fields.
type stateLine struct {
	n      int
	key    string
	fields []string
}

// parseState reads a state file: its lines, and its terrain.
func parseState(text string) (lines []stateLine, terrain []string) {
	in := false
	for i, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if in {
			if strings.TrimSpace(l) == "" {
				in = false
				continue
			}
			terrain = append(terrain, l)
			continue
		}
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Split(l, "\t")
		if f[0] == "地形" {
			in = true
			continue
		}
		var fields []string
		for _, s := range f[1:] {
			fields = append(fields, strings.Fields(s)...)
		}
		lines = append(lines, stateLine{n: i + 1, key: f[0], fields: fields})
	}
	return lines, terrain
}

// StateDungeon returns the dungeon a state file names (ダンジョン), or "".
func StateDungeon(text string) string {
	lines, _ := parseState(text)
	for _, l := range lines {
		if l.key == "ダンジョン" && len(l.fields) > 0 {
			return l.fields[0]
		}
	}
	return ""
}

// loadState sets the scene of the state file text.
func (g *Game) loadState(text string) error {
	lines, terrain := parseState(text)
	errf := func(l stateLine, format string, a ...any) error {
		return fmt.Errorf("state %d: %s: %s", l.n, l.key, fmt.Sprintf(format, a...))
	}
	floor := 0
	for _, l := range lines {
		switch l.key {
		case "ダンジョン":
			if len(l.fields) == 0 || l.fields[0] != g.Dungeon.ID {
				return errf(l, "this is the dungeon %q", g.Dungeon.ID)
			}
		case "階":
			n, err := stateInt(l, 0)
			if err != nil || n < 1 || n > g.Dungeon.Floors {
				return errf(l, "a floor of 1 to %d expected", g.Dungeon.Floors)
			}
			floor = n
		}
	}
	switch {
	case len(terrain) > 0:
		if floor == 0 {
			floor = max(g.Depth, 1)
		}
		l, stairs, err := stateTerrain(terrain)
		if err != nil {
			return err
		}
		g.Depth = floor
		g.newLevel(g.floorRNG(floor))
		g.Level, g.Stairs = l, stairs
		g.Seen = make([]bool, len(l.Tiles))
		g.Safe = make([]bool, len(l.Tiles))
		g.Player = Point{-1, -1}
	case floor != 0:
		g.Depth = floor - 1
		g.descend()
	}
	placed := false
	inventory := false
	for _, l := range lines {
		switch l.key {
		case "ダンジョン", "階":
		case "主人公":
			p, err := statePoint(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			if !g.Level.At(p).Open() {
				return errf(l, "%v is no floor", p)
			}
			g.Player, placed = p, true
		case "レベル":
			n, err := stateInt(l, 0)
			if err != nil || n < 1 {
				return errf(l, "a level expected")
			}
			g.PlayerLevel = n
		case "経験値":
			n, err := stateInt(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.EXP = n
		case "HP":
			a, b, err := stateRatio(l)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.HP, g.MaxHP = a, b
		case "ちから":
			a, b, err := stateRatio(l)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.Strength, g.MaxStrength = a, b
		case "満腹度":
			a, b, err := stateRatio(l)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.Fullness, g.MaxFullness = a, b
		case "お金":
			n, err := stateInt(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.Gold = n
		case "識別":
			g.identified = map[string]bool{}
			for _, id := range l.fields {
				if id == "すべて" {
					for _, d := range g.Dungeon.Items {
						g.identified[d.ID] = true
					}
					continue
				}
				if g.Dungeon.Item(id) == nil {
					return errf(l, "no item %q", id)
				}
				g.identified[id] = true
			}
		case "持ち物":
			if !inventory {
				g.Inventory, inventory = nil, true
			}
			it, err := g.stateItem(l.fields, Point{})
			if err != nil {
				return errf(l, "%v", err)
			}
			g.Inventory = append(g.Inventory, it)
		case "道具":
			p, err := statePoint(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			it, err := g.stateItem(l.fields[1:], p)
			if err != nil {
				return errf(l, "%v", err)
			}
			g.Items = append(g.Items, it)
		case "敵":
			p, err := statePoint(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			if len(l.fields) < 2 || g.Dungeon.Monster(l.fields[1]) == nil {
				return errf(l, "a monster expected")
			}
			m := newMonster(g.Dungeon.Monster(l.fields[1]))
			m.Pos = p
			for _, a := range l.fields[2:] {
				k, v, _ := strings.Cut(a, "=")
				switch k {
				case "眠り":
					m.Dozing = true
				case "仲間":
					m.Ally = true
				case "HP":
					n, err := strconv.Atoi(v)
					if err != nil || n < 1 {
						return errf(l, "HP=%q", v)
					}
					m.HP = n
					m.MaxHP = max(m.MaxHP, n)
				default:
					return errf(l, "unknown attribute %q", a)
				}
			}
			g.Monsters = append(g.Monsters, m)
		case "罠":
			p, err := statePoint(l, 0)
			if err != nil {
				return errf(l, "%v", err)
			}
			if len(l.fields) < 2 || g.Dungeon.Trap(l.fields[1]) == nil {
				return errf(l, "a trap expected")
			}
			t := &Trap{Def: g.Dungeon.Trap(l.fields[1]), Pos: p}
			for _, a := range l.fields[2:] {
				if a != "見える" {
					return errf(l, "unknown attribute %q", a)
				}
				t.Found = true
			}
			g.Traps = slices.DeleteFunc(g.Traps, func(x *Trap) bool { return x.Pos == p })
			g.Traps = append(g.Traps, t)
		default:
			return errf(l, "unknown line")
		}
	}
	if !placed && !g.Level.At(g.Player).Open() {
		// (A terrain with no 主人公: on the first floor of a room.)
		p, ok := g.freeFloor(Point{g.Level.W / 2, g.Level.H / 2})
		if !ok {
			return fmt.Errorf("state: no floor for the player")
		}
		g.Player = p
	}
	g.sortInventory()
	g.look()
	g.say("状態を読み込んだ。")
	return nil
}

// stateInt is the field i of l as a number.
func stateInt(l stateLine, i int) (int, error) {
	if len(l.fields) <= i {
		return 0, fmt.Errorf("a number expected")
	}
	return strconv.Atoi(l.fields[i])
}

// stateRatio is the field "a/b" of l.
func stateRatio(l stateLine) (int, int, error) {
	if len(l.fields) == 0 {
		return 0, 0, fmt.Errorf("a/b expected")
	}
	as, bs, ok := strings.Cut(l.fields[0], "/")
	a, err1 := strconv.Atoi(as)
	b, err2 := strconv.Atoi(bs)
	if !ok || err1 != nil || err2 != nil || b < 1 {
		return 0, 0, fmt.Errorf("a/b expected: %q", l.fields[0])
	}
	return a, b, nil
}

// statePoint is the field i of l as x,y.
func statePoint(l stateLine, i int) (Point, error) {
	if len(l.fields) <= i {
		return Point{}, fmt.Errorf("x,y expected")
	}
	xs, ys, ok := strings.Cut(l.fields[i], ",")
	x, err1 := strconv.Atoi(xs)
	y, err2 := strconv.Atoi(ys)
	if !ok || err1 != nil || err2 != nil || x < 0 || y < 0 || x >= MapWidth || y >= MapHeight {
		return Point{}, fmt.Errorf("x,y on the floor expected: %q", l.fields[i])
	}
	return Point{x, y}, nil
}

// stateItem is the item of the fields (its ID, then its attributes) at p.
func (g *Game) stateItem(fields []string, p Point) (*Item, error) {
	if len(fields) == 0 || g.Dungeon.Item(fields[0]) == nil {
		return nil, fmt.Errorf("an item expected")
	}
	def := g.Dungeon.Item(fields[0])
	it := g.newItemOf(g.rng, def, p)
	it.Cursed, it.Blessed, it.Upgrade, it.Mimic = false, false, 0, nil
	for _, a := range fields[1:] {
		k, v, _ := strings.Cut(a, "=")
		num := func() (int, error) { return strconv.Atoi(v) }
		var err error
		switch {
		case strings.HasPrefix(a, "+") || strings.HasPrefix(a, "-"):
			it.Upgrade, err = strconv.Atoi(a)
		case strings.HasPrefix(a, "枠+"):
			it.MoreSlots, err = strconv.Atoi(strings.TrimPrefix(a, "枠+"))
		case k == "印":
			it.Seals = strings.Split(v, ",")
		case k == "装備":
			it.Equipped = true
		case k == "祝福":
			it.Blessed = true
		case k == "呪い":
			it.Cursed, it.CurseTold = true, true
		case k == "識別":
			it.Known = true
		case k == "回数" || k == "容量":
			it.Charges, err = num()
		case k == "数":
			it.Count, err = num()
		case k == "金額":
			it.Amount, err = num()
		case k == "売り物":
			it.ForSale = true
		case k == "貼りつき":
			it.Stuck = true
		case k == "ミミック":
			if it.Mimic = g.Dungeon.Monster(v); it.Mimic == nil {
				err = fmt.Errorf("no monster %q", v)
			}
		case k == "中身":
			for _, id := range strings.Split(v, ";") {
				in, err := g.stateItem([]string{id}, Point{})
				if err != nil {
					return nil, err
				}
				it.Contents = append(it.Contents, in)
			}
		default:
			err = fmt.Errorf("unknown attribute %q", a)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a, err)
		}
	}
	if it.Blessed && it.Cursed {
		return nil, fmt.Errorf("blessed and cursed")
	}
	return it, nil
}

// stateTerrain makes the floor of the terrain of a state file, and finds
// its stairs.
func stateTerrain(rows []string) (*Level, Point, error) {
	l := &Level{W: MapWidth, H: MapHeight, Tiles: make([]Tile, MapWidth*MapHeight)}
	stairs := Point{-1, -1}
	if len(rows) > MapHeight {
		return nil, stairs, fmt.Errorf("state: the terrain has %d rows, %d at most", len(rows), MapHeight)
	}
	for y, row := range rows {
		if len(row) > MapWidth {
			return nil, stairs, fmt.Errorf("state: the row %d of the terrain is wider than %d", y, MapWidth)
		}
		for x, c := range []byte(row) {
			p := Point{x, y}
			switch c {
			case stateRock, ' ':
			case stateRoom:
				l.set(p, TileRoom)
			case stateCorridor:
				l.set(p, TileCorridor)
			case stateWater:
				l.set(p, TileWater)
			case stateStairs:
				l.set(p, TileRoom)
				stairs = p
			default:
				return nil, stairs, fmt.Errorf("state: %q in the terrain at %d,%d", c, x, y)
			}
		}
	}
	// The rooms: the rectangles around the floor and water that touch.
	seen := make([]bool, len(l.Tiles))
	for i, t := range l.Tiles {
		if seen[i] || t != TileRoom && t != TileWater {
			continue
		}
		r := Room{X: i % l.W, Y: i / l.W, W: 1, H: 1}
		x1, y1 := r.X, r.Y
		stack := []int{i}
		seen[i] = true
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			p := Point{j % l.W, j / l.W}
			r.X, r.Y, x1, y1 = min(r.X, p.X), min(r.Y, p.Y), max(x1, p.X), max(y1, p.Y)
			for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				q := p.Add(d)
				if q.X < 0 || q.Y < 0 || q.X >= l.W || q.Y >= l.H {
					continue
				}
				k := q.Y*l.W + q.X
				if !seen[k] && (l.Tiles[k] == TileRoom || l.Tiles[k] == TileWater) {
					seen[k] = true
					stack = append(stack, k)
				}
			}
		}
		r.W, r.H = x1-r.X+1, y1-r.Y+1
		l.Rooms = append(l.Rooms, r)
	}
	if len(l.Rooms) == 0 {
		return nil, stairs, fmt.Errorf("state: the terrain has no room")
	}
	return l, stairs, nil
}

// DumpState writes the scene as a state file: the terrain of the whole
// floor, seen or not, and all that is on it (not the shop itself, nor the
// states of the monsters: their kind, place, sleep, side and HP).
func (g *Game) DumpState() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# goda-rogue state: %s, %dF, turn %d\n", g.Dungeon.Name, g.Depth, g.Turn)
	fmt.Fprintf(&b, "ダンジョン\t%s\n階\t%d\n", g.Dungeon.ID, g.Depth)
	fmt.Fprintf(&b, "主人公\t%d,%d\nレベル\t%d\n経験値\t%d\n", g.Player.X, g.Player.Y, g.PlayerLevel, g.EXP)
	fmt.Fprintf(&b, "HP\t%d/%d\nちから\t%d/%d\n満腹度\t%d/%d\nお金\t%d\n", g.HP, g.MaxHP, g.Strength, g.MaxStrength, g.Fullness, g.MaxFullness, g.Gold)
	var ids []string
	for _, d := range g.Dungeon.Items {
		if g.identified[d.ID] {
			ids = append(ids, d.ID)
		}
	}
	fmt.Fprintf(&b, "識別\t%s\n", strings.Join(ids, " "))
	for _, it := range g.Inventory {
		fmt.Fprintf(&b, "持ち物\t%s\n", stateItemText(it))
	}
	for _, it := range g.Items {
		fmt.Fprintf(&b, "道具\t%d,%d %s\n", it.Pos.X, it.Pos.Y, stateItemText(it))
	}
	for _, m := range g.Monsters {
		if m.Keeper {
			continue
		}
		s := fmt.Sprintf("%d,%d %s HP=%d", m.Pos.X, m.Pos.Y, m.Def.ID, m.HP)
		if m.Dozing {
			s += " 眠り"
		}
		if m.Ally {
			s += " 仲間"
		}
		fmt.Fprintf(&b, "敵\t%s\n", s)
	}
	for _, t := range g.Traps {
		s := fmt.Sprintf("%d,%d %s", t.Pos.X, t.Pos.Y, t.Def.ID)
		if t.Found {
			s += " 見える"
		}
		fmt.Fprintf(&b, "罠\t%s\n", s)
	}
	b.WriteString("地形\n")
	for y := range g.Level.H {
		row := make([]byte, g.Level.W)
		for x := range g.Level.W {
			p := Point{x, y}
			switch g.Level.At(p) {
			case TileRoom:
				row[x] = stateRoom
			case TileCorridor:
				row[x] = stateCorridor
			case TileWater:
				row[x] = stateWater
			default:
				row[x] = stateRock
			}
			if p == g.Stairs {
				row[x] = stateStairs
			}
		}
		// (Whole rows: a blank line would end the terrain.)
		b.WriteString(string(row) + "\n")
	}
	return b.String()
}

// stateItemText is the item it in a state file: its ID and attributes.
func stateItemText(it *Item) string {
	s := []string{it.Def.ID}
	if it.Upgrade != 0 {
		s = append(s, fmt.Sprintf("%+d", it.Upgrade))
	}
	if len(it.Seals) > 0 {
		s = append(s, "印="+strings.Join(it.Seals, ","))
	}
	if it.MoreSlots > 0 {
		s = append(s, fmt.Sprintf("枠+%d", it.MoreSlots))
	}
	for _, f := range []struct {
		on   bool
		attr string
	}{{it.Equipped, "装備"}, {it.Blessed, "祝福"}, {it.Cursed, "呪い"}, {it.Known, "識別"}, {it.ForSale, "売り物"}, {it.Stuck, "貼りつき"}} {
		if f.on {
			s = append(s, f.attr)
		}
	}
	if it.Charges != 0 {
		s = append(s, fmt.Sprintf("回数=%d", it.Charges))
	}
	if it.Count != 0 {
		s = append(s, fmt.Sprintf("数=%d", it.Count))
	}
	if it.Amount != 0 {
		s = append(s, fmt.Sprintf("金額=%d", it.Amount))
	}
	if it.Mimic != nil {
		s = append(s, "ミミック="+it.Mimic.ID)
	}
	if len(it.Contents) > 0 {
		var ids []string
		for _, in := range it.Contents {
			ids = append(ids, in.Def.ID)
		}
		s = append(s, "中身="+strings.Join(ids, ";"))
	}
	return strings.Join(s, " ")
}
