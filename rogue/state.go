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
	// Blind is the turns left blind (目つぶし): the player sees only the
	// tiles next to it; a monster walks at random and attacks no one.
	Blind int
	// Bound says that it is bound (かなしばり, the scroll of 困った時): it
	// does nothing until a blow (any loss of HP) frees it (Shougo,
	// 2026-11-27, as in Shiren); an ally, also after boundTurns (the
	// turns left), or when another state comes on it.
	Bound int
	// Zowa is the turns left of ゾワゾワ (the scroll): about to attack the
	// player, a monster attacks the other way, and uses no ability.
	Zowa int
	// Unhurt is the turns left of 無敵 (無敵草): nothing takes HP away.
	Unhurt int
}

// The turns of the states.
const (
	confuseTurns = 10
	slowTurns    = 10
	sleepTurns   = 5
	blindTurns   = 10
	// boundTurns is how long かなしばり holds an ally (an enemy: until a
	// blow).
	boundTurns = 20
	// hasteTurns is how long the player stays fast (a monster, for
	// good).
	hasteTurns = 20
	// unhurtTurns is how long 無敵草 keeps from harm (Shiren 1 DS).
	unhurtTurns = 20
)

// stateTexts are what is said when a bolt brings a state: to the player,
// and of a monster (after its name).
var stateTexts = map[MagicEffect]struct{ player, monster string }{
	MagicConfuse: {"混乱した！", "は混乱した！"},
	MagicSlow:    {"体が重くなった……。", "は遅くなった。"},
	MagicSleep:   {"眠ってしまった……。", "は眠ってしまった。"},
	MagicSeal:    {"口が封じられた！", "は能力を封印された。"},
	MagicBlind:   {"目が見えなくなった！", "は目が見えなくなった。"},
	MagicBind:    {"かなしばりになった！", "はかなしばりになった！"},
	MagicUnhurt:  {"無敵になった！", "は無敵になった！"},
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
	case MagicBlind:
		s.Blind = blindTurns
	case MagicBind:
		// (An enemy is freed by a blow only; the count is for the
		// player and the allies.)
		s.Bound = boundTurns
	case MagicUnhurt:
		s.Unhurt = unhurtTurns
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
		{s.Asleep > 0, "睡眠"}, {s.Sealed, sealed}, {s.Blind > 0, "目つぶし"}, {s.Bound > 0, "かなしばり"}, {s.Zowa > 0, "ゾワゾワ"}, {s.Unhurt > 0, "無敵"},
	} {
		if st.on {
			ns = append(ns, st.name)
		}
	}
	return ns
}

// actions is how many of its n actions of the turn a monster in these
// states takes, the turn passing for them: none bound or asleep, and slowed only
// every other turn.
func (s *States) actions(n, turn int) int {
	if s.Unhurt > 0 {
		s.Unhurt--
	}
	if s.Bound > 0 {
		// (An ally's count is in alliesBound.)
		return 0
	}
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

// harm is the damage n it takes: none while unhurt (無敵).
func (s *States) harm(n int) int {
	if s.Unhurt > 0 {
		return 0
	}
	return n
}
