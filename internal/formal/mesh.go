package formal

// meshState models an ordered three-node candidate list. HealthyMask changes
// only by observed forwarding failure in this safety abstraction. AttemptedMask
// prevents a recursive/bouncing route from revisiting an owner in one delivery.
type meshState struct {
	HealthyMask   uint8
	AttemptedMask uint8
	Delivered     bool
	Failed        bool
}

func firstEligible(s meshState) int {
	for i := 0; i < 3; i++ {
		bit := uint8(1 << uint(i))
		if s.HealthyMask&bit != 0 && s.AttemptedMask&bit == 0 {
			return i
		}
	}
	return -1
}

func checkMeshFailover() Result {
	m := model[meshState]{
		Name:    "mesh-failover",
		Initial: meshState{HealthyMask: 0b111},
		Next: func(s meshState) []transition[meshState] {
			if s.Delivered || s.Failed {
				return nil
			}
			owner := firstEligible(s)
			if owner < 0 {
				n := s
				n.Failed = true
				return []transition[meshState]{{Name: "no-route", Next: n}}
			}
			bit := uint8(1 << uint(owner))
			success := s
			success.AttemptedMask |= bit
			success.Delivered = true

			failure := s
			failure.AttemptedMask |= bit
			failure.HealthyMask &^= bit
			return []transition[meshState]{
				{Name: "forward-success", Next: success},
				{Name: "forward-failure", Next: failure},
			}
		},
		Invariants: []namedInvariant[meshState]{
			{Name: "attempts-are-subset-of-known-candidates", Check: func(s meshState) bool {
				return s.AttemptedMask&^uint8(0b111) == 0
			}},
			{Name: "delivery-never-coexists-with-terminal-failure", Check: func(s meshState) bool {
				return !(s.Delivered && s.Failed)
			}},
			{Name: "terminal-failure-means-no-eligible-owner", Check: func(s meshState) bool {
				return !s.Failed || firstEligible(s) == -1
			}},
			{Name: "bounded-owner-attempts-prevent-forwarding-loops", Check: func(s meshState) bool {
				return s.AttemptedMask <= 0b111
			}},
		},
	}
	return explore(m)
}
