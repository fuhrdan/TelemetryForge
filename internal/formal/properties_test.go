package formal

import (
	"math/bits"
	"testing"
)

// FuzzProtocolActions treats input bytes as an adversarial scheduler over the
// replicated durability abstraction. Guards mirror protocol preconditions; the
// assertions are checked after every attempted action. This complements the
// exhaustive finite checker with longer randomized schedules.
func FuzzProtocolActions(f *testing.F) {
	f.Add([]byte{0, 1, 3, 4, 5})
	f.Add([]byte{0, 2, 1, 3, 4, 5})
	f.Add([]byte{3, 4, 5, 0, 1, 3})
	f.Fuzz(func(t *testing.T, actions []byte) {
		state := replicaState{}
		const quorum = 2

		for _, raw := range actions {
			switch raw % 6 {
			case 0: // persist origin
				if state.DurableMask&1 == 0 && !state.Released {
					state.DurableMask |= 1
				}
			case 1: // peer A
				if state.DurableMask&1 != 0 && state.DurableMask&0b010 == 0 && !state.Released {
					state.DurableMask |= 0b010
				}
			case 2: // peer B
				if state.DurableMask&1 != 0 && state.DurableMask&0b100 == 0 && !state.Released {
					state.DurableMask |= 0b100
				}
			case 3: // acknowledge
				if bits.OnesCount8(state.DurableMask) >= quorum {
					state.ClientAck = true
				}
			case 4: // deliver
				if bits.OnesCount8(state.DurableMask) >= quorum {
					state.Downstream = true
				}
			case 5: // release
				if state.Downstream && !state.Released {
					state.Released = true
					state.DurableMask = 0
				}
			}

			if state.ClientAck && !state.Downstream && bits.OnesCount8(state.DurableMask) < quorum {
				t.Fatalf("acked undelivered state lost quorum: %+v", state)
			}
			if state.Released && !state.Downstream {
				t.Fatalf("released before downstream delivery: %+v", state)
			}
			if !state.Released && bits.OnesCount8(state.DurableMask) >= quorum && state.DurableMask&1 == 0 {
				t.Fatalf("quorum omitted origin: %+v", state)
			}
		}
	})
}
