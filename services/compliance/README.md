# Compliance & Insights Service

Owns the retention policy, the pipeline view of every job opening, and the erasure and access
audit logs. It serves the dashboard and stale-position flags (UC-4) and retention and erasure
(UC-5), and keeps the access audit log the gateway reports to (FR-0.4).

**Status:** every operation in the Service–Operations–Collaborators table is built and tested,
and the service runs in the root compose stack. The gateway routes `/compliance/*` here and
reports role rejections, Hiring publishes the pipeline events and serves its erasure, so UC-4
and UC-5 work end to end through the API. The web app has no screens for them yet.

| | |
|---|---|
| API | through the gateway, http://localhost:8000/compliance/…; directly on http://localhost:8083 |
| Language | Go, laid out like the Hiring Service (domain → usecase → adapters → app) |
| Owner | Patiphon Puntusin |

## Decisions it rests on

- **ADR-015:** pipeline events arrive on RabbitMQ, not REST.
- **ADR-016:** this service orchestrates every erasure, and keeps it pending until each holder
  confirms.
- **ADR-017:** both sweeps run on a timer inside this service, under a database lock.
- **ADR-014:** the resume file is in the deployment's own MinIO, and Hiring deletes it.

## API

| Method and path | Operation | Who (enforced at the gateway) |
|---|---|---|
| `GET /compliance/dashboard` | `getPipelineDashboard()`: counts per stage, days open, flag (FR-4.1) | either role |
| `GET`/`PUT /compliance/retention-policy` | `setRetentionPolicy()`: period, anchor, expiry action, warning window (FR-5.1). 404 until one is set | Admin |
| `GET /compliance/held-expiries` | `listHeldExpiries()` (FR-5.6) | either role |
| `POST /compliance/held-expiries/{candidateId}` `{"decision":"erase"\|"keep"}` | `resolveHeldExpiry()`. "keep" asks again after the warning window | either role |
| `POST /compliance/erasures` `{"candidate_id"}` | `eraseCandidate()` now (FR-5.7). **200** erased, **202** pending | Admin |
| `GET /compliance/erasures[?status=pending\|erased]`, `GET /compliance/erasures/{ref}` | `getErasureAudit()` (FR-5.5) | Admin |
| `GET /compliance/access-audit` | the access audit log (NFR-12) | Admin |
| `POST /compliance/sweeps/staleness`, `POST /compliance/sweeps/retention` | `evaluateStaleness()`, `evaluateRetention()` now; 409 if one is running | Admin |
| `POST /internal/access-rejections` | `recordAuthorisationRejected()`: for the gateway, never routed by it | gateway |
| `GET /healthz`, `GET /readyz` | liveness; readiness checks PostgreSQL and the broker | |

## How retention decides

There is **no default policy**. Until an Admin sets one, the sweep only retries pending
erasures, because erasing personal data under a period nobody chose is worse than keeping it.

On each sweep, every candidate in Resume Processing's register is judged by the policy:
- The period runs from the collection date or the last pipeline activity.
- Inside the warning window, the responsible Recruiter is emailed once per expiry date (FR-5.2).
- Past expiry, the candidate is erased (FR-5.3), unless they are linked to an opening that is
  not closed. Then the expiry is **held** for a Recruiter to decide (FR-5.6).
- A candidate linked to an opening this service has never heard of is also held. When events
  are missing, the safe mistake is to ask, not to erase.

An erasure asks Hiring and Resume Processing in turn. It is *erased* only when both confirm, and
only then is the candidate id removed from the record, leaving a random reference (FR-5.5). The
pipeline counts survive (FR-5.4). Today "delete" and "anonymise" do the same at the holders. The
difference is recorded on the audit entry, and what an anonymised record should keep at a holder
is open (ADR-016).

## Run it

```sh
docker compose up -d --build compliance       # with its database, RabbitMQ and Mailpit
curl localhost:8083/readyz
```

Emails go to Mailpit, at http://localhost:8025. Set `COMPLIANCE_SWEEP_INTERVAL=1m` in `.env` to
watch the sweeps run, or call the sweep endpoints.

Tests need only Docker, because the compiler runs in a container:

```sh
make test       # domain, use cases, HTTP, holder clients, email
make test-all   # plus the PostgreSQL integration test; needs: docker compose up -d compliance-db
```

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8083` | |
| `DATABASE_URL` | **required** | The service will not start without it: the audit logs must not live in memory |
| `RABBITMQ_URL` | — | Without it no pipeline events arrive |
| `RESUME_PROCESSING_URL` | — | The candidate register and profile erasure |
| `HIRING_URL` | — | Hiring's erasure |
| `SMTP_ADDR`, `SMTP_FROM`, `SMTP_USERNAME`, `SMTP_PASSWORD` | —, `HireAssist <no-reply@hireassist.local>` | Without `SMTP_ADDR`, notices are logged instead of sent |
| `NOTIFY_FALLBACK_TO` | — | Receives notices for an opening with no known Recruiter |
| `SWEEP_INTERVAL` | `1h` | At least `1m` |

Each missing optional dependency is named in one log line at startup.

## Layout

```
cmd/compliance/          main: configuration from the environment, HTTP server
internal/domain/         retention policy and evaluation, opening view and staleness, erasure record
internal/usecase/        Pipeline, Retention, Erasures, AccessAudit, and the ports they declare
internal/adapter/
  httpapi/               REST
  postgres/              every storage port, the schema, the sweep lock
  rabbitmq/              the pipeline.events consumer
  holders/               REST clients for Resume Processing and Hiring
  email/                 SMTP, or log
  memory/                in-memory storage, for tests
  system/                clock and random references
internal/app/            composition root and the scheduler
```

## Known limits

- A notice that fails to send is not retried. The flag or warning mark is still recorded
  (ADR-017).
- There is no Recruiter on a job opening in Hiring today, so `recruiter` in the events is
  optional and notices fall back to `NOTIFY_FALLBACK_TO`.
- One consumer keeps events in order. A second replica would need per-opening ordering
  (ADR-015).
