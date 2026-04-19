# Webhook Payload Contracts — Bifrost Enterprise Parity

Shapes for outbound HTTP calls Bifrost makes to customer-owned
endpoints, and inbound callbacks Bifrost accepts from IdPs and
partners.

All outbound webhooks are HMAC-signed. All inbound SCIM/SAML/OIDC
flows follow the relevant RFC.

---

## 1. Outbound: Alert Webhook

**Header**: `x-bifrost-signature: t=<unix_seconds>, v1=<hex_hmac_sha256>`
**Secret**: per-destination, set at destination creation time.
**Signed payload**: `t . request_body` (dot-separated)
**Retry policy**: 3 attempts, exponential backoff (1s, 4s, 16s).

```json
{
  "event": "alert.fired",
  "version": 1,
  "id": "01JBFA2K…",
  "organization": { "id": "…", "name": "Acme Corp" },
  "rule": { "id": "…", "name": "P95 latency > 3s" },
  "severity": "critical",
  "observed_value": 3842.1,
  "threshold": 3000,
  "metric": "latency_p95",
  "dashboard_url": "https://bifrost.internal/d/latency?ws=…",
  "fired_at": "2026-04-19T14:30:00Z"
}
```

`alert.resolved` variant carries `resolved_at` and `duration_seconds`.

---

## 2. Outbound: Slack Destination

Uses the Slack Incoming Webhook URL format (no HMAC signature —
the URL itself is the shared secret; store the URL in
`alert_destinations.config_json` 🔐).

```json
{
  "text": "🚨 [Bifrost] P95 latency > 3s fired",
  "blocks": [
    {
      "type": "section",
      "text": { "type": "mrkdwn", "text": "*Rule:* P95 latency > 3s" }
    },
    {
      "type": "section",
      "fields": [
        { "type": "mrkdwn", "text": "*Observed:* 3842ms" },
        { "type": "mrkdwn", "text": "*Threshold:* 3000ms" },
        { "type": "mrkdwn", "text": "*Window:* 300s" },
        { "type": "mrkdwn", "text": "*Organization:* Acme Corp" }
      ]
    },
    {
      "type": "actions",
      "elements": [
        { "type": "button", "text": { "type": "plain_text", "text": "Open dashboard" }, "url": "https://bifrost.internal/d/…" }
      ]
    }
  ]
}
```

---

## 3. Outbound: Budget Threshold Webhook

```json
{
  "event": "budget.threshold_reached",
  "version": 1,
  "organization_id": "…",
  "workspace_id": "…",
  "virtual_key_id": "…",
  "virtual_key_name": "team-alpha-prod",
  "threshold_pct": 75,
  "current_spend_usd": 75.12,
  "budget_usd": 100.00,
  "period_start": "2026-04-01T00:00:00Z",
  "period_end":   "2026-05-01T00:00:00Z",
  "fired_at": "2026-04-19T14:30:00Z"
}
```

Uses the same HMAC signature pattern as Alert Webhook.

---

## 4. Outbound: Custom Guardrail Webhook

**Bifrost → customer endpoint**, synchronous, per-request.

Request:

```json
{
  "event": "guardrail.evaluate",
  "version": 1,
  "request_id": "…",
  "organization_id": "…",
  "workspace_id": "…",
  "virtual_key_id": "…",
  "policy_id": "…",
  "phase": "input|output",
  "content": {
    "messages": [ { "role": "user", "content": "…" } ],
    "metadata": { "x-session-id": "…" }
  },
  "provider": { "name": "openai", "model": "gpt-4o" },
  "timestamp": "2026-04-19T14:30:00Z"
}
```

**Header**: `x-bifrost-signature: t=<unix>, v1=<hex_hmac_sha256>`
signed over `t . request_body`.

Expected response (≤ `timeout_ms`):

```json
{
  "verdict": "allow|deny|warn",
  "reason": "free text (shown to caller on deny)",
  "redactions": [
    { "type": "SSN", "start": 45, "end": 56, "replacement": "<REDACTED:SSN>" }
  ]
}
```

- If the response cannot be parsed, the guardrail-timeout policy
  (`fail_open` or `fail_closed`) takes effect.
- `redactions[]` is optional; when supplied, Bifrost applies the
  replacements to request/response content before passing along.

---

## 5. Outbound: Data Lake Export (S3 object layout)

Not a webhook; file-drop contract.

Path: `s3://<bucket>/<prefix>/year=YYYY/month=MM/day=DD/hour=HH/
part-NNNN.<ext>.gz`

Supported `<ext>`:

- `jsonl` — one JSON object per line, UTF-8, gzip-compressed.
- `parquet` — columnar (curated exports, US23).
- `avro` — MongoDB-style binary.

Object metadata:

- `x-amz-meta-bifrost-org-id`
- `x-amz-meta-bifrost-workspace-id` (optional)
- `x-amz-meta-bifrost-record-count`
- `x-amz-meta-bifrost-schema-version`

Record schema (default `default_full`):

```json
{
  "id": "…",
  "organization_id": "…",
  "workspace_id": "…",
  "virtual_key_id": "…",
  "request_id": "…",
  "provider": "openai",
  "model": "gpt-4o",
  "status_code": 200,
  "latency_ms": 823,
  "input_tokens": 412,
  "output_tokens": 188,
  "cost_usd": 0.0021,
  "created_at": "2026-04-19T14:30:00Z",
  "prompt_redacted": true,
  "response_redacted": true,
  "feedback_score": 4.5,
  "metadata": { "…": "…" }
}
```

Payload bodies (`prompt`, `response`) are included only when the
workspace has `export_include_bodies=true` AND the operator has
accepted the included-payloads disclosure. Otherwise the schema
marks `*_redacted=true` and omits the body.

---

## 6. Inbound: SAML 2.0 Assertion Consumer

Endpoint: `POST /v1/admin/sso/saml/callback`
Content-Type: `application/x-www-form-urlencoded`
Body: `SAMLResponse=<base64 signed assertion>`
Validation:

- Issuer matches the configured IdP entity ID.
- Signature validates against the configured certificate.
- `NotBefore` / `NotOnOrAfter` in window (± 5 min clock skew).
- `AudienceRestriction` matches the configured SP entity ID.
- Assertion signed and/or Response signed per IdP config.

On success: issue session cookie, redirect to `/` or the deep
link carried in `RelayState`.

---

## 7. Inbound: OIDC Authorization Code Callback

Endpoint: `GET /v1/admin/sso/oidc/callback?code=…&state=…`
Validation:

- `state` matches a recently-issued CSRF token bound to the
  session.
- Token exchange against `issuer_url`'s token endpoint.
- `id_token` signature validated against JWKS.
- `aud` claim equals the configured `client_id`.
- `nonce` matches the value issued with the authorization
  request.
- Mandatory claims: `sub`, `email`, optional `groups`.

On success: issue session cookie, redirect.

---

## 8. Inbound: SCIM 2.0 (RFC 7644 subset)

Bifrost implements the endpoints listed in
`admin-api.openapi.yaml` under the `SCIM` tag. Conformance notes:

- Filters supported: `eq`, `ne`, `co`, `sw`, `ew`, `pr`, `and`,
  `or`.
- PATCH ops: `add`, `replace`, `remove`.
- Pagination: `startIndex`, `count`.
- Bulk ops: not supported in v1 (returns 501 on
  `/Bulk`).
- Auth: `Authorization: Bearer <scim_bearer_token>` (configured
  per SSO setup).

Response shapes follow RFC 7644 exactly; see the public
SCIM specification.
