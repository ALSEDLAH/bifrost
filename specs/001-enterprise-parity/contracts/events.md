# Event Contracts — Bifrost Enterprise Parity

Internal event shapes emitted by enterprise plugins. These are not
HTTP endpoints; they flow through the plugin pipeline, the audit
sink, the alert pipeline, and the log export sinks.

Canonical JSON. All timestamps ISO 8601 UTC. All IDs UUID.

---

## audit.entry

Emitted by every enterprise plugin for administrative or
governance actions. Persisted in `audit_entries` (logstore).

```json
{
  "event": "audit.entry",
  "version": 1,
  "id": "01JBFA2K…",
  "organization_id": "…",
  "workspace_id": "…|null",
  "actor": {
    "type": "user|admin_api_key|service_account|system",
    "id": "…|null",
    "display": "user@company.com or bf-admin-AbCdEfGh",
    "ip": "203.0.113.45"
  },
  "action": "virtual_key.create|role.assign|guardrail.block|...",
  "resource": {
    "type": "virtual_key|role|guardrail_policy|…",
    "id": "…|null"
  },
  "outcome": "allowed|denied|error",
  "reason": "free text explaining outcome when non-allowed",
  "before": { "…": "prior state subset or null" },
  "after":  { "…": "new state subset or null" },
  "request_id": "…|null",
  "created_at": "2026-04-19T14:30:00Z"
}
```

Canonical `action` verbs:

| Prefix | Examples |
|--------|----------|
| organization.* | organization.create, organization.update |
| workspace.*    | workspace.create, workspace.delete |
| user.*         | user.invite, user.suspend, user.delete |
| role.*         | role.create, role.update, role.delete |
| user_role_assignment.* | user_role_assignment.create, .delete |
| admin_api_key.* | admin_api_key.create, .rotate, .revoke |
| service_account.* | service_account.create, .rotate, .revoke |
| sso.*          | sso.config_update, sso.login, sso.login_failed |
| scim.*         | scim.user_create, scim.user_suspend, scim.user_delete |
| virtual_key.*  | virtual_key.create, .update, .revoke |
| budget.*       | budget.threshold_reached, budget.exceeded |
| rate_limit.*   | rate_limit.exceeded |
| guardrail_policy.* | guardrail_policy.create, .update, .delete |
| guardrail.*    | guardrail.allow, guardrail.deny, guardrail.warn, guardrail.retry, guardrail.fallback, guardrail.timeout, guardrail.error |
| pii_redaction.* | pii_redaction.applied, pii_redaction.fail_closed |
| alert.*        | alert.rule_create, .rule_update, .fired, .resolved |
| log_export.*   | log_export.config_update, .flushed, .failed, .dead_lettered |
| retention.*    | retention.policy_update, .deletion_run |
| prompt.*       | prompt.create, .version_create, .publish, .render |
| config.*       | config.create, .version_create, .publish |
| canary.*       | canary.start, canary.promoted, canary.aborted |
| kms.*          | kms.config_update, kms.rotate, kms.revoke_observed |
| break_glass.*  | break_glass.login |

---

## guardrail.event

Emitted by every guardrail execution regardless of outcome.
Persisted in `guardrail_events` (logstore).

```json
{
  "event": "guardrail.event",
  "version": 1,
  "id": "…",
  "organization_id": "…",
  "workspace_id": "…|null",
  "virtual_key_id": "…|null",
  "request_id": "…",
  "policy_id": "…",
  "phase": "input|output",
  "outcome": "allow|deny|warn|retry|fallback|error",
  "reason": "pattern matched: SSN",
  "latency_ms": 23,
  "metadata": {
    "pattern_type": "regex|llm|partner|webhook",
    "partner": "aporia|pillar|patronus|sydelabs|pangea|null",
    "redaction_applied_count": 0
  },
  "created_at": "2026-04-19T14:30:00Z"
}
```

---

## alert.fired / alert.resolved

Emitted by the alert dispatcher when a rule transitions.

```json
{
  "event": "alert.fired",
  "version": 1,
  "id": "…",
  "organization_id": "…",
  "rule_id": "…",
  "rule_name": "P95 latency > 3s for 5m",
  "severity": "info|warning|critical",
  "observed_value": 3842.1,
  "threshold": 3000,
  "metric": "latency_p95",
  "window_seconds": 300,
  "dashboard_url": "https://bifrost.internal/d/latency?ws=…",
  "fired_at": "2026-04-19T14:30:00Z"
}
```

`alert.resolved` carries the same shape with `resolved_at` and a
`duration_seconds`.

---

## export.batch

Emitted by each log-export sink on batch flush (success or
failure). Optional — persisted only when `export_audit_enabled`
is true.

```json
{
  "event": "export.batch",
  "version": 1,
  "destination_id": "…",
  "destination_type": "s3|azure_blob|gcs|mongodb|otlp",
  "records": 1234,
  "bytes": 589824,
  "outcome": "success|retry|dead_lettered",
  "attempt": 1,
  "target_path": "s3://company-bifrost-logs/prod/year=2026/…/part-0001.jsonl.gz",
  "flushed_at": "2026-04-19T14:30:00Z"
}
```

---

## canary.report

Emitted daily (and on-demand) summarizing a canary run.

```json
{
  "event": "canary.report",
  "version": 1,
  "config_id": "…",
  "canary_id": "…",
  "window": {
    "from": "2026-04-18T00:00:00Z",
    "to":   "2026-04-19T00:00:00Z"
  },
  "primary": {
    "target": {"provider": "openai", "model": "gpt-4o"},
    "traffic_pct": 90,
    "requests": 47821,
    "latency_p50_ms": 620,
    "latency_p95_ms": 1830,
    "cost_usd": 213.48,
    "error_rate": 0.003,
    "feedback_score_avg": 4.3
  },
  "canary": {
    "target": {"provider": "openai", "model": "gpt-5-preview"},
    "traffic_pct": 10,
    "requests": 5292,
    "latency_p50_ms": 710,
    "latency_p95_ms": 2140,
    "cost_usd": 41.22,
    "error_rate": 0.002,
    "feedback_score_avg": 4.5
  }
}
```

---

## Event Versioning

Every event carries a top-level `version` integer. Changes to
event shapes follow the Constitution's Non-Breaking principle:

- New optional fields → no version bump.
- Removed or renamed fields → version bump + dual-emit for
  at least one minor release cycle.

Consumers (log-export sinks, SIEM parsers, customer ETL) should
handle unknown fields gracefully.
