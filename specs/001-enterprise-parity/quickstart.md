# Operator Quickstart — Bifrost Enterprise Parity

**Branch**: `001-enterprise-parity` | **Date**: 2026-04-19
**Target**: An operator upgrading an existing Bifrost v1.5.2
deployment (or starting fresh) completes end-to-end enterprise
onboarding in ≤30 minutes (SC-002).

This quickstart assumes familiarity with Bifrost's existing HTTP
deployment and config.json conventions.

---

## Part 1 — Zero-Config Upgrade (SC-001 validation)

**Goal**: Prove that installing v1.6.0 on top of a v1.5.2 config
changes nothing.

1. Snapshot your v1.5.2 deployment's `config.json`, configstore
   database, and logstore database.
2. Upgrade the container image tag from `v1.5.2` to `v1.6.0`
   (Helm: `helm upgrade bifrost bifrost/bifrost --version 1.6.0`).
3. Let the new pod start. The first-boot migration runs
   automatically:
   - Adds `organization_id` / `workspace_id` columns to existing
     tables with a default pointing at the synthetic system
     organization.
   - Creates new empty tables (orgs, workspaces, users, etc.)
     with the synthetic row pre-populated.
   - Records migration completion in `migration_history`.
4. Replay your standard golden-set request suite (100 requests
   against the HTTP gateway). Every response body, header, and
   latency distribution should match the v1.5.2 baseline within
   tolerance.
5. Query Prometheus for metric-series names. The set should be
   identical to v1.5.2 (no new labels introduced when enterprise
   features are disabled).

**If any step above fails**, STOP and open a regression — SC-001 is
a release blocker.

---

## Part 2 — Enable Enterprise Features

**Goal**: Turn on Orgs/Workspaces, RBAC, SSO, Audit, Admin API Keys
(Train A) in a controlled way.

### 2.1 Configure SSO (OIDC example, 5 min)

Add to `config.json`:

```json
{
  "enterprise": {
    "enabled": true,
    "sso": {
      "enabled": true,
      "provider": "oidc",
      "issuer_url": "https://company.okta.com",
      "client_id": "bifrost-prod",
      "client_secret": "env.OKTA_CLIENT_SECRET",
      "auto_accept_invites": true,
      "group_role_map": {
        "bifrost-owners": "Owner",
        "bifrost-admins": "Admin"
      },
      "default_role": "Member"
    }
  }
}
```

On save, `atomic.Pointer`-backed hot-reload picks this up within
30 seconds on all instances (R-22). The first Okta-authenticated
user with `bifrost-owners` group becomes the Org Owner.

### 2.2 Create Workspaces (2 min)

Logged in as Owner:

1. Navigate to **Settings → Workspaces → Create**.
2. Create "Product", "Data Science", "Compliance" workspaces.
3. Each workspace automatically inherits org-level defaults
   (retention, guardrail policies) unless overridden.

### 2.3 Create a Custom Role (3 min)

Logged in as Owner:

1. Navigate to **Settings → Roles → New Role**.
2. Name: "ReadOnly Analyst"
3. Scopes: `metrics:read`, `completions:read` only.
4. Save.
5. Assign the role to a user via **Users → <user> → Assign Role**.

The user, on next login, will see dashboards but any write action
returns a 403 naming the missing scope.

### 2.4 Generate an Admin API Key (1 min)

1. **Settings → Admin API Keys → Create**.
2. Scopes: `virtual_keys:write`, `analytics:read`.
3. Expiration: 90 days.
4. Copy the displayed key (shown once).

Test:
```bash
curl https://bifrost.internal/v1/admin/virtual-keys \
  -H "Authorization: Bearer bf-admin-AbCdEfGhIjKl..." \
  -X POST \
  -d '{"workspace_id":"...","budget_usd_cents":10000}'
```

---

## Part 3 — Configure Governance (Train B)

### 3.1 Enable Org-Wide PII Redaction (2 min)

In `config.json`:

```json
{
  "enterprise": {
    "guardrails": {
      "central": [
        {
          "name": "org-pii",
          "type": "deterministic",
          "subtype": "pii",
          "applies_to": "both",
          "action_on_fail": "log_only",
          "config": {
            "patterns": ["ssn", "credit_card", "phone", "email"],
            "mode": "redact-in-logs-only"
          }
        }
      ]
    }
  }
}
```

Submit a request containing an SSN. Verify:
- The request succeeds end-to-end.
- The stored log entry replaces the SSN with `<REDACTED:SSN>`.
- The `bifrost_pii_redactions_total{type="SSN"}` counter
  incremented by 1.
- An audit entry recorded the redaction at INFO severity.

### 3.2 Add a Budget with Threshold Alerts (3 min)

Via UI or Admin API, attach to a virtual key:

```json
{
  "budget": {
    "period": "monthly",
    "cost_cap_usd_cents": 10000,
    "alert_thresholds_pct": [50, 75, 90]
  },
  "rate_limits": [
    {"window": "minute", "request_limit": 50},
    {"window": "hour",   "request_limit": 1000}
  ]
}
```

Drive synthetic traffic past 50% of the budget. The configured
alert destination (Slack / webhook) receives a message within 90
seconds.

### 3.3 Add a Custom Guardrail Webhook (3 min)

1. Publish an HTTPS endpoint that accepts signed payloads and
   returns `{"verdict": "deny"|"allow"|"warn", "reason": "..."}`.
2. Register it:

   ```json
   {
     "enterprise": {
       "guardrails": {
         "central": [
           {
             "name": "company-codenames",
             "type": "custom_webhook",
             "applies_to": "input",
             "execution_mode": "sync_parallel",
             "action_on_fail": "deny",
             "timeout_ms": 500,
             "on_timeout_policy": "fail_closed",
             "config": {
               "endpoint_url": "https://internal.company/guardrails",
               "signing_secret": "env.GUARDRAIL_HMAC_SECRET"
             }
           }
         ]
       }
     }
   }
   ```

3. Submit a test request containing your trigger string; verify the
   446 response, the reason text, and the guardrail_events row.

---

## Part 4 — Verify End-to-End (SC-002 validation)

Target: from the Owner login completed in §2.1 until here, total
elapsed time ≤30 minutes.

Walk the audit log (**Audit Logs** page) and confirm entries for:

- `sso_config.create` (§2.1)
- `workspace.create` × 3 (§2.2)
- `role.create` (§2.3)
- `user_role_assignment.create` (§2.3)
- `admin_api_key.create` (§2.4)
- `virtual_key.create` (§3.2)
- `guardrail_policy.create` × 2 (§3.1, §3.3)
- `pii_redaction.info` (§3.1)
- `alert.fired` (§3.2 after traffic)
- `guardrail_event.deny` (§3.3 test request)

Each entry should include actor, IP, action, before/after, and
request ID. Export the audit trail to CSV; verify fields.

---

## Part 5 — Observability Verification (SC-004)

For each enabled feature, verify:

1. **OTEL**: `otel-cli traces` shows spans named
   `bifrost.enterprise.<feature>.*` with tenant attributes.
2. **Prometheus**: scrape `/metrics` and grep for
   `bifrost_enterprise_<feature>_*` counters and histograms.
3. **Audit**: feature's representative admin action produces an
   audit entry.

Any gap is a Principle VI violation and fails the CI completeness
test (R-09).

---

## Part 6 — BYOK Onboarding (Train D; optional)

### 6.1 Enable AWS KMS for configstore secrets (5 min)

Prerequisite: An AWS KMS CMK with a key policy allowing the
Bifrost IAM role to `Encrypt` and `Decrypt`.

Add to `config.json`:

```json
{
  "enterprise": {
    "kms": [
      {
        "provider": "aws_kms",
        "key_ref": "arn:aws:kms:us-east-1:123:key/abc-def",
        "applies_to": [
          {"entity": "configstore_secrets", "enabled": true},
          {"entity": "logstore_payloads",   "enabled": false}
        ],
        "dek_cache_ttl_seconds": 900
      }
    ]
  }
}
```

Rotate an existing provider key via UI; verify in the database
that the stored ciphertext begins with the `version(1B) |
kek_ref_hash(32B)` envelope header (R-05).

### 6.2 Opt Workspace into Logstore Payload Encryption (2 min)

For a single workspace (HIPAA use case), flip
`payload_encryption_enabled: true` via workspace settings. From
that moment forward, new log records for that workspace carry
BYOK envelope encryption. Expect a ~1–2ms added write latency
for that workspace (within SC-005 envelope).

---

## Part 7 — Log Export (Train C)

### 7.1 Stream logs to S3 (5 min)

1. Create an S3 bucket.
2. Provide the Bifrost IAM role `s3:PutObject` on that bucket.
3. Configure:

   ```json
   {
     "enterprise": {
       "log_exports": [
         {
           "destination_type": "s3",
           "mode": "streaming",
           "stream_interval_seconds": 60,
           "partition_scheme": "year=YYYY/month=MM/day=DD/hour=HH",
           "record_schema": "default_full",
           "destination_config": {
             "bucket": "company-bifrost-logs",
             "region": "us-east-1",
             "prefix": "prod/"
           }
         }
       ]
     }
   }
   ```

4. Drive 1,000 requests.
5. After ≤2 minutes, query the S3 bucket and confirm gzipped
   JSONL files partitioned by hour.

Reliability validation: block bucket egress for 10 minutes;
confirm records buffer; restore egress; confirm backlog drains
to dead-letter store on persistent failure.

---

## Part 8 — Air-Gapped Smoke (Train D; optional, US19)

If deploying air-gapped, use the dedicated values file:

```bash
helm install bifrost bifrost/bifrost \
  -f helm-charts/bifrost/values-airgapped.yaml
```

The air-gapped profile explicitly supports only:
Orgs/Workspaces, RBAC, SSO (OIDC), Audit, Admin API Keys, BYOK.

Run the published `smoke-airgapped.sh` script:

```bash
./smoke-airgapped.sh
```

Expected output: all enabled features pass, eBPF egress monitor
reports zero outbound connections to non-whitelisted endpoints.

---

## Troubleshooting Pointers

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| Existing v1.5.2 virtual keys return 404 after upgrade | Migration not applied | Check `migration_history`; restart with migration debug logs |
| SSO login loops back to login page | Group-role mapping empty | Set `default_role: "Member"` |
| Guardrail timeouts at p99 | `sync_parallel` with slow partner | Switch to `async` or raise `timeout_ms` |
| Budget threshold alert misses | Alert destination not reachable | Check `alert_events` for last_error |
| BYOK writes fail silently | KMS permission missing | Bifrost role needs `Encrypt`; failures surface in audit |
| Dashboard slow | Rollup job lagging | Force-run `rollup-exec-metrics.sh`; check DB IOPS |

---

## Next Commands

After this quickstart, the feature is ready for real traffic. The
next Spec Kit invocation generates tasks.md:

```
/speckit-tasks
```

Tasks will decompose into:
- Setup (module scaffolding, schema migrations, CI gates)
- Foundational (tenancy primitive, crypto primitive, audit sink)
- Per-user-story implementation (23 stories)
- Polish (docs, charts, terraform module, air-gapped smoke)
