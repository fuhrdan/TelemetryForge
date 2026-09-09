package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

// CardinalityFinding is the stored representation returned to the dashboard.
type CardinalityFinding = policy.Finding

// PolicyDiff is the stored active-versus-shadow comparison.
type PolicyDiff = policy.Diff

// RecordCardinalityFinding persists a policy finding without the raw dimension
// value. The short fingerprint is sufficient to recognize repeated values
// during investigation without leaking an identifier into operational tables.
func (store *PostgresStore) RecordCardinalityFinding(ctx context.Context, finding policy.Finding) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO cardinality_findings
			(tenant_id, observed_at, policy_name, policy_version, mode, source,
			 event_type, dimension, observed_unique, projected_unique, action,
			 reason, value_fingerprint, first_seen, last_seen)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenantForPolicyFinding(ctx, finding.TenantID),
		time.Now().UTC(),
		finding.PolicyName,
		finding.PolicyVersion,
		finding.Mode,
		finding.Source,
		finding.EventType,
		finding.Dimension,
		finding.ObservedUnique,
		finding.ProjectedUnique,
		string(finding.Action),
		finding.Reason,
		finding.ValueFingerprint,
		finding.FirstSeen,
		finding.LastSeen,
	)
	if err != nil {
		return fmt.Errorf("insert cardinality finding: %w", err)
	}
	return nil
}

// RecordPolicyDiff persists one candidate-policy disagreement.
func (store *PostgresStore) RecordPolicyDiff(ctx context.Context, diff policy.Diff) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO policy_shadow_diffs
			(tenant_id, observed_at, source, event_type, dimension, active_policy,
			 active_version, active_action, shadow_policy, shadow_version,
			 shadow_action, reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		tenantForPolicyFinding(ctx, diff.TenantID),
		diff.ObservedAt,
		diff.Source,
		diff.EventType,
		diff.Dimension,
		diff.ActivePolicy,
		diff.ActiveVersion,
		string(diff.ActiveAction),
		diff.ShadowPolicy,
		diff.ShadowVersion,
		string(diff.ShadowAction),
		diff.Reason,
	)
	if err != nil {
		return fmt.Errorf("insert policy shadow diff: %w", err)
	}
	return nil
}

// RecordQuarantine preserves the original normalized event evidence for a
// policy-blocked event. The normal processing path also marks the event so a
// future downstream router can refuse forwarding it.
func (store *PostgresStore) RecordQuarantine(ctx context.Context, event domain.Event, reason string) error {
	envelope, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode quarantined event: %w", err)
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO quarantined_events
			(tenant_id, event_id, source, event_type, reason, envelope)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		ON CONFLICT (tenant_id, event_id) DO NOTHING`,
		tenantForEvent(ctx, event), event.ID, event.Source, event.Type,
		reason, string(envelope))
	if err != nil {
		return fmt.Errorf("insert quarantined event: %w", err)
	}
	return nil
}

// ListCardinalityFindings returns newest-first active/shadow findings.
func (store *PostgresStore) ListCardinalityFindings(ctx context.Context, limit int) ([]policy.Finding, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
		SELECT tenant_id, policy_name, policy_version, mode, source, event_type,
		       dimension, observed_unique, projected_unique, action, reason,
		       value_fingerprint, first_seen, last_seen
		  FROM cardinality_findings
		 WHERE tenant_id = $1
		 ORDER BY observed_at DESC
		 LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query cardinality findings: %w", err)
	}
	defer rows.Close()

	findings := make([]policy.Finding, 0, limit)
	for rows.Next() {
		var finding policy.Finding
		var action string
		if err := rows.Scan(
			&finding.TenantID,
			&finding.PolicyName,
			&finding.PolicyVersion,
			&finding.Mode,
			&finding.Source,
			&finding.EventType,
			&finding.Dimension,
			&finding.ObservedUnique,
			&finding.ProjectedUnique,
			&action,
			&finding.Reason,
			&finding.ValueFingerprint,
			&finding.FirstSeen,
			&finding.LastSeen,
		); err != nil {
			return nil, fmt.Errorf("scan cardinality finding: %w", err)
		}
		finding.Action = policy.Action(action)
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

// ListPolicyDiffs returns newest-first active-versus-shadow differences.
func (store *PostgresStore) ListPolicyDiffs(ctx context.Context, limit int) ([]policy.Diff, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
		SELECT tenant_id, observed_at, source, event_type, dimension,
		       active_policy, active_version, active_action, shadow_policy,
		       shadow_version, shadow_action, reason
		  FROM policy_shadow_diffs
		 WHERE tenant_id = $1
		 ORDER BY observed_at DESC
		 LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query policy shadow diffs: %w", err)
	}
	defer rows.Close()

	diffs := make([]policy.Diff, 0, limit)
	for rows.Next() {
		var diff policy.Diff
		var activeAction string
		var shadowAction string
		if err := rows.Scan(
			&diff.TenantID,
			&diff.ObservedAt,
			&diff.Source,
			&diff.EventType,
			&diff.Dimension,
			&diff.ActivePolicy,
			&diff.ActiveVersion,
			&activeAction,
			&diff.ShadowPolicy,
			&diff.ShadowVersion,
			&shadowAction,
			&diff.Reason,
		); err != nil {
			return nil, fmt.Errorf("scan policy shadow diff: %w", err)
		}
		diff.ActiveAction = policy.Action(activeAction)
		diff.ShadowAction = policy.Action(shadowAction)
		diffs = append(diffs, diff)
	}
	return diffs, rows.Err()
}

func tenantForPolicyFinding(ctx context.Context, tenantID string) string {
	if tenantID != "" {
		return tenantID
	}
	return security.TenantID(ctx)
}
