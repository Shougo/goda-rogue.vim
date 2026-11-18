package rogue

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRecordsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), recordsFile)
	if rs, err := readRecords(path); err != nil || rs != nil {
		t.Fatalf("no file: %v %v", rs, err)
	}
	date := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	want := []Record{
		{Date: date, Dungeon: "cave", Seed: "a\tb", Depth: 7, Result: "鼠に倒された", Turns: 900, Time: 61 * time.Second},
		{Date: date.Add(time.Hour), Dungeon: "cave", Seed: "c", Depth: 30, Cleared: true, Result: "踏破した", Turns: 9000, Time: time.Hour, Debug: true},
	}
	for _, r := range want {
		if err := appendRecord(path, r); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(string(b), "\n"); lines[0] != recordsHeader || len(lines) != 4 {
		t.Errorf("file %q", b)
	}
	// A broken line is skipped.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("broken\n")
	f.Close()
	got, err := readRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	want[0].Seed = "a b"
	if !slices.EqualFunc(got, want, func(a, b Record) bool { return a.Date.Equal(b.Date) && a.line() == b.line() }) {
		t.Errorf("records %+v, want %+v", got, want)
	}
}

func TestRecordLines(t *testing.T) {
	if got := recordLines(nil, nil); !slices.Equal(got, []string{"記録はまだない。"}) {
		t.Errorf("no records: %q", got)
	}
	date := time.Date(2026, 11, 18, 12, 0, 0, 0, time.Local)
	rs := []Record{
		{Date: date, Dungeon: "cave", Seed: "1", Depth: 5, Result: "倒れた"},
		{Date: date, Dungeon: "cave", Seed: "2", Depth: 12, Result: "倒れた"},
		{Date: date, Dungeon: "cave", Seed: "3", Depth: 30, Cleared: true, Result: "踏破", Time: 2 * time.Hour},
		{Date: date, Dungeon: "cave", Seed: "4", Depth: 30, Cleared: true, Result: "踏破", Time: time.Hour},
		// Debug runs are never the best.
		{Date: date, Dungeon: "cave", Seed: "5", Depth: 30, Cleared: true, Result: "踏破", Time: time.Minute, Debug: true},
	}
	lines := recordLines(rs, map[string]string{"cave": "はじまりの洞窟"})
	if lines[0] != "自己ベスト" || !strings.Contains(lines[2], "はじまりの洞窟 30F 踏破  1:00:00") || !strings.HasSuffix(lines[2], "シード 4") {
		t.Errorf("best %q", lines[:3])
	}
	if !slices.Contains(lines, "すべての記録（5回、踏破2回）") {
		t.Errorf("lines %q", lines)
	}
	// The newest first: the debug run.
	if i := slices.Index(lines, "すべての記録（5回、踏破2回）"); !strings.Contains(lines[i+2], "[デバッグ]") {
		t.Errorf("all %q", lines[i:])
	}
}

// TestRecordOnEnd appends the record of a run when it ends.
func TestRecordOnEnd(t *testing.T) {
	dir := t.TempDir()
	p, _ := recordedGame(t, dir, "record")
	for p.game.Over == "" {
		if err := p.do("wait", nil); err != nil {
			t.Fatal(err)
		}
	}
	// Actions after the end add nothing.
	if err := p.do("wait", nil); err != nil {
		t.Fatal(err)
	}
	p.rec.Close()
	rs, err := readRecords(filepath.Join(dir, recordsFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Turns != p.game.Turn || rs[0].Result != p.game.Over || rs[0].Seed != "record" {
		t.Errorf("records %+v", rs)
	}
}
