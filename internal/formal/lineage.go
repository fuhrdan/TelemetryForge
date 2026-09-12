package formal

// lineageState abstracts a three-record chain. Count is the number of records,
// ChainOK tracks predecessor/hash continuity, Sealed commits the chain, and
// Tampered represents mutation after the signed commitment was produced.
type lineageState struct {
	Count    uint8
	ChainOK  bool
	Sealed   bool
	Tampered bool
	Verified bool
	Rejected bool
}

func checkCryptographicLineage() Result {
	m := model[lineageState]{
		Name:    "cryptographic-lineage",
		Initial: lineageState{ChainOK: true},
		Next: func(s lineageState) []transition[lineageState] {
			var out []transition[lineageState]
			if !s.Sealed && s.Count < 3 && !s.Tampered {
				n := s
				n.Count++
				out = append(out, transition[lineageState]{Name: "append-linked-record", Next: n})
			}
			if !s.Sealed && s.Count > 0 && s.ChainOK {
				n := s
				n.Sealed = true
				out = append(out, transition[lineageState]{Name: "sign-merkle-root", Next: n})
			}
			if s.Sealed && !s.Tampered && !s.Verified && !s.Rejected {
				n := s
				n.Verified = true
				out = append(out, transition[lineageState]{Name: "verify-clean-seal", Next: n})
			}
			if s.Sealed && !s.Tampered && !s.Verified && !s.Rejected {
				n := s
				n.Tampered = true
				n.ChainOK = false
				out = append(out, transition[lineageState]{Name: "tamper-after-seal", Next: n})
			}
			if s.Sealed && s.Tampered && !s.Verified && !s.Rejected {
				n := s
				n.Rejected = true
				out = append(out, transition[lineageState]{Name: "reject-tampered-seal", Next: n})
			}
			return out
		},
		Invariants: []namedInvariant[lineageState]{
			{Name: "verified-history-is-not-tampered", Check: func(s lineageState) bool {
				return !s.Verified || !s.Tampered
			}},
			{Name: "tampered-sealed-history-cannot-verify", Check: func(s lineageState) bool {
				return !(s.Sealed && s.Tampered) || !s.Verified
			}},
			{Name: "rejection-requires-tamper", Check: func(s lineageState) bool {
				return !s.Rejected || s.Tampered
			}},
			{Name: "seal-requires-nonempty-valid-chain", Check: func(s lineageState) bool {
				return !s.Sealed || (s.Count > 0 && (s.ChainOK || s.Tampered))
			}},
		},
	}
	return explore(m)
}
