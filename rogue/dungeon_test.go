package rogue

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuiltinDungeons(t *testing.T) {
	ids := BuiltinDungeons()
	if !slices.Contains(ids, DefaultDungeon) {
		t.Fatalf("builtins %v", ids)
	}
	for _, id := range ids {
		d, err := LoadBuiltin(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if d.Name == "" || d.Floors < 1 || len(d.Monsters) == 0 || d.Hash == "" {
			t.Errorf("%s: %+v", id, d)
		}
	}
}

// copyFS returns the files of the built-in dungeon, to change in tests.
func copyFS(t *testing.T) fstest.MapFS {
	t.Helper()
	sub, err := fs.Sub(builtinFS, "dungeons/"+DefaultDungeon)
	if err != nil {
		t.Fatal(err)
	}
	m := fstest.MapFS{}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := fs.ReadFile(sub, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = &fstest.MapFile{Data: b}
	}
	return m
}

func TestDungeonErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(fstest.MapFS)
		want   string
	}{
		{"unknown key", func(m fstest.MapFS) {
			m["rules.txt"].Data = append(m["rules.txt"].Data, "速さ\t3\n"...)
		}, `rules.txt:`},
		{"half-width glyph", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\tx\tばつ\n"...)
		}, `items.txt:`},
		{"unknown monster", func(m fstest.MapFS) {
			m["monster_table.txt"].Data = append(m["monster_table.txt"].Data, "1\t3\tnobody\t1\n"...)
		}, `monster_table.txt:`},
		{"floor out of range", func(m fstest.MapFS) {
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t31\t5-8\t6-10\t0\t通常\t0\t0\t0\t0\n"...)
		}, `floors 1-31 out of 1-30`},
		{"no monster on a floor", func(m fstest.MapFS) {
			m["header.txt"].Data = []byte("名前\tx\n階数\t31\n")
		}, `no monster on 31F`},
		{"levels out of order", func(m fstest.MapFS) {
			m["growth.txt"].Data = append(m["growth.txt"].Data, "普通\t40\t999999\t1\t1\t1\n"...)
		}, `level 40, want 31`},
		{"bad speed", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t光速\t-\n"...)
		}, `速度: want`},
		{"unknown effect", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t腕\tx\t腕輪\t0\t飛行\t0\t-\t-\t0\t0\n"...)
		}, `効果: 腕輪 has no effect "飛行"`},
		{"effect of a grass", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t草\tx\t草\t0\t回復\t0\t-\t-\t0\t0\n"...)
		}, `効果: 草 has no effect "回復"`},
		{"charges of a grass", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t草\tx\t草\t0\t-\t0\t3\t-\t0\t0\n"...)
		}, `回数: only staffs, arrows and pots`},
		{"bad charges", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t杖\tx\t杖\t0\t-\t0\t5-2\t-\t0\t0\n"...)
		}, `回数: want min-max`},
		{"staff without charges", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t杖\tx\t杖\t0\t-\t0\t-\t-\t0\t0\n"...)
		}, `回数: 杖 needs some`},
		{"too few fake names", func(m fstest.MapFS) {
			m["fake_names.txt"].Data = []byte("種類\t名前\n草\t赤い草\n")
		}, `too few names: 1 for the 3 items of 草`},
		{"fake name of an item", func(m fstest.MapFS) {
			m["fake_names.txt"].Data = append(m["fake_names.txt"].Data, "草\t薬草\n"...)
		}, `"薬草" is the name of an item`},
		{"fake name of money", func(m fstest.MapFS) {
			m["fake_names.txt"].Data = append(m["fake_names.txt"].Data, "お金\t小判\n"...)
		}, `money is never unidentified`},
		{"bad brightness", func(m fstest.MapFS) {
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t1\t5-8\t6-10\t0\t真っ暗\t0\t0\t0\t0\n"...)
		}, `明るさ: want 通常, 明るい or 暗い`},
		{"unknown trap effect", func(m fstest.MapFS) {
			m["traps.txt"].Data = append(m["traps.txt"].Data, "x\tx\t爆弾\t1\n"...)
		}, `効果: unknown "爆弾"`},
		{"no trap on a floor", func(m fstest.MapFS) {
			m["trap_table.txt"].Data = []byte("開始\t終了\tID\t頻度\n2\t30\tjirai\t1\n")
		}, `no trap on 1F`},
		{"bad debug rule", func(m fstest.MapFS) {
			m["rules.txt"].Data = append(m["rules.txt"].Data, "デバッグ\tはい\n"...)
		}, `デバッグ: want 可 or 不可`},
		{"bad found trap rule", func(m fstest.MapFS) {
			m["rules.txt"].Data = append(m["rules.txt"].Data, "見つけた罠\t危険\n"...)
		}, `見つけた罠: want 安全 or 発動`},
		{"too many special floors", func(m fstest.MapFS) {
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t1\t5-8\t6-10\t0\t通常\t60\t50\t0\t0\n"...)
		}, `大部屋 and 迷路: 110% together`},
		{"unknown seal", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t剣\tx\t武器\t1\t-\t0\t-\t炎\t2\t0\n"...)
		}, `印: no seal "炎"`},
		{"seal of a grass", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t草\tx\t草\t1\t-\t0\t-\t会\t2\t0\n"...)
		}, `印: only weapons and shields`},
		{"no slot for its seal", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t剣\tx\t武器\t1\t-\t0\t-\t会\t0\t0\n"...)
		}, `印数: no room`},
		{"unknown seal effect", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "炎\t炎の印\t炎\t1\n"...)
		}, `効果: unknown "炎"`},
		{"seal of two characters", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "炎炎\t炎の印\t攻撃\t1\n"...)
		}, `印: want one character`},
		{"cracking grass", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t草\tx\t草\t0\t-\t5\t-\t-\t0\t0\n"...)
		}, `ひび: only bracelets`},
		{"missing file", func(m fstest.MapFS) {
			delete(m, "growth.txt")
		}, `growth.txt`},
	} {
		m := copyFS(t)
		tc.change(m)
		_, err := LoadDungeon(m, "x")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
			continue
		}
		var de *DungeonError
		if !errors.As(err, &de) {
			t.Errorf("%s: %T", tc.name, err)
		}
	}
}

func TestDungeonErrorLine(t *testing.T) {
	m := copyFS(t)
	lines := strings.Count(string(m["items.txt"].Data), "\n")
	m["items.txt"].Data = append(m["items.txt"].Data, "x\tx\tばつ\n"...)
	_, err := LoadDungeon(m, "x")
	var de *DungeonError
	if !errors.As(err, &de) || de.File != "items.txt" || de.Line != lines+1 {
		t.Errorf("%v, want items.txt:%d", err, lines+1)
	}
}

// TestDefinitionChangesFloors: the same seed with another definition makes
// other floors.
func TestDefinitionChangesFloors(t *testing.T) {
	m := copyFS(t)
	a, err := LoadDungeon(m, "x")
	if err != nil {
		t.Fatal(err)
	}
	m["header.txt"].Data = append(m["header.txt"].Data, "# a comment\n"...)
	b, err := LoadDungeon(m, "x")
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash == b.Hash {
		t.Fatal("the same hash")
	}
	if slices.Equal(NewGame(a, "s").Level.Tiles, NewGame(b, "s").Level.Tiles) {
		t.Error("the same floors")
	}
}
