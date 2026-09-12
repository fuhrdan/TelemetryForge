package formal

// durableState abstracts one accepted-at-most-once client request through the
// local WAL lifecycle. ClientAck is the externally visible success response.
type durableState struct {
	LocalDurable bool
	ClientAck    bool
	Downstream   bool
	Compacted    bool
}

func checkDurableIngest() Result {
	m := model[durableState]{
		Name:    "durable-ingest",
		Initial: durableState{},
		Next: func(s durableState) []transition[durableState] {
			var out []transition[durableState]
			if !s.LocalDurable && !s.Compacted {
				n := s
				n.LocalDurable = true
				out = append(out, transition[durableState]{Name: "fsync-local-wal", Next: n})
			}
			if s.LocalDurable && !s.ClientAck {
				n := s
				n.ClientAck = true
				out = append(out, transition[durableState]{Name: "ack-client", Next: n})
			}
			if s.LocalDurable && !s.Downstream {
				n := s
				n.Downstream = true
				out = append(out, transition[durableState]{Name: "deliver", Next: n})
			}
			if s.Downstream && s.LocalDurable && !s.Compacted {
				n := s
				n.Compacted = true
				n.LocalDurable = false
				out = append(out, transition[durableState]{Name: "compact-after-delivery", Next: n})
			}
			return out
		},
		Invariants: []namedInvariant[durableState]{
			{Name: "client-ack-implies-durable-or-delivered", Check: func(s durableState) bool {
				return !s.ClientAck || s.LocalDurable || s.Downstream
			}},
			{Name: "compaction-only-after-downstream-delivery", Check: func(s durableState) bool {
				return !s.Compacted || s.Downstream
			}},
			{Name: "undelivered-acked-event-remains-local-durable", Check: func(s durableState) bool {
				return !(s.ClientAck && !s.Downstream) || s.LocalDurable
			}},
		},
	}
	return explore(m)
}
