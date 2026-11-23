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
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t31\t5-8\t6-10\t床\t0\t0\t通常\t0\t0\t0\t0\n"...)
		}, `floors 1-31 out of 1-30`},
		{"no monster on a floor", func(m fstest.MapFS) {
			m["header.txt"].Data = []byte("名前\tx\n階数\t31\n")
		}, `no monster on 31F`},
		{"levels out of order", func(m fstest.MapFS) {
			m["growth.txt"].Data = append(m["growth.txt"].Data, "普通\t40\t999999\t1\t1\t1\n"...)
		}, `level 40, want 31`},
		{"bad speed", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t光速\t-\t-\t-\t-\t-\n"...)
		}, `速度: want`},
		{"unknown ability", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\t飛行:1\t-\t-\t-\n"...)
		}, `能力: unknown ability "飛行"`},
		{"numbers of an ability", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\t怒り:33\t-\t-\t-\n"...)
		}, `能力: 怒り: want 2 numbers`},
		{"ability out of range", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\tふらふら:0\t-\t-\t-\n"...)
		}, `能力: ふらふら: chance % wants 1 to 100, not "0"`},
		{"ability twice", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\tふらふら:5 ふらふら:6\t-\t-\t-\n"...)
		}, `能力: the ability ふらふら twice`},
		{"drop of no item", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\t-\t-\tnoitem:5\t-\n"...)
		}, `落とし物: no item "noitem"`},
		{"drop rate", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\t-\t-\tお金:0\t-\n"...)
		}, `落とし物: want -, なし, or what:percent`},
		{"asleep above 100", func(m fstest.MapFS) {
			m["monsters.txt"].Data = append(m["monsters.txt"].Data, "x1\t鼠\tx\t1\t1\t1\t1\t1\t通常\t-\t-\t101\t-\t-\n"...)
		}, `眠り: 101 above 100`},
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
		}, `too few names: 1 for the 4 items of 草`},
		{"fake name of an item", func(m fstest.MapFS) {
			m["fake_names.txt"].Data = append(m["fake_names.txt"].Data, "草\t薬草\n"...)
		}, `"薬草" is the name of an item`},
		{"fake name of money", func(m fstest.MapFS) {
			m["fake_names.txt"].Data = append(m["fake_names.txt"].Data, "お金\t小判\n"...)
		}, `money is never unidentified`},
		{"bad brightness", func(m fstest.MapFS) {
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t1\t5-8\t6-10\t床\t0\t0\t真っ暗\t0\t0\t0\t0\n"...)
		}, `明るさ: want 通常, 明るい or 暗い`},
		{"unknown trap effect", func(m fstest.MapFS) {
			m["traps.txt"].Data = append(m["traps.txt"].Data, "x\tx\t爆弾\t1\n"...)
		}, `効果: unknown "爆弾"`},
		{"no trap on a floor", func(m fstest.MapFS) {
			m["trap_table.txt"].Data = []byte("開始\t終了\tID\t頻度\n2\t30\tjirai\t1\n")
		}, `no trap on 1F`},
		{"unknown kind in a table", func(m fstest.MapFS) {
			m["category_tables.txt"].Data = append(m["category_tables.txt"].Data, "床\t1\t30\t宝石\t1\n"...)
		}, `種類: no kind "宝石"`},
		{"items of an unknown table", func(m fstest.MapFS) {
			m["item_tables.txt"].Data = append(m["item_tables.txt"].Data, "宝\t1\t30\tkatana\t1\n"...)
		}, `表: no table "宝"`},
		{"a kind without items", func(m fstest.MapFS) {
			m["category_tables.txt"].Data = append(m["category_tables.txt"].Data, "草\t1\t30\t巻物\t1\n"...)
		}, `shops.txt: kusa: the item table "草" has no item of 巻物 on 2F`},
		{"an item never given", func(m fstest.MapFS) {
			m["item_tables.txt"].Data = append(m["item_tables.txt"].Data, "草\t1\t30\tkatana\t1\n"...)
		}, `never gives katana on 2F (no weight for 武器)`},
		{"money in a shop", func(m fstest.MapFS) {
			m["category_tables.txt"].Data = append(m["category_tables.txt"].Data, "草\t1\t30\tお金\t1\n"...)
			m["item_tables.txt"].Data = append(m["item_tables.txt"].Data, "草\t1\t30\tokane\t1\n"...)
		}, `the item table "草" of a shop has money`},
		{"a shop table not on its floors", func(m fstest.MapFS) {
			m["item_tables.txt"].Data = []byte(strings.ReplaceAll(string(m["item_tables.txt"].Data), "腕輪\t1\t30\t", "腕輪\t1\t10\t"))
			m["item_tables.txt"].Data = []byte(strings.ReplaceAll(string(m["item_tables.txt"].Data), "腕輪\t15\t30\t", "腕輪\t15\t20\t"))
			m["item_tables.txt"].Data = []byte(strings.ReplaceAll(string(m["item_tables.txt"].Data), "腕輪\t1\t14\t", "腕輪\t1\t10\t"))
		}, `shops.txt: ude: the item table "腕輪" has no item of 腕輪 on 11F`},
		{"unknown floor table", func(m fstest.MapFS) {
			m["floors.txt"].Data = []byte(strings.ReplaceAll(string(m["floors.txt"].Data), "\t床\t", "\t地下\t"))
		}, `道具表: no item table "地下"`},
		{"unknown shop table", func(m fstest.MapFS) {
			m["shops.txt"].Data = []byte(strings.ReplaceAll(string(m["shops.txt"].Data), "\tよろず\t", "\tなんでも\t"))
		}, `品表: no item table "なんでも"`},
		{"unknown guaranteed item", func(m fstest.MapFS) {
			m["guaranteed_items.txt"].Data = append(m["guaranteed_items.txt"].Data, "1\t3\tnothing\n"...)
		}, `ID: no item "nothing"`},
		{"bad debug rule", func(m fstest.MapFS) {
			m["rules.txt"].Data = append(m["rules.txt"].Data, "デバッグ\tはい\n"...)
		}, `デバッグ: want 可 or 不可`},
		{"shooting no arrow", func(m fstest.MapFS) {
			m["monsters.txt"].Data = []byte(strings.ReplaceAll(string(m["monsters.txt"].Data), "射撃:10:100:kiya:0", "射撃:10:100:bokutou:0"))
		}, `能力: 射撃: no arrow "bokutou"`},
		{"a drop of no kind", func(m fstest.MapFS) {
			m["monsters.txt"].Data = []byte(strings.ReplaceAll(string(m["monsters.txt"].Data), "\t矢:3\t", "\t剣:3\t"))
		}, `落とし物: no item "剣"`},
		{"hit rate above 100", func(m fstest.MapFS) {
			m["rules.txt"].Data = []byte(strings.ReplaceAll(string(m["rules.txt"].Data), "仲間の命中\t88", "仲間の命中\t101"))
		}, `仲間の命中: 101 above 100`},
		{"bad found trap rule", func(m fstest.MapFS) {
			m["rules.txt"].Data = append(m["rules.txt"].Data, "見つけた罠\t危険\n"...)
		}, `見つけた罠: want 安全 or 発動`},
		{"too many special floors", func(m fstest.MapFS) {
			m["floors.txt"].Data = append(m["floors.txt"].Data, "1\t1\t5-8\t6-10\t床\t0\t0\t通常\t60\t50\t0\t0\n"...)
		}, `大部屋 and 迷路: 110% together`},
		{"unknown seal", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t剣\tx\t武器\t1\t-\t0\t-\t炎\t2\t0\n"...)
		}, `印: no seal "炎"`},
		{"slots of a grass", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t草\tx\t草\t1\t-\t0\t-\t会\t2\t0\n"...)
		}, `印数: only weapons and shields`},
		{"money as a material", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t金\tx\tお金\t0\t-\t0\t-\t会\t0\t0\n"...)
		}, `印: money is no material`},
		{"a seal twice for weapons", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "会\t会心の印\t会心\t5\t武器\n"...)
		}, `the seal "会" again`},
		{"no slot for its seal", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t剣\tx\t武器\t1\t-\t0\t-\t会\t0\t0\n"...)
		}, `印数: no room`},
		{"a weapon seal on a shield", func(m fstest.MapFS) {
			m["items.txt"].Data = append(m["items.txt"].Data, "x\t盾\tx\t盾\t1\t-\t0\t-\t会\t2\t0\n"...)
		}, `印: the seal "会" goes on a 武器, not a 盾`},
		{"a seal on neither", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "炎\t炎の印\t攻撃\t1\t草\n"...)
		}, `付く先: want 武器 or 盾`},
		{"unknown seal effect", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "炎\t炎の印\t炎\t1\t武器\n"...)
		}, `効果: unknown "炎"`},
		{"seal of two characters", func(m fstest.MapFS) {
			m["seals.txt"].Data = append(m["seals.txt"].Data, "炎炎\t炎の印\t攻撃\t1\t武器\n"...)
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
