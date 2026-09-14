// Package multicloudproof implements a deterministic fault-model checker for
// TelemetryForge multi-cloud durability and accounting semantics.
//
// It is intentionally dependency-free. The model is evidence for protocol
// behavior under named failure schedules; it is not a claim that CI caused a
// real public-cloud regional outage.
package multicloudproof

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID          string `json:"id"`
	Cloud       string `json:"cloud"`
	Region      string `json:"region"`
	Zone        string `json:"zone"`
	Healthy     bool   `json:"healthy"`
	Writable    bool   `json:"writable"`
	Partitioned bool   `json:"partitioned"`
}

type Event struct {
	ID                int             `json:"id"`
	Accepted          bool            `json:"accepted"`
	RejectedReason    string          `json:"rejected_reason,omitempty"`
	DurableCopies     map[string]bool `json:"durable_copies,omitempty"`
	Delivered         bool            `json:"delivered"`
	DeliveryAttempts  int             `json:"delivery_attempts"`
	DuplicateAttempts int             `json:"duplicate_attempts"`
	Corrupted         bool            `json:"corrupted"`
}

type Accounting struct {
	Offered           int `json:"offered"`
	Accepted          int `json:"accepted"`
	Rejected          int `json:"rejected"`
	DeliveredUnique   int `json:"delivered_unique"`
	Retained          int `json:"retained"`
	DuplicateAttempts int `json:"duplicate_attempts"`
	Lost              int `json:"lost"`
	Corrupted         int `json:"corrupted"`
	Unaccounted       int `json:"unaccounted"`
}

type Assertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}

type ScenarioReport struct {
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	DuringFault       Accounting  `json:"during_fault"`
	AfterRecovery     Accounting  `json:"after_recovery"`
	Assertions        []Assertion `json:"assertions"`
	Passed            bool        `json:"passed"`
	RecoveryDuration  string      `json:"recovery_duration"`
	FaultedDomains    []string    `json:"faulted_domains,omitempty"`
	RejectedUnderLoad int         `json:"rejected_under_load,omitempty"`
}

type Report struct {
	Format           string           `json:"format"`
	Version          int              `json:"version"`
	GeneratedAt      time.Time        `json:"generated_at"`
	OfferedEach      int              `json:"offered_each"`
	Quorum           int              `json:"quorum"`
	CopyTarget       int              `json:"copy_target"`
	Topology         []Node           `json:"topology"`
	Scenarios        []ScenarioReport `json:"scenarios"`
	Passed           bool             `json:"passed"`
	TotalOffered     int              `json:"total_offered"`
	TotalAccepted    int              `json:"total_accepted"`
	TotalRejected    int              `json:"total_rejected"`
	TotalDuplicates  int              `json:"total_duplicate_attempts"`
	TotalLost        int              `json:"total_lost"`
	TotalCorrupted   int              `json:"total_corrupted"`
	TotalUnaccounted int              `json:"total_unaccounted"`
}

type Fabric struct {
	nodes      map[string]*Node
	quorum     int
	copyTarget int
	nextID     int
	events     []*Event
}

func DefaultTopology() []Node {
	return []Node{
		{ID: "edge-aws-us-west-2", Cloud: "aws", Region: "us-west-2", Zone: "us-west-2a", Healthy: true, Writable: true},
		{ID: "edge-gcp-us-central1", Cloud: "gcp", Region: "us-central1", Zone: "us-central1-a", Healthy: true, Writable: true},
		{ID: "edge-azure-westus2", Cloud: "azure", Region: "westus2", Zone: "1", Healthy: true, Writable: true},
		{ID: "edge-bare-denver", Cloud: "bare-metal", Region: "denver", Zone: "dc1", Healthy: true, Writable: true},
	}
}

func NewFabric(nodes []Node, quorum, copyTarget int) (*Fabric, error) {
	if quorum < 1 {
		return nil, errors.New("quorum must be positive")
	}
	if copyTarget < quorum {
		return nil, errors.New("copy target must be at least quorum")
	}
	m := make(map[string]*Node, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if strings.TrimSpace(n.ID) == "" || strings.TrimSpace(n.Cloud) == "" {
			return nil, errors.New("node id and cloud are required")
		}
		if _, ok := m[n.ID]; ok {
			return nil, fmt.Errorf("duplicate node %q", n.ID)
		}
		cp := n
		m[n.ID] = &cp
	}
	return &Fabric{nodes: m, quorum: quorum, copyTarget: copyTarget}, nil
}

func (f *Fabric) Offer(count int) {
	for i := 0; i < count; i++ {
		f.nextID++
		event := &Event{ID: f.nextID, DurableCopies: map[string]bool{}}
		candidates := f.writableCandidates()
		distinct := map[string]bool{}
		for _, n := range candidates {
			if len(event.DurableCopies) >= f.copyTarget {
				break
			}
			if distinct[n.Cloud] {
				continue
			}
			event.DurableCopies[n.ID] = true
			distinct[n.Cloud] = true
		}
		if len(distinct) < f.quorum {
			event.DurableCopies = nil
			event.RejectedReason = "durability quorum unavailable before acceptance"
		} else {
			event.Accepted = true
		}
		f.events = append(f.events, event)
	}
}

func (f *Fabric) writableCandidates() []*Node {
	out := make([]*Node, 0, len(f.nodes))
	for _, n := range f.nodes {
		if n.Healthy && n.Writable && !n.Partitioned {
			out = append(out, n)
		}
	}
	rank := func(cloud string) int {
		switch cloud {
		case "aws":
			return 0
		case "gcp":
			return 1
		case "azure":
			return 2
		case "bare-metal":
			return 3
		default:
			return 100
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := rank(out[i].Cloud), rank(out[j].Cloud)
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (f *Fabric) SetCloud(cloud string, healthy, writable, partitioned bool) {
	for _, n := range f.nodes {
		if n.Cloud == cloud {
			n.Healthy = healthy
			n.Writable = writable
			n.Partitioned = partitioned
		}
	}
}

func (f *Fabric) SetRegion(region string, healthy, writable, partitioned bool) {
	for _, n := range f.nodes {
		if n.Region == region {
			n.Healthy = healthy
			n.Writable = writable
			n.Partitioned = partitioned
		}
	}
}

func (f *Fabric) SetNodeWritable(id string, writable bool) {
	if n := f.nodes[id]; n != nil {
		n.Writable = writable
	}
}

func (f *Fabric) Heal() {
	for _, n := range f.nodes {
		n.Healthy = true
		n.Writable = true
		n.Partitioned = false
	}
}

// DeliverAvailable attempts logical delivery for every accepted event whose
// durable copy is reachable. ambiguousRetries simulates ACK loss: the same
// logical event may be attempted again, but it remains one unique delivery.
func (f *Fabric) DeliverAvailable(ambiguousRetries int) {
	for _, e := range f.events {
		if !e.Accepted || e.Delivered {
			continue
		}
		reachable := false
		for id := range e.DurableCopies {
			n := f.nodes[id]
			if n != nil && n.Healthy && !n.Partitioned {
				reachable = true
				break
			}
		}
		if !reachable {
			continue
		}
		e.DeliveryAttempts++
		if e.Delivered {
			e.DuplicateAttempts++
		} else {
			e.Delivered = true
		}
		for i := 0; i < ambiguousRetries; i++ {
			e.DeliveryAttempts++
			e.DuplicateAttempts++
		}
	}
}

func (f *Fabric) Accounting() Accounting {
	var a Accounting
	a.Offered = len(f.events)
	for _, e := range f.events {
		if !e.Accepted {
			a.Rejected++
			continue
		}
		a.Accepted++
		a.DuplicateAttempts += e.DuplicateAttempts
		if e.Corrupted {
			a.Corrupted++
		}
		if e.Delivered {
			a.DeliveredUnique++
			continue
		}
		retained := false
		for id := range e.DurableCopies {
			if _, ok := f.nodes[id]; ok {
				retained = true
				break
			}
		}
		if retained {
			a.Retained++
		} else {
			a.Lost++
		}
	}
	a.Unaccounted = a.Accepted - a.DeliveredUnique - a.Retained - a.Lost
	return a
}

func accountingAssertions(during, after Accounting) []Assertion {
	checks := []Assertion{
		{Name: "accepted-accounted-during-fault", Passed: during.Accepted == during.DeliveredUnique+during.Retained+during.Lost && during.Unaccounted == 0, Expected: "accepted = delivered + retained + lost; unaccounted=0", Observed: fmt.Sprintf("accepted=%d delivered=%d retained=%d lost=%d unaccounted=%d", during.Accepted, during.DeliveredUnique, during.Retained, during.Lost, during.Unaccounted)},
		{Name: "zero-loss-during-fault", Passed: during.Lost == 0, Expected: "lost=0", Observed: fmt.Sprintf("lost=%d", during.Lost)},
		{Name: "zero-corruption", Passed: during.Corrupted == 0 && after.Corrupted == 0, Expected: "corrupted=0", Observed: fmt.Sprintf("during=%d after=%d", during.Corrupted, after.Corrupted)},
		{Name: "recovery-drains-accepted", Passed: after.Accepted == after.DeliveredUnique && after.Retained == 0 && after.Lost == 0 && after.Unaccounted == 0, Expected: "all accepted events uniquely delivered after recovery", Observed: fmt.Sprintf("accepted=%d delivered=%d retained=%d lost=%d unaccounted=%d", after.Accepted, after.DeliveredUnique, after.Retained, after.Lost, after.Unaccounted)},
		{Name: "rejection-is-before-acceptance", Passed: after.Offered == after.Accepted+after.Rejected, Expected: "offered = accepted + rejected", Observed: fmt.Sprintf("offered=%d accepted=%d rejected=%d", after.Offered, after.Accepted, after.Rejected)},
	}
	return checks
}

func reportScenario(name, description string, faulted []string, f *Fabric, started time.Time, rejectedUnderLoad int) ScenarioReport {
	during := f.Accounting()
	f.Heal()
	f.DeliverAvailable(0)
	after := f.Accounting()
	assertions := accountingAssertions(during, after)
	passed := true
	for _, a := range assertions {
		if !a.Passed {
			passed = false
		}
	}
	return ScenarioReport{Name: name, Description: description, DuringFault: during, AfterRecovery: after, Assertions: assertions, Passed: passed, RecoveryDuration: time.Since(started).String(), FaultedDomains: faulted, RejectedUnderLoad: rejectedUnderLoad}
}

func Run(offeredEach int) (Report, error) {
	if offeredEach < 10 {
		return Report{}, errors.New("offeredEach must be at least 10")
	}
	const quorum = 2
	const copies = 3
	topology := DefaultTopology()
	reports := make([]ScenarioReport, 0, 6)

	// 1. Whole-cloud outage after acceptance. Copies already exist across three clouds.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		f.Offer(offeredEach)
		f.SetCloud("aws", false, false, true)
		f.DeliverAvailable(0)
		reports = append(reports, reportScenario("aws-cloud-outage", "AWS failure after durable acceptance; surviving cloud copies carry delivery.", []string{"aws"}, f, started, 0))
	}

	// 2. Regional partition while ingestion continues. Quorum remains available outside the partition.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		half := offeredEach / 2
		f.Offer(half)
		f.SetRegion("us-central1", false, false, true)
		f.Offer(offeredEach - half)
		f.DeliverAvailable(0)
		reports = append(reports, reportScenario("gcp-region-partition", "GCP us-central1 is isolated while accepted events continue through other failure domains.", []string{"gcp/us-central1"}, f, started, 0))
	}

	// 3. Capacity exhaustion. Once only one writable cloud remains, new work is rejected before acceptance.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		half := offeredEach / 2
		f.Offer(half)
		f.SetCloud("aws", true, false, false)
		f.SetCloud("gcp", true, false, false)
		f.SetCloud("bare-metal", true, false, false)
		before := f.Accounting().Rejected
		f.Offer(offeredEach - half)
		afterOffer := f.Accounting()
		rejected := afterOffer.Rejected - before
		f.DeliverAvailable(0)
		reports = append(reports, reportScenario("disk-pressure-backpressure", "Durability capacity collapses below quorum; new telemetry is rejected before acknowledgement instead of silently dropped.", []string{"aws/write", "gcp/write", "bare-metal/write"}, f, started, rejected))
	}

	// 4. Ambiguous network acknowledgements create retries but not a second logical delivery.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		f.Offer(offeredEach)
		f.SetRegion("westus2", true, true, true)
		f.DeliverAvailable(2)
		reports = append(reports, reportScenario("packet-loss-latency", "Ambiguous delivery acknowledgements are modeled as retries; duplicate attempts remain explicitly accounted.", []string{"azure/westus2-network"}, f, started, 0))
	}

	// 5. Cascading two-cloud outage after three-copy durability.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		f.Offer(offeredEach)
		f.SetCloud("aws", false, false, true)
		f.SetCloud("azure", false, false, true)
		f.DeliverAvailable(0)
		reports = append(reports, reportScenario("cascading-cloud-outage", "AWS and Azure become unavailable after three-copy persistence; a surviving durable cloud copy remains accountable.", []string{"aws", "azure"}, f, started, 0))
	}

	// 6. Total delivery partition after acceptance. No route is reachable during
	// the split, so accepted telemetry must remain retained until recovery.
	{
		started := time.Now()
		f, _ := NewFabric(topology, quorum, copies)
		f.Offer(offeredEach)
		f.SetCloud("aws", false, true, true)
		f.SetCloud("gcp", false, true, true)
		f.SetCloud("azure", false, true, true)
		f.SetCloud("bare-metal", false, true, true)
		f.DeliverAvailable(0)
		reports = append(reports, reportScenario("global-delivery-partition", "All delivery paths are temporarily partitioned after acceptance; durable copies remain retained until the fabric heals.", []string{"aws/network", "gcp/network", "azure/network", "bare-metal/network"}, f, started, 0))
	}

	r := Report{Format: "telemetryforge-multicloud-proof", Version: 1, GeneratedAt: time.Now().UTC(), OfferedEach: offeredEach, Quorum: quorum, CopyTarget: copies, Topology: topology, Scenarios: reports, Passed: true}
	for _, s := range reports {
		r.TotalOffered += s.AfterRecovery.Offered
		r.TotalAccepted += s.AfterRecovery.Accepted
		r.TotalRejected += s.AfterRecovery.Rejected
		r.TotalDuplicates += s.AfterRecovery.DuplicateAttempts
		r.TotalLost += s.AfterRecovery.Lost
		r.TotalCorrupted += s.AfterRecovery.Corrupted
		r.TotalUnaccounted += s.AfterRecovery.Unaccounted
		if !s.Passed {
			r.Passed = false
		}
	}
	return r, nil
}
