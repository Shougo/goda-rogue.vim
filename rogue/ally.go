package rogue

import "slices"

// Order is what the player told an ally to do.
type Order string

// The orders of an ally. It follows until told otherwise.
const (
	// OrderFollow: stay with the player, and fight the enemies near it.
	OrderFollow Order = "ついてきて"
	// OrderFree: go where it likes, and fight the enemies it sees.
	OrderFree Order = "自由に"
	// OrderWait: stay where it is, and fight the enemies next to it.
	OrderWait Order = "待って"
)

// orders are the orders, as the prompt lists them.
var orders = []Order{OrderFollow, OrderFree, OrderWait}

// promptOrder asks what to tell an ally.
const promptOrder = "order"

// order returns what m was told (OrderFollow at first).
func (m *Monster) order() Order {
	if m.Order == "" {
		return OrderFollow
	}
	return m.Order
}

// allies returns the allies on the floor.
func (g *Game) allies() []*Monster {
	var as []*Monster
	for _, m := range g.Monsters {
		if m.Ally {
			as = append(as, m)
		}
	}
	return as
}

// askOrder asks what to tell the ally m (the one it follows now is
// marked), unless the player's mouth is sealed.
func (g *Game) askOrder(m *Monster) {
	if g.Sealed {
		g.say("口が封じられていて命令できない。")
		return
	}
	p := &Prompt{Title: m.Name + "への命令", kind: promptOrder}
	for _, o := range orders {
		c := string(o)
		if o == m.order() {
			c += " [今]"
		}
		p.Choices = append(p.Choices, c)
	}
	g.Prompt = p
	g.ordered = m
}

// giveOrder tells the ally of the prompt the order n. It costs no turn.
func (g *Game) giveOrder(n int) {
	m := g.ordered
	g.ordered = nil
	if m == nil || !m.Ally || !slices.Contains(g.Monsters, m) {
		return
	}
	m.Order, m.hasTarget = orders[n], false
	g.say(m.Name + "に「" + string(orders[n]) + "」と伝えた。")
}

// recruit sends a bolt that makes the monster it hits an ally: not a
// shopkeeper or a guard, and not past the dungeon's limit of allies.
func (g *Game) recruit() {
	m := g.bolt()
	if m == nil {
		return
	}
	switch {
	case m.Ally:
		g.say(m.Name + "はもう仲間だ。")
	case m.Keeper || m.Def.Kind != "":
		g.say(m.Name + "には効かなかった。")
	case len(g.allies()) >= g.Dungeon.Rules.MaxAllies:
		g.say("これ以上仲間を連れて行けない。")
	default:
		m.Ally, m.Order = true, OrderFollow
		m.Dozing, m.Hunter, m.Angry, m.House, m.hasTarget = false, false, false, false, false
		m.Asleep, m.Confused = 0, 0
		g.say(m.Name + "が仲間になった！")
	}
}

// comesAlong says whether the ally m goes down the stairs with the
// player: next to it, in its room, or at an entrance of its room (on its
// way in: to the eye, it is there).
func (g *Game) comesAlong(m *Monster) bool {
	if chebyshev(m.Pos, g.Player) <= 1 {
		return true
	}
	room := g.Level.RoomAt(g.Player)
	if room < 0 {
		return false
	}
	if g.Level.RoomAt(m.Pos) == room {
		return true
	}
	for _, d := range around {
		if g.Level.RoomAt(m.Pos.Add(d)) == room && g.clearFrom(m.Pos, d) {
			return true
		}
	}
	return false
}
