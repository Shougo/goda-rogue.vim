package rogue

import (
	"slices"
	"testing"
)

func TestInfo(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	sword := carry(g, "bokutou", 2)
	sword.Seals = []string{"会"}
	turn := g.Turn
	command(t, g, sword, commandInfo)
	want := []string{
		"木刀+2【会】（武器）",
		"攻撃力 5（基本 3、修正値 +2）",
		"印 1/2",
		"【会】会心の印：25% の確率で会心の一撃（2 倍のダメージ）。",
	}
	if g.Prompt == nil || !slices.Equal(g.Prompt.Choices, want) || g.Turn != turn {
		t.Fatalf("prompt %+v, turn %d", g.Prompt, g.Turn)
	}
	if err := g.Do("choose", []string{"0"}); err != nil || g.Prompt != nil {
		t.Errorf("choose: %v, %+v", err, g.Prompt)
	}

	// An upgrade value not known yet.
	sword.Known = false
	if got := g.info(sword)[1]; got != "攻撃力 3+?（修正値は装備するか識別すると分かる）" {
		t.Errorf("unknown upgrade: %q", got)
	}

	for _, c := range []struct {
		id   string
		want string
	}{
		{"yakusou", "飲むと、HP が 25 回復する。HP が満タンなら最大 HP が上がる。"},
		{"akari", "読むと、フロア全体が明るくなり、地形と道具と敵が見える。"},
		{"onigiri", "食べると、満腹度が 50 回復する。"},
	} {
		it := carry(g, c.id, 0)
		if got := g.info(it); len(got) != 2 || got[1] != c.want {
			t.Errorf("%s: %q", c.id, got)
		}
	}

	// An unidentified kind tells nothing.
	g.identified["yakusou"] = false
	grass := carry(g, "yakusou", 0)
	if got := g.info(grass); len(got) != 2 || got[1] != "正体はまだ分からない。使うか識別すると分かる。" {
		t.Errorf("unidentified: %q", got)
	}
}
