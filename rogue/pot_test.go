package rogue

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// pot gives the player the pot id, holding n.
func pot(g *Game, id string, n int) *Item {
	p := carry(g, id, 0)
	p.Charges = n
	return p
}

// chooseLabel chooses the choice c of the prompt.
func chooseLabel(t *testing.T, g *Game, c string) {
	t.Helper()
	if g.Prompt == nil {
		t.Fatalf("no prompt for %q", c)
	}
	i := slices.Index(g.Prompt.Choices, c)
	if i < 0 {
		t.Fatalf("no %q in %q", c, g.Prompt.Choices)
	}
	if err := g.Do("choose", []string{fmt.Sprint(i)}); err != nil {
		t.Fatal(err)
	}
}

func TestKeepingPot(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "hozon", 2)
	grass := carry(g, "yakusou", 0)
	if g.Label(p) != "壺 保存の壺[2]" {
		t.Errorf("label %q", g.Label(p))
	}
	command(t, g, p, commandPutIn)
	turn := g.Turn
	chooseLabel(t, g, g.Entry(grass))
	if !slices.Equal(p.Contents, []*Item{grass}) || slices.Contains(g.Inventory, grass) || g.Turn != turn+1 {
		t.Fatalf("contents %v, turn %d", p.Contents, g.Turn)
	}
	if g.Label(p) != "壺 保存の壺[1]【草】" || g.Message != "薬草を保存の壺に入れた。" {
		t.Errorf("label %q, message %q", g.Label(p), g.Message)
	}
	command(t, g, p, commandLook)
	chooseLabel(t, g, "草   薬草")
	if !slices.Equal(g.Prompt.Choices, []string{commandDrink, commandThrow, commandTakeOut, commandInfo}) {
		t.Fatalf("choices %q", g.Prompt.Choices)
	}
	chooseLabel(t, g, commandTakeOut)
	if len(p.Contents) != 0 || !slices.Contains(g.Inventory, grass) || g.Label(p) != "壺 保存の壺[2]" {
		t.Errorf("contents %v, label %q", p.Contents, g.Label(p))
	}

	// Full.
	p.Contents = []*Item{carry(g, "onigiri", 0), carry(g, "onigiri", 0)}
	command(t, g, p, commandPutIn)
	if g.Prompt != nil || g.Message != "保存の壺はいっぱいだ。" {
		t.Errorf("prompt %+v, message %q", g.Prompt, g.Message)
	}
}

// TestUseInPot uses items kept in a pot of keeping where they are.
func TestUseInPot(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "hozon", 3)
	grass := carry(g, "yakusou", 0)
	food := carry(g, "onigiri", 0)
	g.discard(grass)
	g.discard(food)
	p.Contents = []*Item{grass, food}
	g.HP = 1

	turn := g.Turn
	command(t, g, p, commandLook)
	if g.Prompt.Title != "保存の壺の中" {
		t.Errorf("title %q", g.Prompt.Title)
	}
	chooseLabel(t, g, g.Entry(grass))
	chooseLabel(t, g, commandDrink)
	if g.HP == 1 || !slices.Equal(p.Contents, []*Item{food}) || g.Turn != turn+1 {
		t.Errorf("HP %d, contents %v, turn %d", g.HP, p.Contents, g.Turn)
	}
	if g.Label(p) != "壺 保存の壺[2]【食】" {
		t.Errorf("label %q", g.Label(p))
	}

	// Thrown from the pot.
	command(t, g, p, commandLook)
	chooseLabel(t, g, g.Entry(food))
	chooseLabel(t, g, commandThrow)
	if len(p.Contents) != 0 || len(g.Items) != 1 {
		t.Errorf("contents %v, items %v", p.Contents, g.Items)
	}

	// Nothing in it.
	command(t, g, p, commandLook)
	if g.Prompt != nil || g.Message != "保存の壺には何も入っていない。" {
		t.Errorf("prompt %+v, message %q", g.Prompt, g.Message)
	}
}

func TestSynthesis(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "gousei", 4)
	base := carry(g, "katana", 1)
	other := carry(g, "kaishin", 2)
	other.Seals = []string{"会", "必"}
	command(t, g, base, commandEquip)
	put := func(it *Item) {
		t.Helper()
		command(t, g, p, commandPutIn)
		chooseLabel(t, g, g.Entry(it))
	}
	put(base)
	if base.Equipped {
		t.Error("a weapon in a pot is still equipped")
	}
	put(other)
	if len(p.Contents) != 1 || base.Upgrade != 3 || !slices.Equal(base.Seals, []string{"会", "必"}) {
		t.Fatalf("contents %d, upgrade %d, seals %v", len(p.Contents), base.Upgrade, base.Seals)
	}
	if !strings.Contains(g.Message, "カタナと会心剣が合成された！") {
		t.Errorf("message %q", g.Message)
	}
	// No more seals than the slots, no seal twice.
	more := carry(g, "seidou", 0)
	more.Seals = []string{"錆"}
	put(more) // A shield: kept apart.
	sword := carry(g, "bokutou", 0)
	sword.Seals = []string{"会", "攻", "反"}
	put(sword)
	if !slices.Equal(base.Seals, []string{"会", "必", "攻", "反"}) || len(p.Contents) != 2 {
		t.Errorf("seals %v, contents %d", base.Seals, len(p.Contents))
	}
	if g.describe(base) != "カタナ+3【会必攻反】" {
		t.Errorf("label %q", g.describe(base))
	}
	// Four items went in, two merged: the pot is full.
	if n := p.Charges - len(p.Contents); n != 0 {
		t.Errorf("room for %d more, want 0", n)
	}
	command(t, g, p, commandPutIn)
	if g.Prompt != nil || !strings.Contains(g.Message, "いっぱいだ") {
		t.Errorf("a full pot: prompt %+v, message %q", g.Prompt, g.Message)
	}

	// Nothing comes out of it, nor is used in it.
	command(t, g, p, commandLook)
	chooseLabel(t, g, g.Entry(base))
	if !slices.Equal(g.Prompt.Choices, []string{commandTakeOut, commandInfo}) {
		t.Fatalf("choices %q", g.Prompt.Choices)
	}
	chooseLabel(t, g, commandTakeOut)
	if g.Prompt != nil || !strings.Contains(g.Message, "からは出せない。") {
		t.Errorf("message %q", g.Message)
	}
}

// TestMixedSynthesis: a material gives a weapon its seal for weapons and
// a shield its seal for shields (one glyph, two effects), by mixed
// synthesis; a pot of synthesis merges items of a kind only, so a
// material stays in it as it is.
func TestMixedSynthesis(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "gousei", 3)
	sword := carry(g, "katana", 0)
	grass := carry(g, "chikarakusa", 0)
	for _, it := range []*Item{sword, grass} {
		command(t, g, p, commandPutIn)
		chooseLabel(t, g, g.Entry(it))
	}
	if len(p.Contents) != 2 || len(sword.Seals) != 0 {
		t.Fatalf("pot: contents %d, seals %v", len(p.Contents), sword.Seals)
	}

	if !g.merge(sword, carry(g, "chikarakusa", 0), true) || !slices.Equal(sword.Seals, []string{"力"}) {
		t.Fatalf("seals %v", sword.Seals)
	}
	if !strings.Contains(g.Message, "カタナとちからの草が合成された！") {
		t.Errorf("message %q", g.Message)
	}
	shield := carry(g, "tetsutate", 0)
	if !g.merge(shield, carry(g, "chikarakusa", 0), true) || !slices.Equal(shield.Seals, []string{"力"}) {
		t.Fatalf("seals %v", shield.Seals)
	}
	if g.merge(shield, carry(g, "yakusou", 0), true) {
		t.Error("merged a grass of no seal")
	}

	// The two sides: attack on the weapon, a counter on the shield.
	g.Inventory = nil
	sword.Equipped, shield.Equipped = true, true
	g.Inventory = append(g.Inventory, sword, shield)
	if n := g.sealStrength(SealAttack); n != 2 {
		t.Errorf("attack %d", n)
	}
	if n := g.sealStrength(SealCounter); n != 15 {
		t.Errorf("counter %d", n)
	}
	if n := g.sealStrength(SealDefense); n != 0 {
		t.Errorf("defense %d", n)
	}
	info := strings.Join(g.info(carry(g, "chikarakusa", 0)), "\n")
	for _, want := range []string{"武器に異種合成すると【力】力の印：攻撃力 +2。", "盾に異種合成すると【力】力の印：15% の確率"} {
		if !strings.Contains(info, want) {
			t.Errorf("info %q, want %q", info, want)
		}
	}
}

func TestMendBracelet(t *testing.T) {
	g, _ := roomGame(t, 4, 3)
	p := pot(g, "gousei", 3)
	b := carry(g, "kehai", 0)
	b.Cracks = 2
	command(t, g, p, commandPutIn)
	chooseLabel(t, g, g.Entry(b))
	other := carry(g, "kehai", 0)
	command(t, g, p, commandPutIn)
	chooseLabel(t, g, g.Entry(other))
	if b.Cracks != 0 || len(p.Contents) != 1 || !strings.Contains(g.Message, "ひびが直った") {
		t.Errorf("cracks %d, contents %d, message %q", b.Cracks, len(p.Contents), g.Message)
	}
	if n := p.Charges - len(p.Contents); n != 1 {
		t.Errorf("room for %d more, want 1 (two of 3 taken)", n)
	}
}

func TestSmashPot(t *testing.T) {
	g, r := roomGame(t, 8, 3)
	g.Facing = "right"
	p := pot(g, "hozon", 3)
	p.Contents = []*Item{{Def: g.Dungeon.Item("yakusou"), Glyph: "草"}, {Def: g.Dungeon.Item("onigiri"), Glyph: "食"}}
	command(t, g, p, commandThrow)
	if len(g.Items) != 2 || !strings.Contains(g.Message, "保存の壺は割れた。") {
		t.Errorf("items %v, message %q", g.Items, g.Message)
	}
	for _, it := range g.Items {
		if it.Def.Category == CategoryPot || it.Pos.X <= r.X {
			t.Errorf("%s at %v", it.Def.ID, it.Pos)
		}
	}
}

func TestSeals(t *testing.T) {
	g, r := roomGame(t, 6, 3)
	g.HP, g.MaxHP = 9999, 9999
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.SpawnTurns = 1 << 30 })
	sword := carry(g, "tetsuken", 0)
	shield := carry(g, "tetsutate", 0)
	sword.Equipped, shield.Equipped = true, true
	attack, defense := g.Attack(), g.Defense()
	sword.Seals = []string{"攻", "会", "必"}
	shield.Seals = []string{"防", "反", "錆", "腹"}
	if g.Attack() != attack+3 || g.Defense() != defense+3 {
		t.Errorf("attack %d, defense %d", g.Attack(), g.Defense())
	}

	// 必中 and 会心: every attack hits, some twice as hard.
	g.Dungeon = withRules(g.Dungeon, func(r *Rules) { r.PlayerHitRate = 0 })
	g.Facing = "right"
	m := place(g, kind("nezumi4"), Point{r.X + 1, r.Y})
	m.HP, m.MaxHP = 99999, 99999
	crit, counter := 0, 0
	for range 100 {
		hp := m.HP
		if err := g.Do("attack", nil); err != nil {
			t.Fatal(err)
		}
		if m.HP == hp && !strings.Contains(g.Message, "反撃") {
			t.Fatalf("missed: %q", g.Message)
		}
		if strings.Contains(g.Message, "会心の一撃！") {
			crit++
		}
		if strings.Contains(g.Message, "反撃した！") {
			counter++
		}
	}
	if crit == 0 || crit == 100 || counter == 0 {
		t.Errorf("%d critical hits, %d counters", crit, counter)
	}

	// サビよけ.
	g.Monsters = nil
	trap(g, "sabi", Point{r.X + 1, r.Y})
	move(t, g, "right")
	if sword.Upgrade != 0 || !strings.Contains(g.Message, "サビよけの印で守られた。") {
		t.Errorf("upgrade %d, message %q", sword.Upgrade, g.Message)
	}

	// ハラヘラズ: half as hungry.
	g.Fullness, g.hunger = 50, 0
	wait(t, g, g.Dungeon.Rules.HungerTurns*10)
	if g.Fullness != 45 {
		t.Errorf("fullness %d", g.Fullness)
	}
}
