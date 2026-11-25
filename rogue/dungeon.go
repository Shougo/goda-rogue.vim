package rogue

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path"
	"slices"
	"strconv"
	"strings"
)

// A dungeon is a directory of tab-separated text files: what is in the
// dungeon, and its rules. Lines starting with "#" are comments.
//
//	header.txt         name, description, floors (key<TAB>value)
//	                   and, for a built-in dungeon, its base (元): the
//	                   dungeon whose files it uses for those it lacks
//	rules.txt          the rules of the run (key<TAB>value)
//	monsters.txt       the monsters (a table)
//	monster_table.txt  which monsters appear on which floors (a table)
//	items.txt          the items (a table; 種類 is their category, 強さ
//	                   the strength of a weapon or a shield, 効果 what
//	                   an item does, ひび how easily a bracelet cracks, 回数
//	                   how many times a staff can be waved, how many
//	                   arrows there are, how many items a pot holds,
//	                   印 the seal a weapon or a shield is born with,
//	                   印数 how many seals it can have)
//	category_tables.txt  the item tables: the weights of the kinds of
//	                   items, by floor (a table; 表 is the table)
//	item_tables.txt    the item tables: the weights of the items among
//	                   those of their kind, by floor (a table)
//	guaranteed_items.txt  items that surely appear on one floor of a range
//	                   (a table, optional)
//	seals.txt          the seals of weapons and shields (a table)
//	fake_names.txt     the names of unidentified items, by kind (a table)
//	shops.txt          the shops (a table)
//	shop_table.txt     which shops appear on which floors (a table)
//	traps.txt          the traps (a table)
//	trap_table.txt     which traps appear on which floors (a table)
//	floors.txt         how many monsters, items and traps a floor has, the
//	                   item table of its items, its brightness, the
//	                   chances of a big room, a maze and a monster house
//	                   (a table)
//	growth.txt         the growth tables (a table)
//
// In key<TAB>value files, a later line overrides an earlier one. A table
// starts with a line of column names; then every line is a row. "開始" and
// "終了" are the first and last floors a row is for.

//go:embed dungeons
var builtinFS embed.FS

// Dungeon is a dungeon, loaded and checked.
type Dungeon struct {
	ID          string
	Name        string
	Description string
	Floors      int
	// Base is the dungeon it is based on (header.txt 元; "" for none):
	// the files it does not have are the base's.
	Base        string
	Rules       Rules
	Monsters    []*MonsterDef
	MonsterRows []FloorRow
	Items       []*ItemDef
	// ItemTables are the item tables, by name (itemtable.go).
	ItemTables map[string]*ItemTable
	// Guaranteed are the items that surely appear on one floor of
	// [First, Last] (Weight unused).
	Guaranteed []FloorRow
	Traps      []*TrapDef
	TrapRows   []FloorRow
	Seals      []*SealDef
	Shops      []*ShopDef
	ShopRows   []FloorRow
	// FakeNames are the names of unidentified items, by category; items
	// of a category without them are identified from the start.
	FakeNames map[Category][]string
	FloorRows []FloorCounts
	Growth    map[string][]GrowthRow
	// Hash is a hash of all the files: the same seed makes the same
	// floors only with the same definition.
	Hash string
}

// defaultGoldThrow is お金投げ without it in rules.txt: a tenth of the
// amount (Shiren 6 has a fifth, for its tougher monsters).
const defaultGoldThrow = 10

// Rules are the rules of a run.
type Rules struct {
	PlayerHP       int
	Strength       int
	RegenTurns     int
	SpawnTurns     int
	MaxMonsters    int
	WindTurns      [3]int
	DefenseK       int
	PlayerHitRate  int
	MonsterHitRate int
	// AllyHitRate is the hit rate of the attacks of allies (the monsters
	// attack an ally at MonsterHitRate).
	AllyHitRate int
	// PlayerThrowRate and MonsterThrowRate are the hit rates of what the
	// player and the monsters throw and shoot.
	PlayerThrowRate, MonsterThrowRate int
	// Growth is the growth table of the player and of monsters without
	// one of their own.
	Growth string
	// FirstAlly is the monster that comes along from the start ("" for
	// none).
	FirstAlly string
	// MaxAllies is how many allies the player has at most (rules.txt
	// 仲間の上限, 3 without it).
	MaxAllies int
	// CurseGear and CurseBracelet are how rarely a weapon or a shield,
	// and a bracelet, found in the dungeon are cursed: one in so many
	// (rules.txt 武具の呪い and 腕輪の呪い; 0, as without them, for never;
	// curse.go).
	CurseGear, CurseBracelet int
	// CurseItem and CurseFood are the same for food (食料の呪い) and for
	// the other items that are used (道具の呪い: grasses, scrolls,
	// staves, pots).
	CurseItem, CurseFood int
	// Bless is how rarely an item that can be cursed is blessed
	// instead: one in so many (rules.txt 祝福; 0 for never; bless.go).
	Bless int
	// Lucky is how rarely a scroll that luck makes better does more:
	// one in so many (rules.txt ラッキー; 0 for never unless blessed;
	// lucky.go).
	Lucky int
	// OverAttack and OverDefense are the percents of its attack and of
	// its defense that a monster of rank 4 gets each time it ranks up
	// beyond (rules.txt 超過攻撃 and 超過防御, optional; over.go).
	OverAttack, OverDefense int
	// StrongerBy is how many ranks above the floor's the monster of the
	// second wind is.
	StrongerBy int
	// Capacity is how many items the player can carry.
	Capacity int
	// CrackStages is how many cracks a bracelet can have.
	CrackStages int
	// Wear is how easily bracelets crack, a percent of their Crack.
	Wear int
	// Debug says whether runs in the dungeon may use the debug mode
	// (debug.go): for the makers of edit dungeons.
	Debug bool
	// FoundTrapsSafe says whether a trap the player has found never goes
	// off.
	FoundTrapsSafe bool
	// Fullness is the max fullness; HungerTurns how many turns make the
	// player 1 hungrier.
	Fullness, HungerTurns int
	// GoldThrow divides the amount of money thrown into its damage
	// (rules.txt お金投げ, optional: defaultGoldThrow).
	GoldThrow int
	// DropRate is the chance, a percent, that a monster the player kills
	// drops an item of the floor's item table, unless monsters.txt says
	// otherwise (rules.txt 落とし物, optional: 0).
	DropRate int
}

// MonsterDef is a kind of monster. Kinds of the same glyph are one family,
// one kind a rank.
type MonsterDef struct {
	ID      string
	Glyph   string
	Name    string
	Rank    int
	HP      int
	Attack  int
	Defense int
	EXP     int
	Speed   Speed
	Growth  string
	// Kind is "" for monsters of the floors, KindKeeper or KindGuard.
	Kind string
	// Abilities are its special abilities (monsters.txt 能力).
	Abilities []Ability
	// Asleep is the share of them asleep on a new floor, a percent; -1
	// for the default (a half).
	Asleep int
	// Drop is what it drops when the player kills it (monsters.txt
	// 落とし物); nil for the dungeon's rule.
	Drop *Drop
	// Description is what the maker of the dungeon says of it (monsters.txt
	// 説明), shown when the player looks at it; "" for none.
	Description string
}

// Drop is what a kind of monster drops when killed, and how often.
type Drop struct {
	// What is DropItem, DropMoney, DropNone, a category of items (an
	// item of it from the floor's item table) or the ID of an item.
	What string
	// Rate is the chance, a percent.
	Rate int
	// file and line are where it is read, for the check of the item.
	file string
	line int
}

// What a monster drops (monsters.txt 落とし物).
const (
	// DropItem is an item of the floor's item table.
	DropItem = "道具"
	// DropMoney is money, as much as on the floor.
	DropMoney = "お金"
	// DropNone is nothing.
	DropNone = "なし"
)

// The kinds of monsters that are not of the floors (monsters.txt 種別).
const (
	KindKeeper = "店主"
	KindGuard  = "番犬"
)

// ItemDef is a kind of item.
type ItemDef struct {
	ID       string
	Glyph    string
	Name     string
	Category Category
	// Strength is the attack of a weapon, the defense of a shield, how
	// much the effect of another item does.
	Strength int
	// Effect is what the item does ("" for nothing): a bracelet while it is
	// worn, another item when it is used.
	Effect Effect
	// Crack is how easily a bracelet cracks, a percent (0 for never).
	Crack int
	// Price is the price in shops (of one arrow).
	Price int
	// Seal is the seal a weapon or a shield is born with ("" for none),
	// Slots how many seals it can have.
	Seal  string
	Slots int
	// Charges are how many times a staff can be waved, how many arrows
	// there are, how many items a pot holds: from, to.
	Charges [2]int
}

// Effect is what an item does: a bracelet while it is worn, another item
// when it is used.
type Effect string

// The effects.
const (
	EffectStrength   Effect = "ちから"
	EffectRegen      Effect = "回復"
	EffectSenseEnemy Effect = "気配察知"
	EffectSenseItem  Effect = "道具感知"
	EffectNoRust     Effect = "サビよけ"
	EffectNoTheft    Effect = "盗みよけ"
	EffectNoSleep    Effect = "睡眠よけ"
	EffectNoConfuse  Effect = "混乱よけ"
	EffectNoCurse    Effect = "呪いよけ"
	EffectNoBlind    Effect = "目つぶしよけ"
	EffectNoSeal     Effect = "口封じよけ"
	EffectNoWeak     Effect = "弱りよけ"
	// EffectHungry: hungry twice as fast, the blows of the player do
	// its strength in % more.
	EffectHungry Effect = "ハラヘリ"
	// EffectDrops: the monsters killed drop an item its strength in %
	// more often; the player lets one of its own fall now and then
	// (leakChance).
	EffectDrops     Effect = "垂れ流し"
	EffectHeal      Effect = "HP回復"
	EffectLight     Effect = "あかり"
	EffectIdentify  Effect = "識別"
	EffectBlowAway  Effect = "吹き飛ばし"
	EffectRecruit   Effect = "仲間"
	EffectEyeDrops  Effect = "目薬"
	EffectMaxHP     Effect = "最大HP"
	EffectUpWeapon  Effect = "武器強化"
	EffectUpShield  Effect = "盾強化"
	EffectSwap      Effect = "場所替え"
	EffectFood      Effect = "満腹"
	EffectRotten    Effect = "腐った"
	EffectKeep      Effect = "保存"
	EffectSynthesis Effect = "合成"
	EffectPlain     Effect = "普通"
	EffectHealPot   Effect = "全回復"
	EffectUncurse   Effect = "おはらい"
	EffectMoreSlots Effect = "印増大"
	EffectEraseSeal Effect = "印消し"
)

// effects are the effects the items of each category can have.
var effects = map[Category][]Effect{
	CategoryBracelet: {EffectStrength, EffectRegen, EffectSenseEnemy, EffectSenseItem, EffectNoRust, EffectNoTheft, EffectNoSleep, EffectNoConfuse,
		EffectNoCurse, EffectNoBlind, EffectNoSeal, EffectNoWeak, EffectHungry, EffectDrops},
	CategoryGrass:  {EffectHeal, EffectStrength, EffectEyeDrops, EffectMaxHP},
	CategoryScroll: {EffectLight, EffectIdentify, EffectUpWeapon, EffectUpShield, EffectUncurse, EffectMoreSlots, EffectEraseSeal},
	CategoryStaff:  {EffectBlowAway, EffectRecruit, EffectSwap},
	CategoryFood:   {EffectFood, EffectRotten},
	CategoryPot:    {EffectKeep, EffectSynthesis, EffectHealPot, EffectPlain},
}

// FloorRow is a row of an appearance table: id appears on the floors
// [First, Last], with a weight.
type FloorRow struct {
	First, Last int
	ID          string
	Weight      int
}

// FloorCounts are how many monsters and items the floors [First, Last]
// have, between the minimum and the maximum.
type FloorCounts struct {
	// ItemTable is the item table of the items of the floors.
	ItemTable              string
	First, Last            int
	MinMonster, MaxMonster int
	MinItem, MaxItem       int
	MinTrap, MaxTrap       int
	// MinHouseTrap and MaxHouseTrap are the traps a monster house adds
	// in its room (ハウスの罠, optional: none).
	MinHouseTrap, MaxHouseTrap int
	// BigRoom and Maze are the chances (percent) of a floor of one big
	// room, of a maze; House the chance of a monster house.
	BigRoom, Maze, House int
	// Shop is the chance (percent) of a shop.
	Shop       int
	Brightness Brightness
}

// Brightness is how much of a floor the player sees.
type Brightness string

// The brightness of floors.
const (
	// BrightnessNormal: the room the player is in, and around it.
	BrightnessNormal Brightness = "通常"
	// BrightnessBright: all of the floor, as if lit by a scroll.
	BrightnessBright Brightness = "明るい"
	// BrightnessDark: only around the player.
	BrightnessDark Brightness = "暗い"
)

// GrowthRow is a level of a growth table: the total EXP for it, and what
// it adds.
type GrowthRow struct {
	Level, EXP          int
	HP, Attack, Defense int
}

// DungeonError is a mistake in a dungeon's files, at a line (0 for the
// file as a whole).
type DungeonError struct {
	File string
	Line int
	Msg  string
}

func (e *DungeonError) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.File, e.Msg)
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

// BuiltinDungeons returns the IDs of the dungeons in the binary.
func BuiltinDungeons() []string {
	entries, _ := fs.ReadDir(builtinFS, "dungeons")
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids
}

// LoadBuiltin loads the dungeon id of the binary. A dungeon with a base
// (header.txt 元) reads the files it does not have from the base's.
func LoadBuiltin(id string) (*Dungeon, error) {
	fsys, err := dungeonFiles(builtinFS, id, nil)
	if err != nil {
		return nil, err
	}
	return LoadDungeon(fsys, id)
}

// dungeonFiles returns the files of the dungeon id of root (a directory
// "dungeons" of dungeons), over those of its base, if it has one (seen
// are the dungeons on the way: a base may not lead back to one of them).
func dungeonFiles(root fs.FS, id string, seen []string) (fs.FS, error) {
	sub, err := fs.Sub(root, path.Join("dungeons", id))
	if err != nil {
		return nil, err
	}
	b, err := fs.ReadFile(sub, "header.txt")
	if err != nil {
		return nil, &DungeonError{"header.txt", 0, fmt.Sprintf("dungeon %q: %v", id, err)}
	}
	for _, ln := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		key, base, ok := strings.Cut(ln, "\t")
		if !ok || strings.TrimSpace(key) != headerBase {
			continue
		}
		base = strings.TrimSpace(base)
		if base == id || slices.Contains(seen, base) {
			return nil, &DungeonError{"header.txt", 0, fmt.Sprintf("%s: %q is based on itself", headerBase, base)}
		}
		under, err := dungeonFiles(root, base, append(seen, id))
		if err != nil {
			return nil, err
		}
		return overlayFS{sub, under}, nil
	}
	return sub, nil
}

// headerBase is the key of header.txt for the dungeon a dungeon is based
// on.
const headerBase = "元"

// overlayFS is the files of top, and under them those of base.
type overlayFS struct{ top, base fs.FS }

func (o overlayFS) Open(name string) (fs.File, error) {
	f, err := o.top.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return o.base.Open(name)
	}
	return f, err
}

// LoadDungeon loads the dungeon id from the files of fsys, and checks it.
// The error lists every mistake found (errors.Join of *DungeonError).
func LoadDungeon(fsys fs.FS, id string) (*Dungeon, error) {
	l := &loader{fsys: fsys, d: &Dungeon{ID: id, Growth: map[string][]GrowthRow{}}}
	l.load()
	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return l.d, nil
}

type loader struct {
	fsys fs.FS
	d    *Dungeon
	errs []error
	h    []byte
	// rules are the lines of rules.txt by key, for the checks of the
	// files read after it.
	rules map[string]line
}

func (l *loader) errorf(file string, line int, format string, args ...any) {
	l.errs = append(l.errs, &DungeonError{file, line, fmt.Sprintf(format, args...)})
}

// line is a line of a file, split at tabs.
type line struct {
	n      int
	fields []string
}

// read returns the lines of file that are not empty nor comments.
func (l *loader) read(file string) []line {
	b, err := fs.ReadFile(l.fsys, file)
	if err != nil {
		l.errorf(file, 0, "%v", err)
		return nil
	}
	l.h = append(l.h, file...)
	l.h = append(l.h, 0)
	l.h = append(l.h, b...)
	var lines []line
	for i, s := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(s) == "" || strings.HasPrefix(s, "#") {
			continue
		}
		fields := strings.Split(s, "\t")
		for j := range fields {
			fields[j] = strings.TrimSpace(fields[j])
		}
		lines = append(lines, line{i + 1, fields})
	}
	return lines
}

// keys reads a key<TAB>value file, the later lines overriding.
func (l *loader) keys(file string, known []string) map[string]line {
	m := map[string]line{}
	for _, ln := range l.read(file) {
		if len(ln.fields) != 2 {
			l.errorf(file, ln.n, "want key<TAB>value")
			continue
		}
		if !slices.Contains(known, ln.fields[0]) {
			l.errorf(file, ln.n, "unknown key %q", ln.fields[0])
			continue
		}
		m[ln.fields[0]] = ln
	}
	return m
}

// row is a row of a table, by column name.
type row struct {
	file string
	n    int
	cols map[string]string
}

// table reads a table file: its first line names the columns, which must
// include required.
func (l *loader) table(file string, required ...string) []row {
	lines := l.read(file)
	if len(lines) == 0 {
		return nil
	}
	head := lines[0]
	for _, c := range required {
		if !slices.Contains(head.fields, c) {
			l.errorf(file, head.n, "no column %q", c)
		}
	}
	var rows []row
	for _, ln := range lines[1:] {
		if len(ln.fields) != len(head.fields) {
			l.errorf(file, ln.n, "%d columns, want %d (%s)", len(ln.fields), len(head.fields), strings.Join(head.fields, " "))
			continue
		}
		r := row{file, ln.n, map[string]string{}}
		for i, c := range head.fields {
			r.cols[c] = ln.fields[i]
		}
		rows = append(rows, r)
	}
	return rows
}

// int parses the column c of r; at least min.
func (l *loader) int(r row, c string, min int) int {
	n, err := strconv.Atoi(r.cols[c])
	if err != nil || n < min {
		l.errorf(r.file, r.n, "%s: want a number of at least %d, not %q", c, min, r.cols[c])
	}
	return n
}

// keyInt parses the value of key in m; at least min. A missing key is a
// mistake.
func (l *loader) keyInt(file string, m map[string]line, key string, min int) int {
	ln, ok := m[key]
	if !ok {
		l.errorf(file, 0, "no %q", key)
		return 0
	}
	n, err := strconv.Atoi(ln.fields[1])
	if err != nil || n < min {
		l.errorf(file, ln.n, "%s: want a number of at least %d, not %q", key, min, ln.fields[1])
	}
	return n
}

// floors parses the columns 開始 and 終了 of r: a range of floors of the
// dungeon.
func (l *loader) floors(r row) (int, int) {
	first, last := l.int(r, "開始", 1), l.int(r, "終了", 1)
	if last < first || last > l.d.Floors {
		l.errorf(r.file, r.n, "floors %d-%d out of 1-%d", first, last, l.d.Floors)
	}
	return first, last
}

// span parses a column of the form "min-max" (or one number).
func (l *loader) span(r row, c string) (int, int) {
	lo, hi, ok := strings.Cut(r.cols[c], "-")
	if !ok {
		hi = lo
	}
	a, err1 := strconv.Atoi(lo)
	b, err2 := strconv.Atoi(hi)
	if err1 != nil || err2 != nil || a < 0 || b < a {
		l.errorf(r.file, r.n, "%s: want min-max, not %q", c, r.cols[c])
	}
	return a, b
}

// glyph checks that a glyph is one full-width character, as the map needs.
func (l *loader) glyph(r row, c string) string {
	g := r.cols[c]
	if n := len([]rune(g)); n != 1 || len(g) != glyphBytes {
		l.errorf(r.file, r.n, "%s: want one full-width character (3 bytes in UTF-8), not %q", c, g)
	}
	return g
}

var speeds = map[string]Speed{"鈍足": SpeedSlow, "通常": SpeedNormal, "倍速": SpeedDouble, "移動倍速": SpeedMoveDouble}

func (l *loader) load() {
	l.loadHeader()
	l.loadRules()
	l.loadGrowth()
	l.loadMonsters()
	l.loadSeals()
	l.loadItems()
	l.loadFakeNames()
	l.loadTables()
	l.loadTraps()
	l.loadShops()
	l.loadFloors()
	l.checkFloors()

	sum := sha256.Sum256(l.h)
	l.d.Hash = fmt.Sprintf("%x", sum[:4])
}

// loadHeader reads the header: the name, the description and the floors.
func (l *loader) loadHeader() {
	d := l.d
	h := l.keys("header.txt", []string{"名前", "説明", "階数", headerBase})
	if ln, ok := h["名前"]; ok {
		d.Name = ln.fields[1]
	} else {
		l.errorf("header.txt", 0, "no %q", "名前")
	}
	if ln, ok := h["説明"]; ok {
		d.Description = ln.fields[1]
	}
	d.Floors = l.keyInt("header.txt", h, "階数", 1)
	if ln, ok := h[headerBase]; ok {
		d.Base = ln.fields[1]
	}
}

// loadRules reads the rules.
func (l *loader) loadRules() {
	d := l.d
	rk := l.keys("rules.txt", []string{
		"最大HP", "ちから", "回復ターン", "湧きターン", "最大モンスター数",
		"風1", "風2", "風3", "防御K", "主の命中", "敵の命中", "成長", "最初の仲間", "強敵ランク差",
		"持ち物の数", "ひびの段階", "腕輪の傷みやすさ", "満腹度", "腹減りターン", "見つけた罠", "デバッグ",
		"お金投げ", "落とし物", "仲間の命中", "主の投擲命中", "敵の投擲命中", "仲間の上限", "武具の呪い", "腕輪の呪い", "道具の呪い", "食料の呪い", "祝福", "ラッキー",
		"超過攻撃", "超過防御",
	})
	l.rules = rk
	r := &d.Rules
	r.PlayerHP = l.keyInt("rules.txt", rk, "最大HP", 1)
	r.Strength = l.keyInt("rules.txt", rk, "ちから", 0)
	r.RegenTurns = l.keyInt("rules.txt", rk, "回復ターン", 1)
	r.SpawnTurns = l.keyInt("rules.txt", rk, "湧きターン", 1)
	r.MaxMonsters = l.keyInt("rules.txt", rk, "最大モンスター数", 1)
	for i := range r.WindTurns {
		r.WindTurns[i] = l.keyInt("rules.txt", rk, fmt.Sprint("風", i+1), 1)
		if i > 0 && r.WindTurns[i] <= r.WindTurns[i-1] {
			l.errorf("rules.txt", rk[fmt.Sprint("風", i+1)].n, "風%d must come after 風%d", i+1, i)
		}
	}
	r.DefenseK = l.keyInt("rules.txt", rk, "防御K", 1)
	r.PlayerHitRate = l.keyInt("rules.txt", rk, "主の命中", 0)
	r.MonsterHitRate = l.keyInt("rules.txt", rk, "敵の命中", 0)
	r.StrongerBy = l.keyInt("rules.txt", rk, "強敵ランク差", 0)
	r.Capacity = l.keyInt("rules.txt", rk, "持ち物の数", 1)
	r.CrackStages = l.keyInt("rules.txt", rk, "ひびの段階", 0)
	r.Wear = l.keyInt("rules.txt", rk, "腕輪の傷みやすさ", 0)
	r.Fullness = l.keyInt("rules.txt", rk, "満腹度", 1)
	// (Optional: dungeons from before it keep their hash, so their runs
	// and replays.)
	r.GoldThrow = defaultGoldThrow
	if _, ok := rk["お金投げ"]; ok {
		r.GoldThrow = l.keyInt("rules.txt", rk, "お金投げ", 1)
	}
	r.AllyHitRate = r.MonsterHitRate
	if _, ok := rk["仲間の命中"]; ok {
		r.AllyHitRate = l.keyInt("rules.txt", rk, "仲間の命中", 0)
	}
	r.PlayerThrowRate, r.MonsterThrowRate = r.PlayerHitRate, r.MonsterHitRate
	if _, ok := rk["主の投擲命中"]; ok {
		r.PlayerThrowRate = l.keyInt("rules.txt", rk, "主の投擲命中", 0)
	}
	if _, ok := rk["敵の投擲命中"]; ok {
		r.MonsterThrowRate = l.keyInt("rules.txt", rk, "敵の投擲命中", 0)
	}
	for _, key := range []string{"主の命中", "敵の命中", "仲間の命中", "主の投擲命中", "敵の投擲命中"} {
		if ln, ok := rk[key]; ok {
			if n, err := strconv.Atoi(ln.fields[1]); err == nil && n > 100 {
				l.errorf("rules.txt", ln.n, "%s: %d above 100", key, n)
			}
		}
	}
	if _, ok := rk["落とし物"]; ok {
		r.DropRate = l.keyInt("rules.txt", rk, "落とし物", 0)
		if r.DropRate > 100 {
			l.errorf("rules.txt", rk["落とし物"].n, "落とし物: %d above 100", r.DropRate)
		}
	}
	if ln, ok := rk["見つけた罠"]; !ok {
		l.errorf("rules.txt", 0, "no %q", "見つけた罠")
	} else {
		switch ln.fields[1] {
		case "安全":
			r.FoundTrapsSafe = true
		case "発動":
		default:
			l.errorf("rules.txt", ln.n, "見つけた罠: want 安全 or 発動, not %q", ln.fields[1])
		}
	}
	r.HungerTurns = l.keyInt("rules.txt", rk, "腹減りターン", 1)
	if ln, ok := rk["成長"]; ok {
		r.Growth = ln.fields[1]
	} else {
		l.errorf("rules.txt", 0, "no %q", "成長")
	}
	if ln, ok := rk["最初の仲間"]; ok {
		r.FirstAlly = ln.fields[1]
	}
	r.MaxAllies = 3
	if _, ok := rk["仲間の上限"]; ok {
		r.MaxAllies = l.keyInt("rules.txt", rk, "仲間の上限", 1)
	}
	if _, ok := rk["武具の呪い"]; ok {
		r.CurseGear = l.keyInt("rules.txt", rk, "武具の呪い", 0)
	}
	if _, ok := rk["腕輪の呪い"]; ok {
		r.CurseBracelet = l.keyInt("rules.txt", rk, "腕輪の呪い", 0)
	}
	if _, ok := rk["道具の呪い"]; ok {
		r.CurseItem = l.keyInt("rules.txt", rk, "道具の呪い", 0)
	}
	if _, ok := rk["ラッキー"]; ok {
		r.Lucky = l.keyInt("rules.txt", rk, "ラッキー", 0)
	}
	if _, ok := rk["祝福"]; ok {
		r.Bless = l.keyInt("rules.txt", rk, "祝福", 0)
	}
	if _, ok := rk["食料の呪い"]; ok {
		r.CurseFood = l.keyInt("rules.txt", rk, "食料の呪い", 0)
	}
	r.OverAttack, r.OverDefense = defaultOverAttack, defaultOverDefense
	if _, ok := rk["超過攻撃"]; ok {
		r.OverAttack = l.keyInt("rules.txt", rk, "超過攻撃", 100)
	}
	if _, ok := rk["超過防御"]; ok {
		r.OverDefense = l.keyInt("rules.txt", rk, "超過防御", 100)
	}
	// Optional: most dungeons have no debug mode.
	if ln, ok := rk["デバッグ"]; ok {
		switch ln.fields[1] {
		case "可":
			r.Debug = true
		case "不可":
		default:
			l.errorf("rules.txt", ln.n, "デバッグ: want 可 or 不可, not %q", ln.fields[1])
		}
	}
}

// loadGrowth reads the growth tables.
func (l *loader) loadGrowth() {
	d := l.d
	r := &d.Rules
	for _, row := range l.table("growth.txt", "名前", "レベル", "経験値", "HP", "攻撃", "防御") {
		name := row.cols["名前"]
		g := GrowthRow{
			Level: l.int(row, "レベル", 2), EXP: l.int(row, "経験値", 1),
			HP: l.int(row, "HP", 0), Attack: l.int(row, "攻撃", 0), Defense: l.int(row, "防御", 0),
		}
		rows := d.Growth[name]
		if want := len(rows) + 2; g.Level != want {
			l.errorf(row.file, row.n, "%s: level %d, want %d (levels in order from 2)", name, g.Level, want)
		} else if len(rows) > 0 && g.EXP <= rows[len(rows)-1].EXP {
			l.errorf(row.file, row.n, "%s: EXP %d not above the level before", name, g.EXP)
		}
		d.Growth[name] = append(rows, g)
	}
	if _, ok := d.Growth[r.Growth]; !ok && r.Growth != "" {
		l.errorf("rules.txt", l.rules["成長"].n, "no growth table %q", r.Growth)
	}
}

// loadMonsters reads the monsters.
func (l *loader) loadMonsters() {
	d := l.d
	r := &d.Rules
	ids := map[string]bool{}
	for _, row := range l.table("monsters.txt", "ID", "字", "名前", "ランク", "HP", "攻撃", "防御", "経験値", "速度") {
		m := &MonsterDef{
			ID: row.cols["ID"], Glyph: l.glyph(row, "字"), Name: row.cols["名前"],
			Rank: l.int(row, "ランク", 1), HP: l.int(row, "HP", 1),
			Attack: l.int(row, "攻撃", 0), Defense: l.int(row, "防御", 0), EXP: l.int(row, "経験値", 0),
			Growth: row.cols["成長"],
		}
		if m.Rank > maxRank {
			l.errorf(row.file, row.n, "ランク: %d above %d", m.Rank, maxRank)
		}
		s, ok := speeds[row.cols["速度"]]
		if !ok {
			l.errorf(row.file, row.n, "速度: want 鈍足, 通常, 倍速 or 移動倍速, not %q", row.cols["速度"])
		}
		m.Speed = s
		as, err := parseAbilities(row.cols["能力"])
		if err != nil {
			l.errorf(row.file, row.n, "能力: %v", err)
		}
		for i := range as {
			as[i].file, as[i].line = row.file, row.n
		}
		m.Abilities = as
		m.Drop = l.drop(row)
		if v := row.cols["説明"]; v != "-" {
			m.Description = v
		}
		m.Asleep = -1
		if v, ok := row.cols["眠り"]; ok && v != "-" {
			m.Asleep = l.int(row, "眠り", 0)
			if m.Asleep > 100 {
				l.errorf(row.file, row.n, "眠り: %d above 100", m.Asleep)
			}
		}
		switch k := row.cols["種別"]; k {
		case "", "-":
		case KindKeeper, KindGuard:
			m.Kind = k
		default:
			l.errorf(row.file, row.n, "種別: want -, 店主 or 番犬, not %q", k)
		}
		if m.Growth == "" {
			m.Growth = r.Growth
		} else if _, ok := d.Growth[m.Growth]; !ok {
			l.errorf(row.file, row.n, "no growth table %q", m.Growth)
		}
		if ids[m.ID] {
			l.errorf(row.file, row.n, "the ID %q again", m.ID)
		}
		ids[m.ID] = true
		d.Monsters = append(d.Monsters, m)
	}
	if r.FirstAlly != "" && d.Monster(r.FirstAlly) == nil {
		l.errorf("rules.txt", l.rules["最初の仲間"].n, "no monster %q", r.FirstAlly)
	}
}

// loadSeals reads the seals.
func (l *loader) loadSeals() {
	d := l.d
	for _, row := range l.table("seals.txt", "印", "名前", "効果", "強さ") {
		s := &SealDef{Glyph: row.cols["印"], Name: row.cols["名前"], Effect: SealEffect(row.cols["効果"])}
		if len([]rune(s.Glyph)) != 1 {
			l.errorf(row.file, row.n, "印: want one character, not %q", s.Glyph)
		}
		if !slices.Contains(sealEffects, s.Effect) {
			l.errorf(row.file, row.n, "効果: unknown %q", s.Effect)
		}
		s.Strength = l.int(row, "強さ", 0)
		s.AsSeal = s.Strength
		if v, ok := row.cols["印の強さ"]; ok && v != "" && v != "-" {
			s.AsSeal = l.int(row, "印の強さ", 0)
		}
		if f, ok := row.cols["付く先"]; ok {
			s.For = Category(f)
			if s.For != CategoryWeapon && s.For != CategoryShield {
				l.errorf(row.file, row.n, "付く先: want %s or %s, not %q", CategoryWeapon, CategoryShield, f)
			}
		}
		// One glyph, two seals: one for weapons and one for shields
		// (a material gives either, by what it is merged into).
		for _, o := range d.Seals {
			if o.Glyph == s.Glyph && (o.For == "" || s.For == "" || o.For == s.For) {
				l.errorf(row.file, row.n, "the seal %q again", s.Glyph)
				break
			}
		}
		d.Seals = append(d.Seals, s)
	}
}

// loadItems reads the items.
func (l *loader) loadItems() {
	d := l.d
	for _, row := range l.table("items.txt", "ID", "字", "名前", "種類") {
		it := &ItemDef{ID: row.cols["ID"], Glyph: l.glyph(row, "字"), Name: row.cols["名前"]}
		c := Category(row.cols["種類"])
		if !slices.Contains(categories, c) {
			l.errorf(row.file, row.n, "種類: unknown %q", c)
		}
		it.Category = c
		if _, ok := row.cols["強さ"]; ok {
			it.Strength = l.int(row, "強さ", 0)
		}
		if e, ok := row.cols["効果"]; ok && e != "-" && e != "" {
			if !slices.Contains(effects[c], Effect(e)) && !castable(c, MagicEffect(e)) {
				l.errorf(row.file, row.n, "効果: %s has no effect %q, want one of %v, or an effect of a bolt (monsters.txt 魔法)", c, e, effects[c])
			}
			it.Effect = Effect(e)
		}
		if n, ok := row.cols["回数"]; ok && n != "-" && n != "" {
			if c != CategoryStaff && c != CategoryArrow && c != CategoryPot {
				l.errorf(row.file, row.n, "回数: only staffs, arrows and pots have one, not %s", c)
			}
			it.Charges[0], it.Charges[1] = l.span(row, "回数")
		}
		if seal, ok := row.cols["印"]; ok && seal != "-" && seal != "" {
			if d.Seal(seal) == nil {
				l.errorf(row.file, row.n, "印: no seal %q", seal)
			}
			it.Seal = seal
		}
		if _, ok := row.cols["印数"]; ok {
			it.Slots = l.int(row, "印数", 0)
		}
		if _, ok := row.cols["値段"]; ok {
			it.Price = l.int(row, "値段", 0)
		}
		equipment := c == CategoryWeapon || c == CategoryShield
		if it.Slots > 0 && !equipment {
			l.errorf(row.file, row.n, "印数: only weapons and shields have seals, not %s", c)
		}
		if it.Seal != "" && c == CategoryMoney {
			l.errorf(row.file, row.n, "印: money is no material")
		}
		if s := d.Seal(it.Seal); s != nil && equipment && d.SealOn(it.Seal, c) == nil {
			l.errorf(row.file, row.n, "印: the seal %q goes on a %s, not a %s", it.Seal, s.For, c)
		}
		if (c == CategoryStaff || c == CategoryArrow || c == CategoryPot) && it.Charges[1] == 0 {
			l.errorf(row.file, row.n, "回数: %s needs some", c)
		}
		if _, ok := row.cols["ひび"]; ok {
			it.Crack = l.int(row, "ひび", 0)
			if it.Crack > 100 {
				l.errorf(row.file, row.n, "ひび: %d above 100", it.Crack)
			} else if it.Crack > 0 && c != CategoryBracelet {
				l.errorf(row.file, row.n, "ひび: only bracelets crack, not %s", c)
			}
		}
		if d.Item(it.ID) != nil {
			l.errorf(row.file, row.n, "the ID %q again", it.ID)
		}
		d.Items = append(d.Items, it)
	}
}

// loadFakeNames reads the fake names of the kinds not identified.
func (l *loader) loadFakeNames() {
	d := l.d
	d.FakeNames = map[Category][]string{}
	for _, row := range l.table("fake_names.txt", "種類", "名前") {
		c, name := Category(row.cols["種類"]), row.cols["名前"]
		switch {
		case !slices.Contains(categories, c):
			l.errorf(row.file, row.n, "種類: unknown %q", c)
		case c == CategoryMoney:
			l.errorf(row.file, row.n, "種類: money is never unidentified")
		case slices.ContainsFunc(d.Items, func(it *ItemDef) bool { return it.Name == name }):
			l.errorf(row.file, row.n, "名前: %q is the name of an item", name)
		case slices.Contains(d.FakeNames[c], name):
			l.errorf(row.file, row.n, "名前: %q again", name)
		default:
			d.FakeNames[c] = append(d.FakeNames[c], name)
		}
	}
	for _, c := range categories {
		names := d.FakeNames[c]
		n := 0
		for _, it := range d.Items {
			if it.Category == c {
				n++
			}
		}
		if len(names) > 0 && len(names) < n {
			l.errorf("fake_names.txt", 0, "too few names: %d for the %d items of %s", len(names), n, c)
		}
	}
}

// loadTables reads the tables of the monsters and items by floor, checks
// the drops, and reads the guaranteed items.
func (l *loader) loadTables() {
	d := l.d
	// Shopkeepers and guards are not monsters of the floors.
	d.MonsterRows = l.appearance("monster_table.txt", func(id string) bool { m := d.Monster(id); return m != nil && m.Kind == "" })
	l.loadItemTables(d)
	l.checkDrops(d)
	if _, err := fs.Stat(l.fsys, "guaranteed_items.txt"); err == nil {
		for _, row := range l.table("guaranteed_items.txt", "開始", "終了", "ID") {
			r := FloorRow{ID: row.cols["ID"]}
			r.First, r.Last = l.floors(row)
			if it := d.Item(r.ID); it == nil || it.Category == CategoryMoney {
				l.errorf(row.file, row.n, "ID: no item %q", r.ID)
			}
			d.Guaranteed = append(d.Guaranteed, r)
		}
	}
}

// loadTraps reads the traps and their table.
func (l *loader) loadTraps() {
	d := l.d
	for _, row := range l.table("traps.txt", "ID", "名前", "効果", "強さ") {
		t := &TrapDef{ID: row.cols["ID"], Name: row.cols["名前"], Effect: TrapEffect(row.cols["効果"])}
		if !slices.Contains(trapEffects, t.Effect) {
			l.errorf(row.file, row.n, "効果: unknown %q", t.Effect)
		}
		t.Strength = l.int(row, "強さ", 0)
		if d.Trap(t.ID) != nil {
			l.errorf(row.file, row.n, "the ID %q again", t.ID)
		}
		d.Traps = append(d.Traps, t)
	}
	d.TrapRows = l.appearance("trap_table.txt", func(id string) bool { return d.Trap(id) != nil })
}

// loadShops reads the shops and their table.
func (l *loader) loadShops() {
	d := l.d
	for _, row := range l.table("shops.txt", "ID", "名前", "品表", "店主", "店主の数", "番犬", "番犬の数") {
		sh := &ShopDef{ID: row.cols["ID"], Name: row.cols["名前"], Table: row.cols["品表"]}
		if d.ItemTable(sh.Table) == nil {
			l.errorf(row.file, row.n, "品表: no item table %q (category_tables.txt)", sh.Table)
		}
		if sh.Keeper = d.Monster(row.cols["店主"]); sh.Keeper == nil || sh.Keeper.Kind != KindKeeper {
			l.errorf(row.file, row.n, "店主: no shopkeeper %q (種別 店主 in monsters.txt)", row.cols["店主"])
		}
		sh.Keepers[0], sh.Keepers[1] = l.span(row, "店主の数")
		if sh.Keepers[0] < 1 {
			l.errorf(row.file, row.n, "店主の数: at least 1")
		}
		if id := row.cols["番犬"]; id != "-" && id != "" {
			if sh.Guard = d.Monster(id); sh.Guard == nil || sh.Guard.Kind != KindGuard {
				l.errorf(row.file, row.n, "番犬: no guard %q (種別 番犬 in monsters.txt)", id)
			}
		}
		sh.Guards[0], sh.Guards[1] = l.span(row, "番犬の数")
		if _, ok := row.cols["品数"]; ok {
			sh.Goods[0], sh.Goods[1] = l.span(row, "品数")
			if sh.Goods[0] < 1 {
				l.errorf(row.file, row.n, "品数: at least 1")
			}
		}
		if d.Shop(sh.ID) != nil {
			l.errorf(row.file, row.n, "the ID %q again", sh.ID)
		}
		d.Shops = append(d.Shops, sh)
	}
	d.ShopRows = l.appearance("shop_table.txt", func(id string) bool { return d.Shop(id) != nil })
}

// loadFloors reads the floors.
func (l *loader) loadFloors() {
	d := l.d
	for _, row := range l.table("floors.txt", "開始", "終了", "敵の数", "道具の数") {
		c := FloorCounts{}
		c.First, c.Last = l.floors(row)
		c.MinMonster, c.MaxMonster = l.span(row, "敵の数")
		c.MinItem, c.MaxItem = l.span(row, "道具の数")
		c.ItemTable = "床"
		if name, ok := row.cols["道具表"]; ok {
			c.ItemTable = name
		}
		if d.ItemTable(c.ItemTable) == nil {
			l.errorf(row.file, row.n, "道具表: no item table %q (category_tables.txt)", c.ItemTable)
		}
		if _, ok := row.cols["罠の数"]; ok {
			c.MinTrap, c.MaxTrap = l.span(row, "罠の数")
		}
		if _, ok := row.cols["ハウスの罠"]; ok {
			c.MinHouseTrap, c.MaxHouseTrap = l.span(row, "ハウスの罠")
		}
		percent := func(col string) int {
			if _, ok := row.cols[col]; !ok {
				return 0
			}
			n := l.int(row, col, 0)
			if n > 100 {
				l.errorf(row.file, row.n, "%s: %d above 100", col, n)
			}
			return n
		}
		c.BigRoom, c.Maze, c.House = percent("大部屋"), percent("迷路"), percent("モンスターハウス")
		c.Shop = percent("店")
		if c.BigRoom+c.Maze > 100 {
			l.errorf(row.file, row.n, "大部屋 and 迷路: %d%% together", c.BigRoom+c.Maze)
		}
		c.Brightness = BrightnessNormal
		if b, ok := row.cols["明るさ"]; ok {
			c.Brightness = Brightness(b)
			switch c.Brightness {
			case BrightnessNormal, BrightnessBright, BrightnessDark:
			default:
				l.errorf(row.file, row.n, "明るさ: want 通常, 明るい or 暗い, not %q", b)
			}
		}
		d.FloorRows = append(d.FloorRows, c)
	}
}

// checkFloors checks that every floor has its monsters, items and counts.
func (l *loader) checkFloors() {
	d := l.d
	// Every floor needs its monsters, items and counts.
	for f := 1; f <= d.Floors; f++ {
		if !slices.ContainsFunc(d.MonsterRows, func(r FloorRow) bool { return r.First <= f && f <= r.Last }) {
			l.errorf("monster_table.txt", 0, "no monster on %dF", f)
		}
		if c := d.counts(f); c.ItemTable != "" && d.ItemTable(c.ItemTable) != nil {
			if err := d.checkItemTable(c.ItemTable, f, false); err != nil {
				l.errorf("floors.txt", 0, "%v", err)
			}
		}
		if d.counts(f).Shop > 0 && !slices.ContainsFunc(d.ShopRows, func(r FloorRow) bool { return r.First <= f && f <= r.Last }) {
			l.errorf("shop_table.txt", 0, "no shop on %dF", f)
		}
		// A shop sells on every floor it appears on.
		for _, r := range d.ShopRows {
			if sh := d.Shop(r.ID); sh != nil && on(f, r.First, r.Last) && d.ItemTable(sh.Table) != nil {
				if err := d.checkItemTable(sh.Table, f, true); err != nil {
					l.errorf("shops.txt", 0, "%s: %v", sh.ID, err)
				}
			}
		}
		if d.counts(f).MaxTrap > 0 && !slices.ContainsFunc(d.TrapRows, func(r FloorRow) bool { return r.First <= f && f <= r.Last }) {
			l.errorf("trap_table.txt", 0, "no trap on %dF", f)
		}
		if !slices.ContainsFunc(d.FloorRows, func(r FloorCounts) bool { return r.First <= f && f <= r.Last }) {
			l.errorf("floors.txt", 0, "no counts for %dF", f)
		}
	}
}

// appearance reads an appearance table; known says whether an ID exists.
func (l *loader) appearance(file string, known func(string) bool) []FloorRow {
	var rows []FloorRow
	for _, row := range l.table(file, "開始", "終了", "ID", "頻度") {
		r := FloorRow{ID: row.cols["ID"], Weight: l.int(row, "頻度", 1)}
		r.First, r.Last = l.floors(row)
		if !known(r.ID) {
			l.errorf(row.file, row.n, "no such ID %q", r.ID)
		}
		rows = append(rows, r)
	}
	return rows
}

// maxRank is the highest rank (red).
const maxRank = 4

// Monster returns the monster id, or nil.
func (d *Dungeon) Monster(id string) *MonsterDef {
	for _, m := range d.Monsters {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// Item returns the item id, or nil.
func (d *Dungeon) Item(id string) *ItemDef {
	for _, it := range d.Items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// pick returns an ID of rows for the floor, by weight.
func pick(rng *rand.Rand, rows []FloorRow, floor int) string {
	total := 0
	for _, r := range rows {
		if r.First <= floor && floor <= r.Last {
			total += r.Weight
		}
	}
	n := rng.IntN(total)
	for _, r := range rows {
		if r.First <= floor && floor <= r.Last {
			if n < r.Weight {
				return r.ID
			}
			n -= r.Weight
		}
	}
	panic("unreachable")
}

// counts returns the counts of the floor (the first row that has it).
func (d *Dungeon) counts(floor int) FloorCounts {
	for _, c := range d.FloorRows {
		if c.First <= floor && floor <= c.Last {
			return c
		}
	}
	return FloorCounts{}
}

// nextRank returns the kind of the family of m one rank above, or nil.
func (d *Dungeon) nextRank(m *MonsterDef) *MonsterDef {
	for _, n := range d.Monsters {
		if n.Glyph == m.Glyph && n.Rank == m.Rank+1 {
			return n
		}
	}
	return nil
}

// prevRank returns the kind of the family of m one rank below, or nil.
func (d *Dungeon) prevRank(m *MonsterDef) *MonsterDef {
	for _, n := range d.Monsters {
		if n.Glyph == m.Glyph && n.Rank == m.Rank-1 {
			return n
		}
	}
	return nil
}

// ofRank returns the kinds of rank, or of the highest rank below it there
// is.
func (d *Dungeon) ofRank(rank int) []*MonsterDef {
	for ; rank > 0; rank-- {
		var ms []*MonsterDef
		for _, m := range d.Monsters {
			if m.Rank == rank && m.Kind == "" {
				ms = append(ms, m)
			}
		}
		if len(ms) > 0 {
			return ms
		}
	}
	return nil
}

// Trap returns the trap id, or nil.
func (d *Dungeon) Trap(id string) *TrapDef {
	for _, t := range d.Traps {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Seal returns the seal of the glyph, or nil: the first, if the glyph
// has one seal for weapons and another for shields.
func (d *Dungeon) Seal(glyph string) *SealDef {
	for _, s := range d.Seals {
		if s.Glyph == glyph {
			return s
		}
	}
	return nil
}

// SealOn returns the seal of the glyph that goes on an item of the
// category c (a weapon or a shield), or nil.
func (d *Dungeon) SealOn(glyph string, c Category) *SealDef {
	for _, s := range d.Seals {
		if s.Glyph == glyph && (s.For == "" || s.For == c) {
			return s
		}
	}
	return nil
}

// Shop returns the shop id, or nil.
func (d *Dungeon) Shop(id string) *ShopDef {
	for _, s := range d.Shops {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// drop reads the 落とし物 of a row of monsters.txt: - (or no column) for
// the dungeon's rule, なし, or what and a percent: 道具:20, お金:100, or the
// ID of an item: onigiri:50.
func (l *loader) drop(row row) *Drop {
	v, ok := row.cols["落とし物"]
	if !ok || v == "-" {
		return nil
	}
	if v == DropNone {
		return &Drop{What: DropNone}
	}
	what, rate, ok := strings.Cut(v, ":")
	n, err := strconv.Atoi(rate)
	if !ok || err != nil || n < 1 || n > 100 {
		l.errorf(row.file, row.n, "落とし物: want -, なし, or what:percent (1 to 100), not %q", v)
		return nil
	}
	// (Items are read later: checkDrops checks the item.)
	return &Drop{What: what, Rate: n, file: row.file, line: row.n}
}

// checkDrops checks the items the monsters drop, and the arrows they
// shoot.
func (l *loader) checkDrops(d *Dungeon) {
	for _, m := range d.Monsters {
		for _, a := range m.Abilities {
			if a.Name == AbilityRot && d.rotted() == nil {
				l.errorf(a.file, a.line, "能力: %s: no rotten onigiri (an item of the effect %s)", a.Name, EffectRotten)
			}
			if a.Item == "" {
				continue
			}
			if it := d.Item(a.Item); it == nil || it.Category != CategoryArrow {
				l.errorf(a.file, a.line, "能力: %s: no arrow %q", a.Name, a.Item)
			}
		}
		if m.Drop == nil || m.Drop.What == DropItem || m.Drop.What == DropMoney || m.Drop.What == DropNone {
			continue
		}
		if c := Category(m.Drop.What); slices.Contains(categories, c) && c != CategoryMoney {
			continue
		}
		if it := d.Item(m.Drop.What); it == nil || it.Category == CategoryMoney {
			l.errorf(m.Drop.file, m.Drop.line, "落とし物: no item %q (want 道具, お金, なし, a kind of items or the ID of an item)", m.Drop.What)
		}
	}
}
