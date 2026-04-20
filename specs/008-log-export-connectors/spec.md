# Feature Specification: Log Export Connectors

**Feature Branch**: `008-log-export-connectors`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Ship the **config + UI** half of log-export connectors (Datadog,
BigQuery). Each connector stores its credentials and its enabled
toggle; the actual log-forwarding pipeline is a phase 2 plugin that
reads the same configs on boot. Shipping storage + UI now lets
customers start filling in their credentials instead of watching a
placeholder.

## User Scenarios

### US1 — SRE records Datadog API key (P1)

**As an** SRE
**I want** to enter our Datadog API key and site (datadoghq.com /
eu.datadoghq.com) into the Datadog connector page
**So that** when the log-forwarding plugin ships, it picks up my
credentials without a redeploy.

## Functional Requirements

- **FR-001**: New table `ent_log_export_connectors` with
  `{id, type (datadog|bigquery), name, config (JSON), enabled,
  updated_at}`. Multiple connectors per type are allowed; typical
  deployments have one.
- **FR-002**: CRUD API at `/api/log-export/connectors`:
  - `GET /?type=datadog` → list (optionally filtered by type)
  - `POST /` → create
  - `PATCH /:id` → update
  - `DELETE /:id` → delete
- **FR-003**: `/workspace/observability` Datadog and BigQuery tabs
  render real forms backed by the new API (currently
  FeatureStatusPanel stubs).
- **FR-004**: Configs are stored but NOT YET CONSUMED by a running
  exporter — the UI clearly marks "Log forwarding activates when the
  log-export plugin ships (spec 008.2)". This is v1 honesty.

## Success Criteria

- **SC-001**: An operator can save a Datadog API key and reload the
  page to see it persisted (API key masked).
- **SC-002**: SC-020 scanner no longer matches
  `datadogConnectorView.tsx` or `bigqueryConnectorView.tsx`.
