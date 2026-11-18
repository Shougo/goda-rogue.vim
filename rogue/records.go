package rogue

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The records (番付) of the finished runs: each run that ends (falls or
// clears) appends a line to a file next to the replay of the last run.
// Records are not progression: nothing of them goes into a run.

// recordsFile is the name of the file of the records, in the directory
// of the replays.
const recordsFile = "records.tsv"

// recordsHeader is the first line of the file: the columns.
const recordsHeader = "# 日時\tダンジョン\tシード\t到達\t踏破\t結果\tターン\t時間\tデバッグ"

// Record is a finished run.
type Record struct {
	Date    time.Time
	Dungeon string
	Seed    string
	Depth   int
	Cleared bool
	Result  string
	Turns   int
	Time    time.Duration
	Debug   bool
}

// recordOf is the record of the game g, over.
func recordOf(g *Game) Record {
	return Record{
		Date: g.Ended, Dungeon: g.Dungeon.ID, Seed: g.Seed, Depth: g.Depth,
		Cleared: g.Cleared, Result: g.Over, Turns: g.Turn, Time: g.PlayTime(),
		Debug: g.Debugged,
	}
}

// cell makes s fit in a cell of the file: no tab nor newline.
func cell(s string) string { return strings.NewReplacer("\t", " ", "\n", " ").Replace(s) }

// line is the record as a line of the file.
func (r Record) line() string {
	flag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	return strings.Join([]string{
		r.Date.Format(time.RFC3339), cell(r.Dungeon), cell(r.Seed), strconv.Itoa(r.Depth),
		flag(r.Cleared), cell(r.Result), strconv.Itoa(r.Turns),
		strconv.FormatInt(r.Time.Milliseconds(), 10), flag(r.Debug),
	}, "\t")
}

// appendRecord appends the record r to the file path.
func appendRecord(path string, r Record) error {
	_, err := os.Stat(path)
	f, ferr := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if ferr != nil {
		return ferr
	}
	defer f.Close()
	s := r.line() + "\n"
	if errors.Is(err, os.ErrNotExist) {
		s = recordsHeader + "\n" + s
	}
	_, err = f.WriteString(s)
	return err
}

// readRecords reads the records of the file path (none if there is no
// file). Lines it cannot read are skipped: the file is the player's.
func readRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rs []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 9 {
			continue
		}
		date, err1 := time.Parse(time.RFC3339, c[0])
		depth, err2 := strconv.Atoi(c[3])
		turns, err3 := strconv.Atoi(c[6])
		ms, err4 := strconv.ParseInt(c[7], 10, 64)
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			continue
		}
		rs = append(rs, Record{
			Date: date, Dungeon: c[1], Seed: c[2], Depth: depth, Cleared: c[4] == "1",
			Result: c[5], Turns: turns, Time: time.Duration(ms) * time.Millisecond, Debug: c[8] == "1",
		})
	}
	return rs, sc.Err()
}

// better says whether the run a is better than b: a clear before a fall,
// the faster clear (time, then turns), the deeper fall.
func better(a, b Record) bool {
	switch {
	case a.Cleared != b.Cleared:
		return a.Cleared
	case a.Cleared && a.Time != b.Time:
		return a.Time < b.Time
	case a.Cleared:
		return a.Turns < b.Turns
	case a.Depth != b.Depth:
		return a.Depth > b.Depth
	}
	return a.Turns < b.Turns
}

// summary is the record in a line, as Game.Summary.
func (r Record) summary(name string) string {
	debug := ""
	if r.Debug {
		debug = "[デバッグ] "
	}
	return fmt.Sprintf("%s  %s%s %dF %s  %s  %dターン  シード %s",
		r.Date.Local().Format("2006-01-02 15:04"), debug, name, r.Depth, r.Result,
		playTime(r.Time), r.Turns, r.Seed)
}

// recordLines are the lines to show the records rs: the best run of each
// dungeon (debug runs never count), then every run, the newest first.
// names gives the names of the dungeons by ID.
func recordLines(rs []Record, names map[string]string) []string {
	name := func(id string) string {
		if n, ok := names[id]; ok {
			return n
		}
		return id
	}
	if len(rs) == 0 {
		return []string{"記録はまだない。"}
	}
	lines := []string{"自己ベスト", ""}
	best := map[string]Record{}
	var dungeons []string
	for _, r := range rs {
		if r.Debug {
			continue
		}
		b, ok := best[r.Dungeon]
		if !ok {
			dungeons = append(dungeons, r.Dungeon)
		}
		if !ok || better(r, b) {
			best[r.Dungeon] = r
		}
	}
	slices.Sort(dungeons)
	for _, id := range dungeons {
		lines = append(lines, "  "+best[id].summary(name(id)))
	}
	if len(dungeons) == 0 {
		lines = append(lines, "  （デバッグなしの記録はまだない）")
	}
	won := 0
	for _, r := range rs {
		if r.Cleared && !r.Debug {
			won++
		}
	}
	lines = append(lines, "", fmt.Sprintf("すべての記録（%d回、踏破%d回）", len(rs), won), "")
	for i := len(rs) - 1; i >= 0; i-- {
		lines = append(lines, "  "+rs[i].summary(name(rs[i].Dungeon)))
	}
	return lines
}
