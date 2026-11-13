# Pipeline events: Hiring → Compliance & Insights (RabbitMQ)

The contract for UC-4 between the Hiring Service (publisher) and the Compliance & Insights Service
(consumer), decided in ADR-015. Change it in one PR that both owners approve.

## Queues

| Queue | Direction | Declared by | Arguments |
|---|---|---|---|
| `pipeline.events` | Hiring → Compliance | either side | durable, none |
| `pipeline.events.dlq` | internal to Compliance | Compliance | durable, none |

- Messages go through the default exchange, routed by queue name, as the screening queues do.
- Publish with `persistent: true` and `contentType: application/json`, with the event id as the
  message id and the correlation id in the `x-correlation-id` header.
- Delivery is at least once. **`event_id` must be unique per event**, and sending the same event
  twice is harmless: every entry is counted once per resume and stage.
- An event Compliance can never apply (not JSON, an unknown type, a required field missing) is
  moved to `pipeline.events.dlq` with the reason in `x-refused-because`. Any other failure, such
  as the database being unavailable, puts it back to be delivered again.

## Body

```json
{
  "event_id": "evt-7f3a…",
  "type": "resume.screened",
  "occurred_at": "2026-11-13T09:30:00Z",
  "job_opening_id": "job-001",
  "resume_id": "res-001",
  "candidate_id": "cand-001",
  "correlation_id": "6f1c…"
}
```

| Field | Required | Meaning |
|---|---|---|
| `event_id` | yes | Unique per event |
| `type` | yes | One of the types below |
| `occurred_at` | yes | When it happened in Hiring (RFC 3339). Days open and last activity are counted from it |
| `job_opening_id` | yes | The opening |
| `title` | `opening.opened` | The opening's title, for the dashboard and notices |
| `expected_time_to_fill_days` | `opening.opened`, > 0 | The staleness rule's threshold (FR-4.3) |
| `recruiter` | no | Email of the responsible Recruiter. Without it, notices go to the deployment's fallback recipient |
| `resume_id` | `resume.*`, `candidate.shortlisted` | Entries are counted by resume, so the count keeps no candidate id after erasure |
| `candidate_id` | from `resume.screened` on | Links the activity to the candidate, for retention counted from last activity |
| `correlation_id` | no | Follows the request through the logs |

| `type` | Hiring publishes it when | Effect in Compliance |
|---|---|---|
| `opening.opened` | an opening is created (FR-1.7). Publish again with new values if the time-to-fill changes | the opening appears on the dashboard |
| `opening.paused` / `opening.resumed` | FR-1.8 | status shown; a paused opening is excluded from staleness while paused (UC-4, 4a) |
| `opening.closed` | FR-1.8 | off the dashboard, flag cleared (FR-4.5). Final: later events for it are ignored |
| `resume.submitted` | a resume is accepted into a batch | counts as **applied** |
| `resume.screened` | a screening result is recorded as scored | counts as **screened** |
| `candidate.shortlisted` | FR-2.9 | counts as **shortlisted** |

An extra field is ignored. An entry for an opening whose `opening.opened` Compliance never
received is kept, and the opening is shown without a title until one arrives.

# REST calls Compliance makes, and one it serves

These are not on the broker, but they are part of the same contract.

| Call | Served by | Success | Meaning |
|---|---|---|---|
| `GET /candidates` | Resume Processing | `200` | `listCandidateProfiles()`: the candidate register; `created_at` is the collection date |
| `DELETE /candidates/{candidateId}` | Resume Processing | `200`, or `404` (nothing held) | `eraseCandidateProfile()` |
| `DELETE /candidates/{candidateId}` | Hiring | **`204` only** | `eraseHiringData()`: screening results, decisions, guides and the resume file. Only a 204 counts, because a Hiring without the route answers 404 too (ADR-016) |
| `POST /internal/access-rejections` | Compliance; called by the gateway, for 403s only | `201` | `recordAuthorisationRejected()`: `{member_id, role, required_role, method, path, occurred_at}` |
