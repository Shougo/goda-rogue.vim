// Command analyze measures the balance of a dungeon: bots play many runs
// of it, and it prints how they went: clears, where and why they fell,
// the HP on arriving on each floor, the items found and used.
//
//	go run ./cmd/analyze -dungeon cave -runs 200
//
// The runs are reproducible: run i of a bot has a seed made from the prefix
// and i (runSeed; the same floors for every bot) and the bot's randomness
// from i. The seed of a run is in the -tsv output.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/Shougo/goda-rogue.vim/rogue"
)

func main() {
	dungeon := flag.String("dungeon", "cave", "the dungeon (a built-in one)")
	runs := flag.Int("runs", 100, "the runs of each bot")
	bots := flag.String("bots", "careful,reckless", "the bots, by style, separated by commas")
	seed := flag.String("seed", "analyze", "the prefix of the seeds of the runs")
	actions := flag.Int("actions", 50000, "the most actions of a run")
	jobs := flag.Int("j", runtime.NumCPU(), "the runs played at once")
	tsv := flag.Bool("tsv", false, "print one line a run, tab-separated, instead of the summary")
	look := flag.Bool("look", false, "the bots look ahead at the hopeless moments (Bot.LookAhead)")
	trace := flag.Int("trace", -1, "a run: print each choice of the first of the bots in run N (of -seed), instead of the summary")
	compare := flag.String("compare", "", "a replay file: print where the first of the bots would have done another kind of thing than its player, instead of playing")
	flag.Parse()
	if *compare != "" {
		style, ok := rogue.BotStyles[strings.Split(*bots, ",")[0]]
		if !ok {
			fmt.Fprintf(os.Stderr, "analyze: unknown bot %q\n", *bots)
			os.Exit(2)
		}
		lines, err := rogue.CompareReport(*compare, style)
		if err != nil {
			fmt.Fprintln(os.Stderr, "analyze:", err)
			os.Exit(1)
		}
		fmt.Println(strings.Join(lines, "\n"))
		return
	}

	d, err := rogue.LoadBuiltin(*dungeon)
	if err != nil {
		fmt.Fprintln(os.Stderr, "analyze:", err)
		os.Exit(1)
	}
	if *trace >= 0 {
		name := strings.Split(*bots, ",")[0]
		s, ok := rogue.BotStyles[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "analyze: no bot %q\n", name)
			os.Exit(2)
		}
		b := rogue.NewBot(s, uint64(*trace))
		b.LookAhead = *look
		fmt.Println(rogue.TraceBot(d, runSeed(*seed, *trace), b, *actions, func(l string) { fmt.Println(l) }))
		return
	}
	var styles []rogue.BotStyle
	for _, name := range strings.Split(*bots, ",") {
		s, ok := rogue.BotStyles[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "analyze: no bot %q\n", name)
			os.Exit(2)
		}
		styles = append(styles, s)
	}
	if *tsv {
		fmt.Println("bot\tseed\tdepth\tcause\tover\tturns\tlevel\tkills\tdamage\tunfair")
	}
	for _, s := range styles {
		rs, err := play(d, s, *seed, *runs, *actions, *jobs, *look)
		if err != nil {
			fmt.Fprintln(os.Stderr, "analyze:", err)
			os.Exit(1)
		}
		if *tsv {
			for _, r := range rs {
				fmt.Printf("%v\t%s\t%d\t%s\t%s\t%d\t%d\t%d\t%d\t%v\n",
					s, r.Seed, r.Depth, r.Cause, r.Over, r.Turns, r.Level, r.Kills, r.Damage, r.Unfair)
			}
			continue
		}
		summarize(os.Stdout, d, s, rs)
	}
}

// play plays n runs of d by bots of style s, jobs at once.
func play(d *rogue.Dungeon, s rogue.BotStyle, seed string, n, actions, jobs int, look bool) ([]rogue.BotRun, error) {
	rs := make([]rogue.BotRun, n)
	errs := make([]error, n)
	next := make(chan int)
	var wg sync.WaitGroup
	for range max(jobs, 1) {
		wg.Go(func() {
			for i := range next {
				b := rogue.NewBot(s, uint64(i))
				b.LookAhead = look
				rs[i], errs[i] = rogue.PlayBot(d, runSeed(seed, i), b, actions)
			}
		})
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return rs, nil
}

// runSeed returns the seed of the run i of the runs of the prefix seed:
// the prefix, and digits that have nothing in common from a run to the
// next. Seeds alike ("analyze0", "analyze1", ...) make runs alike: the
// share of clears of 300 such runs moved by 5 points from a prefix to
// another, twice what chance alone gives.
func runSeed(seed string, i int) string {
	sum := sha256.Sum256(fmt.Append(nil, seed, "\x00", i))
	return fmt.Sprintf("%s-%x", seed, sum[:8])
}

// summarize prints the summary of the runs rs of the bot s.
func summarize(w io.Writer, d *rogue.Dungeon, s rogue.BotStyle, rs []rogue.BotRun) {
	n := len(rs)
	pct := func(k int) string { return fmt.Sprintf("%d (%.0f%%)", k, 100*float64(k)/float64(max(n, 1))) }
	count := func(f func(rogue.BotRun) bool) int {
		k := 0
		for _, r := range rs {
			if f(r) {
				k++
			}
		}
		return k
	}
	mean := func(f func(rogue.BotRun) int) float64 {
		t := 0
		for _, r := range rs {
			t += f(r)
		}
		return float64(t) / float64(max(n, 1))
	}

	fmt.Fprintf(w, "== %s, bot %v, %d runs\n", d.Name, s, n)
	fmt.Fprintf(w, "cleared       %s\n", pct(count(func(r rogue.BotRun) bool { return r.Cleared })))
	depths := make([]int, n)
	for i, r := range rs {
		depths[i] = r.Depth
	}
	slices.Sort(depths)
	if n > 0 {
		fmt.Fprintf(w, "floor reached mean %.1f, median %d, max %d (of %d)\n",
			mean(func(r rogue.BotRun) int { return r.Depth }), depths[n/2], depths[n-1], d.Floors)
	}
	fmt.Fprintf(w, "level         mean %.1f;  turns mean %.0f;  kills mean %.1f;  damage taken mean %.0f\n",
		mean(func(r rogue.BotRun) int { return r.Level }), mean(func(r rogue.BotRun) int { return r.Turns }),
		mean(func(r rogue.BotRun) int { return r.Kills }), mean(func(r rogue.BotRun) int { return r.Damage }))

	fmt.Fprintln(w, "ended by")
	for _, c := range []string{"cleared", "monster", "trap", "hunger", "wind", "unfinished"} {
		if k := count(func(r rogue.BotRun) bool { return r.Cause == c }); k > 0 {
			fmt.Fprintf(w, "  %-10s %s\n", c, pct(k))
		}
	}
	fmt.Fprintf(w, "unfair deaths %s (from full HP in one action, or within 10 turns of arriving)\n",
		pct(count(func(r rogue.BotRun) bool { return r.Unfair })))

	killers := map[string]int{}
	for _, r := range rs {
		if r.Cause == "monster" {
			killers[strings.TrimSuffix(r.Over, "に倒された")]++
		}
	}
	fmt.Fprintln(w, "killed by (top 8)")
	for _, k := range top(killers, 8) {
		fmt.Fprintf(w, "  %-10s %d\n", k, killers[k])
	}

	// By floor: how many reached it, fell on it, and the HP on arriving.
	fmt.Fprintln(w, "floor  reached  fell  HP on arriving (mean %)  turns on it (mean)  answers picked up / used (mean)")
	for f := 1; f <= d.Floors; f++ {
		reached, fell, hp, turns, left, picked, used := 0, 0, 0.0, 0, 0, 0, 0
		for _, r := range rs {
			for i, fl := range r.Floors {
				if fl.Depth != f {
					continue
				}
				reached++
				picked += answers(fl.Picked)
				used += answers(fl.Used)
				hp += 100 * float64(fl.HP) / float64(max(fl.MaxHP, 1))
				if i+1 < len(r.Floors) {
					turns += r.Floors[i+1].Turn - fl.Turn
					left++
				}
			}
			if !r.Cleared && r.Depth == f {
				fell++
			}
		}
		if reached == 0 {
			break
		}
		t := "-"
		if left > 0 {
			t = fmt.Sprintf("%.0f", float64(turns)/float64(left))
		}
		fmt.Fprintf(w, "%4d   %6d  %4d  %5.0f  %-4s  %5s  %5.2f / %4.2f\n", f, reached, fell, hp/float64(reached),
			strings.Repeat("#", fell*40/max(n, 1)), t,
			float64(picked)/float64(reached), float64(used)/float64(reached))
	}

	summarizeParts(w, d, rs)

	fmt.Fprintln(w, "items a run (picked up / used)")
	for _, c := range rogue.Categories() {
		p := mean(func(r rogue.BotRun) int { return r.Picked[c] })
		u := mean(func(r rogue.BotRun) int { return r.Used[c] })
		if p > 0 || u > 0 {
			fmt.Fprintf(w, "  %-6s %5.1f / %4.1f\n", c, p, u)
		}
	}
	summarizeEnds(w, rs)
	fmt.Fprintln(w)
}

// parts are how many parts of the dungeon summarizeParts splits it into.
const parts = 3

// summarizeParts prints the chance of dying in each part of the dungeon,
// for a run that reached the part: the deep floors should be at least as
// deadly as the first ones, or the difficulty sits in the wrong place.
func summarizeParts(w io.Writer, d *rogue.Dungeon, rs []rogue.BotRun) {
	fmt.Fprintln(w, "deaths by part of the dungeon (fell / reached its first floor)")
	for i := range parts {
		from, to := d.Floors*i/parts+1, d.Floors*(i+1)/parts
		reached, fell := 0, 0
		for _, r := range rs {
			if r.Depth < from && !r.Cleared {
				continue
			}
			reached++
			if !r.Cleared && r.Depth <= to {
				fell++
			}
		}
		fmt.Fprintf(w, "  %2d-%2dF  %3d / %3d  %5.1f%%\n", from, to, fell, reached,
			100*float64(fell)/float64(max(reached, 1)))
	}
}

// answerCategories are the categories of the items that answer a danger
// (the answers counted per floor).
var answerCategories = []rogue.Category{rogue.CategoryGrass, rogue.CategoryScroll, rogue.CategoryStaff}

// answers returns the count of answers in counts.
func answers(counts map[rogue.Category]int) int {
	n := 0
	for _, c := range answerCategories {
		n += counts[c]
	}
	return n
}

// lastFloors are the floors before a death whose items used are shown.
const lastFloors = 3

// summarizeEnds prints how the runs ended, as the items go: the items
// left at a clear and at a death, the chance of dying the bot reckoned at
// its last choice before a death by a monster, and the items used on the
// floors before a death.
func summarizeEnds(w io.Writer, rs []rogue.BotRun) {
	var cleared, died []rogue.BotRun
	for _, r := range rs {
		switch r.Cause {
		case "cleared":
			cleared = append(cleared, r)
		case "monster", "trap", "hunger", "wind":
			died = append(died, r)
		}
	}
	meanOf := func(rs []rogue.BotRun, f func(rogue.BotRun) int) float64 {
		t := 0
		for _, r := range rs {
			t += f(r)
		}
		return float64(t) / float64(max(len(rs), 1))
	}
	fmt.Fprintf(w, "items left at the end (mean; %d cleared / %d died)\n", len(cleared), len(died))
	for _, c := range rogue.Categories() {
		a := meanOf(cleared, func(r rogue.BotRun) int { return r.Left[c] })
		b := meanOf(died, func(r rogue.BotRun) int { return r.Left[c] })
		if a > 0 || b > 0 {
			fmt.Fprintf(w, "  %-6s %4.1f / %4.1f\n", c, a, b)
		}
	}

	fmt.Fprintf(w, "  %-6s %4.1f / %4.1f  (healing grasses and room in pots of healing)\n", "heals",
		meanOf(cleared, func(r rogue.BotRun) int { return r.HealsLeft }), meanOf(died, func(r rogue.BotRun) int { return r.HealsLeft }))
	fmt.Fprintf(w, "heals used (mean): %.1f / %.1f\n",
		meanOf(cleared, func(r rogue.BotRun) int { return r.Heals }), meanOf(died, func(r rogue.BotRun) int { return r.Heals }))

	// The chance of dying at the last choice: a risk taken, or a surprise.
	var monster []rogue.BotRun
	for _, r := range died {
		if r.Cause == "monster" {
			monster = append(monster, r)
		}
	}
	fmt.Fprintf(w, "deaths by monsters (%d), by the chance of dying the bot reckoned: at the start of the last fight / at the last choice\n", len(monster))
	for _, b := range []struct {
		name string
		in   func(float64) bool
	}{
		{"no enemy next (surprise)", func(p float64) bool { return p == 0 }},
		{"below 5%", func(p float64) bool { return p > 0 && p < 0.05 }},
		{"5-20%", func(p float64) bool { return p >= 0.05 && p < 0.20 }},
		{"20-50%", func(p float64) bool { return p >= 0.20 && p < 0.50 }},
		{"50% or more", func(p float64) bool { return p >= 0.50 }},
	} {
		start, last := 0, 0
		for _, r := range monster {
			if b.in(r.FightRisk) {
				start++
			}
			if b.in(r.Risk) {
				last++
			}
		}
		fmt.Fprintf(w, "  %-26s %3d / %3d\n", b.name, start, last)
	}

	// The items used on the last floors before a death.
	fmt.Fprintf(w, "items used on the last %d floors before a death (mean)\n", lastFloors)
	none := 0
	for _, c := range rogue.Categories() {
		u := meanOf(died, func(r rogue.BotRun) int {
			n := 0
			for _, f := range r.Floors[max(len(r.Floors)-lastFloors, 0):] {
				n += f.Used[c]
			}
			return n
		})
		if u > 0 {
			fmt.Fprintf(w, "  %-6s %4.1f\n", c, u)
		}
	}
	for _, r := range died {
		n := 0
		for _, f := range r.Floors[max(len(r.Floors)-lastFloors, 0):] {
			n += answers(f.Used)
		}
		if n == 0 {
			none++
		}
	}
	fmt.Fprintf(w, "  deaths with no answer used on them: %d of %d\n", none, len(died))
}

// top returns the n keys of m with the largest counts.
func top(m map[string]int, n int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys[:min(n, len(keys))]
}
