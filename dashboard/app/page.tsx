"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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

export default function Dashboard() {
  const [summary, setSummary] = useState<Summary>(emptySummary);
  const [live, setLive] = useState<TelemetryEvent[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [selected, setSelected] = useState<Incident | null>(null);
  const [incidentEvents, setIncidentEvents] = useState<TelemetryEvent[]>([]);
  const [streamState, setStreamState] = useState("connecting");
  const [sourceFilter, setSourceFilter] = useState("");

  const refresh = useCallback(async () => {
    const [summaryResponse, incidentResponse] = await Promise.all([
      fetch("/telemetry-api/api/v1/dashboard/summary?window=5m", { cache: "no-store" }),
      fetch("/telemetry-api/api/v1/incidents?limit=20", { cache: "no-store" }),
    ]);

    if (summaryResponse.ok) {
      setSummary(await summaryResponse.json());
    }
    if (incidentResponse.ok) {
      const payload = await incidentResponse.json();
      setIncidents(payload.incidents ?? []);
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
            <select value={sourceFilter} onChange={(event) => setSourceFilter(event.target.value)}>
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
