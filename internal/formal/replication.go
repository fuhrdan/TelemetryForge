package formal

import "math/bits"

// replicaState uses bits 0..2 for durable copies in three independent failure
// domains. Bit 0 is the origin. The v2.2 abstraction requires two durable
// domains before a replicated-mode client acknowledgement can be emitted.
type replicaState struct {
	DurableMask uint8
	ClientAck   bool
	Downstream  bool
	Released    bool
}

func checkReplicatedDurability() Result {
	const quorum = 2
	m := model[replicaState]{
		Name:    "replicated-durability",
		Initial: replicaState{},
		Next: func(s replicaState) []transition[replicaState] {
			var out []transition[replicaState]
			if s.DurableMask&1 == 0 && !s.Released {
				n := s
				n.DurableMask |= 1
				out = append(out, transition[replicaState]{Name: "persist-origin", Next: n})
			}
			if s.DurableMask&1 != 0 && !s.Released {
				for peer := uint(1); peer <= 2; peer++ {
					bit := uint8(1 << peer)
					if s.DurableMask&bit == 0 {
						n := s
						n.DurableMask |= bit
						out = append(out, transition[replicaState]{Name: "replicate-peer", Next: n})
					}
				}
			}
			if !s.ClientAck && bits.OnesCount8(s.DurableMask) >= quorum {
				n := s
				n.ClientAck = true
				out = append(out, transition[replicaState]{Name: "ack-after-quorum", Next: n})
			}
			if bits.OnesCount8(s.DurableMask) >= quorum && !s.Downstream {
				n := s
				n.Downstream = true
				out = append(out, transition[replicaState]{Name: "deliver-downstream", Next: n})
			}
			if s.Downstream && !s.Released {
				n := s
				n.Released = true
				n.DurableMask = 0
				out = append(out, transition[replicaState]{Name: "release-replicas", Next: n})
			}
			return out
		},
		Invariants: []namedInvariant[replicaState]{
			{Name: "replicated-ack-requires-quorum", Check: func(s replicaState) bool {
				return !s.ClientAck || s.Released || bits.OnesCount8(s.DurableMask) >= quorum
			}},
			{Name: "quorum-includes-origin-before-release", Check: func(s replicaState) bool {
				return s.Released || bits.OnesCount8(s.DurableMask) < quorum || s.DurableMask&1 != 0
			}},
			{Name: "release-only-after-downstream-delivery", Check: func(s replicaState) bool {
				return !s.Released || s.Downstream
			}},
			{Name: "acked-undelivered-event-retains-quorum", Check: func(s replicaState) bool {
				return !(s.ClientAck && !s.Downstream) || bits.OnesCount8(s.DurableMask) >= quorum
			}},
		},
	}
	return explore(m)
}
