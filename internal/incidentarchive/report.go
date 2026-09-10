package incidentarchive

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type reportView struct {
	ArchiveID       string
	ProductVersion  string
	TenantID        string
	Incident        IncidentMetadata
	EventCount      int
	SchemaCount     int
	DriftCount      int
	ReplayCount     int
	CostCount       int
	Configurations  []ConfigurationMeta
	Hypotheses      []reportHypothesis
	Edges           []reportEdge
	Events          []reportEvent
	GeneratedAt     string
	GraphDisclaimer string
}

type reportHypothesis struct {
	Statement     string
	Status        string
	Supporting    int
	Contradicting int
	Notes         string
}

type reportEdge struct {
	Assessment string
	Relation   string
	Reason     string
}

type reportEvent struct {
	Time          string
	Source        string
	Type          string
	CorrelationID string
	Severity      string
	Value         string
}

// WriteHTMLReport creates a standalone offline investigation report from a
// verified in-memory bundle. No JavaScript, network calls, or external assets
// are required.
func WriteHTMLReport(filename string, bundle Bundle) error {
	view := reportView{
		ArchiveID:       bundle.Manifest.ArchiveID,
		ProductVersion:  bundle.Manifest.TelemetryForge,
		TenantID:        bundle.Manifest.TenantID,
		Incident:        bundle.Incident,
		EventCount:      len(bundle.Events),
		SchemaCount:     len(bundle.Schemas),
		DriftCount:      len(bundle.SchemaDrifts),
		ReplayCount:     len(bundle.ReplayRuns),
		CostCount:       len(bundle.CostResults),
		Configurations:  bundle.Manifest.Configurations,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		GraphDisclaimer: bundle.EvidenceGraph.Disclaimer,
	}

	for _, hypothesis := range bundle.EvidenceGraph.Hypotheses {
		view.Hypotheses = append(view.Hypotheses, reportHypothesis{
			Statement: hypothesis.Statement, Status: hypothesis.Status,
			Supporting: hypothesis.Supporting, Contradicting: hypothesis.Contradicting,
			Notes: hypothesis.Notes,
		})
	}
	for _, edge := range bundle.EvidenceGraph.Edges {
		if edge.Assessment == "related" {
			continue
		}
		view.Edges = append(view.Edges, reportEdge{
			Assessment: edge.Assessment, Relation: edge.Relation, Reason: edge.Reason,
		})
		if len(view.Edges) >= 100 {
			break
		}
	}

	events := append([]EventRecord(nil), bundle.Events...)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Event.Timestamp.Before(events[j].Event.Timestamp)
	})
	for _, record := range events {
		view.Events = append(view.Events, reportEventFrom(record.Event))
		if len(view.Events) >= 500 {
			break
		}
	}

	tpl, err := template.New("incident-report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("parse incident report template: %w", err)
	}
	var output bytes.Buffer
	if err := tpl.Execute(&output, view); err != nil {
		return fmt.Errorf("render incident report: %w", err)
	}
	if err := os.WriteFile(filename, output.Bytes(), 0600); err != nil {
		return fmt.Errorf("write incident report: %w", err)
	}
	return nil
}

func reportEventFrom(event domain.Event) reportEvent {
	severity := ""
	for _, key := range []string{"severity", "level"} {
		if value := strings.TrimSpace(event.Tags[key]); value != "" {
			severity = value
			break
		}
	}
	value := ""
	if event.Value != nil {
		value = fmt.Sprintf("%.3f %s", *event.Value, event.Unit)
	}
	return reportEvent{
		Time:   event.Timestamp.UTC().Format(time.RFC3339Nano),
		Source: event.Source, Type: event.Type,
		CorrelationID: event.CorrelationID, Severity: severity, Value: value,
	}
}

const reportTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TelemetryForge Incident {{.Incident.ID}}</title>
<style>
:root{color-scheme:dark;background:#0b1117;color:#dce7ee;font-family:Inter,system-ui,sans-serif}
body{max-width:1320px;margin:0 auto;padding:28px;background:#0b1117}
h1,h2{margin:.2rem 0 .8rem} h1{font-size:2rem} h2{font-size:1rem;letter-spacing:.06em;text-transform:uppercase;color:#84dfc7}
small,.muted{color:#82929e}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px;margin:16px 0}
.card,.panel{background:#101922;border:1px solid #22303b;border-radius:10px;padding:14px}.card strong{display:block;font-size:1.35rem}
.panel{margin-top:14px}.warn{border-left:4px solid #f3bf65;padding:10px;background:#181914}.supporting{color:#62e0bb}.contradicting{color:#ff7888}
table{width:100%;border-collapse:collapse;font-size:.78rem}th,td{text-align:left;padding:8px;border-top:1px solid #22303b;vertical-align:top}th{color:#82929e}
.badge{display:inline-block;border:1px solid #334553;border-radius:999px;padding:3px 7px;text-transform:uppercase;font-size:.65rem}
pre{white-space:pre-wrap;word-break:break-word}.hypothesis{margin:8px 0;padding:10px;border:1px solid #22303b;border-radius:8px}
footer{margin-top:30px;color:#63737e;font-size:.7rem}
</style>
</head>
<body>
<div class="muted">TELEMETRYFORGE INCIDENT ARCHIVE · FORMAT V1</div>
<h1>{{.Incident.Title}}</h1>
<div class="muted">{{.Incident.ID}} · tenant {{.TenantID}} · archive {{.ArchiveID}}</div>

<div class="grid">
<div class="card"><small>Frozen events</small><strong>{{.EventCount}}</strong></div>
<div class="card"><small>Schema records</small><strong>{{.SchemaCount}}</strong></div>
<div class="card"><small>Schema drift</small><strong>{{.DriftCount}}</strong></div>
<div class="card"><small>Replay runs</small><strong>{{.ReplayCount}}</strong></div>
<div class="card"><small>Cost simulations</small><strong>{{.CostCount}}</strong></div>
</div>

<div class="panel">
<h2>Incident metadata</h2>
<table>
<tr><th>Status</th><td>{{.Incident.Status}}</td></tr>
<tr><th>Trigger</th><td>{{.Incident.TriggerReason}}</td></tr>
<tr><th>Frozen window</th><td>{{.Incident.FrozenFrom}} → {{.Incident.FrozenTo}}</td></tr>
<tr><th>Detected</th><td>{{.Incident.DetectedAt}}</td></tr>
<tr><th>TelemetryForge</th><td>{{.ProductVersion}}</td></tr>
</table>
</div>

<div class="panel">
<h2>Evidence Graph</h2>
<div class="warn">{{if .GraphDisclaimer}}{{.GraphDisclaimer}}{{else}}Relationships are evidence associations, not automated root-cause claims.{{end}}</div>
{{range .Hypotheses}}
<div class="hypothesis"><span class="badge">{{.Status}}</span>
<strong>{{.Statement}}</strong><br>
<small>{{.Supporting}} supporting · {{.Contradicting}} contradicting</small><br>
<span class="muted">{{.Notes}}</span></div>
{{else}}<div class="muted">No hypotheses stored.</div>{{end}}
<table>
<tr><th>Assessment</th><th>Relationship</th><th>Basis</th></tr>
{{range .Edges}}<tr><td class="{{.Assessment}}">{{.Assessment}}</td><td>{{.Relation}}</td><td>{{.Reason}}</td></tr>{{end}}
</table>
</div>

<div class="panel">
<h2>Frozen timeline</h2>
<table>
<tr><th>Time</th><th>Source</th><th>Type</th><th>Severity</th><th>Metric</th><th>Correlation</th></tr>
{{range .Events}}<tr><td>{{.Time}}</td><td>{{.Source}}</td><td>{{.Type}}</td><td>{{.Severity}}</td><td>{{.Value}}</td><td>{{.CorrelationID}}</td></tr>{{end}}
</table>
{{if ge .EventCount 501}}<div class="muted">Timeline display capped at the first 500 events; the archive retains all events.</div>{{end}}
</div>

<div class="panel">
<h2>Configuration snapshots</h2>
<table><tr><th>Kind</th><th>Role</th><th>Name</th><th>Version</th><th>Archive member</th></tr>
{{range .Configurations}}<tr><td>{{.Kind}}</td><td>{{.Role}}</td><td>{{.Name}}</td><td>{{.Version}}</td><td>{{.Path}}</td></tr>{{else}}<tr><td colspan="5">No configuration snapshots.</td></tr>{{end}}
</table>
</div>

<footer>Generated offline from a verified .tfincident archive at {{.GeneratedAt}}. This report intentionally contains no external scripts, fonts, trackers, or network requests.</footer>
</body>
</html>`
