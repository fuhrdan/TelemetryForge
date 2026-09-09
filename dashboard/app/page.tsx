"use client";

import { type ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Sparkline } from "../components/Sparkline";

type TelemetryEvent = {
  id: string;
  source: string;
  type: string;
  timestamp: string;
  tags?: Record<string, string>;
  value?: number;
  unit?: string;
  correlation_id?: string;
};

type Summary = {
  window_seconds: number;
  events: number;
  events_per_second: number;
  error_count: number;
  error_rate: number;
  p95_latency_ms: number;
  active_sources: number;
};

type Incident = {
  id: string;
  title: string;
  status: string;
  trigger_reason?: string;
  frozen_from: string;
  frozen_to: string;
  detected_at: string;
  event_count: number;
};

type CardinalityFinding = {
  policy_name: string;
  policy_version: string;
  mode: "active" | "shadow";
  source: string;
  event_type: string;
  dimension: string;
  observed_unique: number;
  projected_unique: number;
  action: "allow" | "drop_tag" | "quarantine";
  reason: string;
  last_seen: string;
};

type PolicyDiff = {
  observed_at: string;
  source: string;
  event_type: string;
  dimension: string;
  active_policy: string;
  active_version: string;
  active_action: string;
  shadow_policy: string;
  shadow_version: string;
  shadow_action: string;
  reason: string;
};

type ReplayRun = {
  run_id: string;
  incident_id: string;
  mode: string;
  status: string;
  active_policy: string;
  active_version: string;
  shadow_policy?: string;
  shadow_version?: string;
  output_topic?: string;
  started_at: string;
  completed_at?: string;
  event_count: number;
  changed_event_count: number;
  dropped_tag_count: number;
  quarantined_count: number;
  finding_count: number;
  shadow_diff_count: number;
  published_count: number;
  error?: string;
};

type CostProfile = {
  bytes: number;
  series: number;
  changed_events: number;
};

type CostSimulation = {
  simulation_id: string;
  incident_id: string;
  active_policy: string;
  active_version: string;
  shadow_policy?: string;
  shadow_version?: string;
  pricing_model?: string;
  currency?: string;
  started_at: string;
  completed_at: string;
  event_count: number;
  baseline: CostProfile;
  active: CostProfile;
  shadow: CostProfile;
  projected_monthly_baseline_gb: number;
  projected_monthly_active_gb: number;
  projected_monthly_shadow_gb: number;
  projected_monthly_baseline_cost?: number;
  projected_monthly_active_cost?: number;
  projected_monthly_shadow_cost?: number;
  error?: string;
};

const emptySummary: Summary = {
  window_seconds: 300,
  events: 0,
  events_per_second: 0,
  error_count: 0,
  error_rate: 0,
  p95_latency_ms: 0,
  active_sources: 0,
};

function pct(value: number) {
  return `${(value * 100).toFixed(2)}%`;
}

function clock(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleTimeString();
}

function compact(value: number) {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);
}

function reduction(baseline: number, candidate: number) {
  if (baseline <= 0) {
    return "0%";
  }
  return `${Math.max(0, ((baseline - candidate) / baseline) * 100).toFixed(1)}%`;
}

function money(value: number | undefined, currency?: string) {
  if (value === undefined) {
    return "volume only";
  }
  try {
    return new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: currency || "USD",
      maximumFractionDigits: 2,
    }).format(value);
  } catch {
    return `${currency || ""} ${value.toFixed(2)}`.trim();
  }
}

export default function Dashboard() {
  const [summary, setSummary] = useState<Summary>(emptySummary);
  const [live, setLive] = useState<TelemetryEvent[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [selected, setSelected] = useState<Incident | null>(null);
  const [incidentEvents, setIncidentEvents] = useState<TelemetryEvent[]>([]);
  const [streamState, setStreamState] = useState("connecting");
  const [sourceFilter, setSourceFilter] = useState("");
  const [findings, setFindings] = useState<CardinalityFinding[]>([]);
  const [policyDiffs, setPolicyDiffs] = useState<PolicyDiff[]>([]);
  const [replayRuns, setReplayRuns] = useState<ReplayRun[]>([]);
  const [costSimulations, setCostSimulations] = useState<CostSimulation[]>([]);

  const refresh = useCallback(async () => {
    const [
      summaryResponse,
      incidentResponse,
      findingResponse,
      diffResponse,
      replayResponse,
      costResponse,
    ] = await Promise.all([
      fetch("/telemetry-api/api/v1/dashboard/summary?window=5m", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/incidents?limit=20", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/cardinality/findings?limit=30", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/policy/shadow-diffs?limit=20", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/replays?limit=20", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/cost-simulations?limit=20", { cache: "no-store" }),
    ]);

    if (summaryResponse.ok) {
      setSummary(await summaryResponse.json());
    }
    if (incidentResponse.ok) {
      const payload = await incidentResponse.json();
      setIncidents(payload.incidents ?? []);
    }
    if (findingResponse.ok) {
      const payload = await findingResponse.json();
      setFindings(payload.findings ?? []);
    }
    if (diffResponse.ok) {
      const payload = await diffResponse.json();
      setPolicyDiffs(payload.diffs ?? []);
    }
    if (replayResponse.ok) {
      const payload = await replayResponse.json();
      setReplayRuns(payload.runs ?? []);
    }
    if (costResponse.ok) {
      const payload = await costResponse.json();
      setCostSimulations(payload.simulations ?? []);
    }
  }, []);

  useEffect(() => {
    refresh().catch(() => undefined);
    const timer = window.setInterval(() => refresh().catch(() => undefined), 5000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  useEffect(() => {
    const source = new EventSource("/telemetry-api/api/v1/live");

    source.addEventListener("ready", () => setStreamState("live"));
    source.addEventListener("telemetry", (message) => {
      const event = JSON.parse((message as MessageEvent).data) as TelemetryEvent;
      setLive((previous) => [...previous.slice(-119), event]);
    });
    source.onerror = () => setStreamState("reconnecting");

    return () => source.close();
  }, []);

  useEffect(() => {
    if (!selected) {
      setIncidentEvents([]);
      return;
    }
    fetch(`/telemetry-api/api/v1/incidents/${encodeURIComponent(selected.id)}/events`)
      .then((response) => (response.ok ? response.json() : Promise.reject()))
      .then((payload) => setIncidentEvents(payload.events ?? []))
      .catch(() => setIncidentEvents([]));
  }, [selected]);

  const sources = useMemo(
    () => Array.from(new Set(live.map((event) => event.source))).sort(),
    [live],
  );

  const visible = sourceFilter
    ? live.filter((event) => event.source === sourceFilter)
    : live;

  const metricValues = visible
    .filter((event) => typeof event.value === "number")
    .map((event) => event.value as number)
    .slice(-50);

  const latest = [...visible].reverse().slice(0, 18);

  return (
    <main>
      <header className="topbar">
        <div>
          <div className="eyebrow">DISTRIBUTED OBSERVABILITY CONTROL PLANE</div>
          <h1>TelemetryForge</h1>
        </div>
        <div className={`live-pill ${streamState}`}>
          <span className="pulse" />
          {streamState.toUpperCase()}
        </div>
      </header>

      <section className="score-grid">
        <MetricCard label="Events / sec" value={summary.events_per_second.toFixed(2)} detail={`${summary.events} in 5m`} />
        <MetricCard label="Error rate" value={pct(summary.error_rate)} detail={`${summary.error_count} errors`} alert={summary.error_rate > 0.02} />
        <MetricCard label="P95 latency" value={`${summary.p95_latency_ms.toFixed(0)} ms`} detail="duration / latency metrics" alert={summary.p95_latency_ms >= 1000} />
        <MetricCard label="Active sources" value={String(summary.active_sources)} detail="last 5 minutes" />
      </section>

      <section className="main-grid">
        <article className="panel chart-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">LIVE METRIC SIGNAL</div>
              <h2>Incoming metric values</h2>
            </div>
            <select value={sourceFilter} onChange={(event: ChangeEvent<HTMLSelectElement>) => setSourceFilter(event.target.value)}>
              <option value="">All sources</option>
              {sources.map((source) => <option key={source} value={source}>{source}</option>)}
            </select>
          </div>
          <Sparkline values={metricValues} label="Recent live metric values" />
        </article>

        <article className="panel incident-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">FLIGHT RECORDER</div>
              <h2>Incidents</h2>
            </div>
            <span className="count-badge">{incidents.length}</span>
          </div>
          <div className="incident-list">
            {incidents.length === 0 && <div className="empty">No frozen incidents yet.</div>}
            {incidents.map((incident) => (
              <button
                className={`incident-row ${selected?.id === incident.id ? "selected" : ""}`}
                key={incident.id}
                onClick={() => setSelected(incident)}
              >
                <div>
                  <strong>{incident.title}</strong>
                  <span>{incident.trigger_reason || "Manual Flight Recorder freeze"}</span>
                </div>
                <div className="incident-meta">
                  <span>{incident.event_count} events</span>
                  <span>{clock(incident.detected_at)}</span>
                </div>
              </button>
            ))}
          </div>
        </article>
      </section>

      <section className="lower-grid">
        <article className="panel stream-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">SSE STREAM</div>
              <h2>Live telemetry</h2>
            </div>
            <span className="muted">{visible.length} buffered</span>
          </div>
          <div className="event-table">
            <div className="event-header"><span>Time</span><span>Source</span><span>Type</span><span>Value</span></div>
            {latest.map((event) => (
              <div className="event-row" key={event.id}>
                <span>{clock(event.timestamp)}</span>
                <span className="source">{event.source}</span>
                <span>{event.type}</span>
                <span>{typeof event.value === "number" ? `${event.value.toFixed(1)} ${event.unit ?? ""}` : "event"}</span>
              </div>
            ))}
          </div>
        </article>

        <article className="panel investigation-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">INCIDENT INVESTIGATION</div>
              <h2>{selected?.id ?? "Select an incident"}</h2>
            </div>
          </div>
          {!selected && <div className="empty investigation-empty">Choose a frozen incident to inspect its captured timeline.</div>}
          {selected && (
            <>
              <div className="reason-box">
                <span>Trigger</span>
                <strong>{selected.trigger_reason || "Manual freeze"}</strong>
              </div>
              <div className="timeline">
                {incidentEvents.slice(0, 30).map((event) => (
                  <div className="timeline-row" key={`${selected.id}-${event.id}`}>
                    <span className="timeline-dot" />
                    <div>
                      <strong>{event.source}</strong>
                      <span>{event.type}</span>
                    </div>
                    <time>{clock(event.timestamp)}</time>
                  </div>
                ))}
                {incidentEvents.length === 0 && <div className="empty">No captured events in this window.</div>}
              </div>
            </>
          )}
        </article>
      </section>

      <section className="policy-grid">
        <article className="panel firewall-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">CARDINALITY FIREWALL</div>
              <h2>High-risk dimensions</h2>
            </div>
            <span className="count-badge">{findings.length}</span>
          </div>
          <div className="policy-table">
            <div className="policy-header">
              <span>Source</span><span>Dimension</span><span>Unique</span><span>Projected</span><span>Action</span>
            </div>
            {findings.slice(0, 12).map((finding, index) => (
              <div className="policy-row" key={`${finding.source}-${finding.dimension}-${finding.last_seen}-${index}`}>
                <span>{finding.source}</span>
                <span className="source">{finding.dimension}</span>
                <span>{finding.observed_unique}</span>
                <span>{finding.projected_unique}</span>
                <span className={`action ${finding.action}`}>{finding.mode === "shadow" ? `shadow:${finding.action}` : finding.action}</span>
              </div>
            ))}
            {findings.length === 0 && <div className="empty policy-empty">No cardinality findings yet.</div>}
          </div>
        </article>

        <article className="panel shadow-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">SHADOW PIPELINE</div>
              <h2>Candidate policy differences</h2>
            </div>
            <span className="count-badge">{policyDiffs.length}</span>
          </div>
          <div className="shadow-list">
            {policyDiffs.slice(0, 10).map((diff, index) => (
              <div className="shadow-row" key={`${diff.source}-${diff.dimension}-${diff.observed_at}-${index}`}>
                <div>
                  <strong>{diff.source} · {diff.dimension}</strong>
                  <span>{diff.event_type}</span>
                </div>
                <div className="shadow-actions">
                  <span>{diff.active_action}</span>
                  <b>→</b>
                  <span>{diff.shadow_action}</span>
                </div>
              </div>
            ))}
            {policyDiffs.length === 0 && <div className="empty policy-empty">Active and shadow policies currently agree.</div>}
          </div>
        </article>
      </section>

      <section className="evidence-grid">
        <article className="panel replay-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">INCIDENT REPLAY</div>
              <h2>Policy replay history</h2>
            </div>
            <span className="count-badge">{replayRuns.length}</span>
          </div>
          <div className="replay-list">
            {replayRuns.slice(0, 10).map((run) => (
              <div className="replay-row" key={run.run_id}>
                <div>
                  <strong>{run.incident_id}</strong>
                  <span>{run.active_policy}@{run.active_version} · {run.mode}</span>
                </div>
                <div className="replay-metrics">
                  <span>{run.event_count} events</span>
                  <span>{run.changed_event_count} changed</span>
                  <span>{run.dropped_tag_count} tags dropped</span>
                  <span>{run.quarantined_count} quarantined</span>
                </div>
                <div className={`run-status ${run.status}`}>{run.status}</div>
              </div>
            ))}
            {replayRuns.length === 0 && (
              <div className="empty evidence-empty">
                No replay runs yet. Use telemetryctl incident replay on a frozen incident.
              </div>
            )}
          </div>
        </article>

        <article className="panel cost-panel">
          <div className="panel-heading">
            <div>
              <div className="eyebrow">TELEMETRY COST SIMULATOR</div>
              <h2>Policy impact projections</h2>
            </div>
            <span className="count-badge">{costSimulations.length}</span>
          </div>
          <div className="cost-list">
            {costSimulations.slice(0, 8).map((simulation) => (
              <div className="cost-row" key={simulation.simulation_id}>
                <div className="cost-title">
                  <strong>{simulation.incident_id}</strong>
                  <span>{simulation.active_policy}@{simulation.active_version}</span>
                </div>
                <div className="cost-stat">
                  <span>Monthly volume</span>
                  <strong>{compact(simulation.projected_monthly_active_gb)} GB</strong>
                  <small>{reduction(simulation.projected_monthly_baseline_gb, simulation.projected_monthly_active_gb)} lower</small>
                </div>
                <div className="cost-stat">
                  <span>Sample series</span>
                  <strong>{compact(simulation.active.series)}</strong>
                  <small>{reduction(simulation.baseline.series, simulation.active.series)} lower</small>
                </div>
                <div className="cost-stat">
                  <span>Projected cost</span>
                  <strong>{money(simulation.projected_monthly_active_cost, simulation.currency)}</strong>
                  <small>{simulation.pricing_model || "no pricing model"}</small>
                </div>
              </div>
            ))}
            {costSimulations.length === 0 && (
              <div className="empty evidence-empty">
                No cost simulations yet. Dollar estimates appear only with explicit pricing inputs.
              </div>
            )}
          </div>
        </article>
      </section>
    </main>
  );
}

function MetricCard({
  label,
  value,
  detail,
  alert = false,
}: {
  label: string;
  value: string;
  detail: string;
  alert?: boolean;
}) {
  return (
    <article className={`metric-card ${alert ? "alert" : ""}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  );
}
