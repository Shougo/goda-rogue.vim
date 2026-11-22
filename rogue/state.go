package rogue

// States are the states of a creature, the player or a monster, that
// pass with time: the turns left confused (moving and attacking in
// directions at random), slowed (the player: two turns an action; a
// monster: every other turn), fast (the player: two actions a turn; a
// fast monster has another Speed instead) and asleep (not woken by
// blows), and whether it is sealed (the player: its mouth, for the
// floor: no eating, drinking, reading, talking or orders; a monster:
// its abilities).
type States struct {
	Confused, Slowed, Hasted, Asleep int
	Sealed                           bool
}

// The turns of the states.
const (
	confuseTurns = 10
	slowTurns    = 10
	sleepTurns   = 5
	// hasteTurns is how long the player stays fast (a monster, for
	// good).
	hasteTurns = 20
)

// stateTexts are what is said when a bolt brings a state: to the player,
// and of a monster (after its name).
var stateTexts = map[MagicEffect]struct{ player, monster string }{
	MagicConfuse: {"混乱した！", "は混乱した！"},
	MagicSlow:    {"体が重くなった……。", "は遅くなった。"},
	MagicSleep:   {"眠ってしまった……。", "は眠ってしまった。"},
	MagicSeal:    {"口が封じられた！", "は能力を封印された。"},
}

// take brings the state of the bolt e, and says whether e is one of a
// state (the same for the player and a monster).
func (s *States) take(e MagicEffect) bool {
	switch e {
	case MagicConfuse:
		s.Confused = confuseTurns
	case MagicSlow:
		s.Slowed, s.Hasted = slowTurns, 0
	case MagicSleep:
		s.Asleep = sleepTurns
	case MagicSeal:
		s.Sealed = true
	default:
		return false
	}
	return true
}

// names returns the names of the states it is in, in a fixed order, the
// seal as sealed says (the player's is of its mouth).
func (s States) names(sealed string) []string {
	var ns []string
	for _, st := range []struct {
		on   bool
		name string
	}{
		{s.Confused > 0, "混乱"}, {s.Slowed > 0, "鈍足"}, {s.Hasted > 0, "倍速"},
		{s.Asleep > 0, "睡眠"}, {s.Sealed, sealed},
	} {
		if st.on {
			ns = append(ns, st.name)
		}
	}
	return ns
}

// actions is how many of its n actions of the turn a monster in these
// states takes, the turn passing for them: none asleep, and slowed only
// every other turn.
func (s *States) actions(n, turn int) int {
	if s.Asleep > 0 {
		s.Asleep--
		return 0
	}
	if s.Slowed > 0 {
		s.Slowed--
		n = min(n, turn%2)
	}
	return n
}
