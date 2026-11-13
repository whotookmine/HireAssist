# ADR-015: Pipeline events reach Compliance & Insights through RabbitMQ

**Date:** 2026-11-13
**Deciders:** Patiphon Puntusin

> In the context of the pipeline dashboard and stale-position flags (UC-4), facing a choice
> between Hiring calling the Compliance & Insights Service directly for every pipeline event and
> publishing those events on the message broker, we decided to **publish them on a RabbitMQ
> queue that Compliance & Insights consumes**, so that Hiring keeps working when Compliance is
> down and no event is lost, accepting that the dashboard trails what has happened by however
> long the queue takes to drain.

---

## Summary

### Issue

The Compliance & Insights Service keeps its own view of every job opening: how many candidates
applied, were screened and were shortlisted, and how long the opening has been open (FR-4.1,
the dashboard). It builds that view from events Hiring reports as they happen, rather than by
querying Hiring, because the dashboard spans services and has to be maintained incrementally
(ADR-001).

Until now the documents had Hiring call `recordPipelineEvent()` on Compliance over REST, and
listed that as an open decision. A direct call has two faults:

- **Compliance becomes a synchronous dependency of Hiring's write path.** If Compliance is slow
  or down, then creating an opening, submitting a batch or shortlisting a candidate is slow or
  fails, even though none of them needs Compliance to succeed.
- **A lost call is a lost event.** If the call fails after Hiring has committed its own change,
  Hiring must either retry it itself or the dashboard is wrong for good. Neither service has a
  way to notice.

RabbitMQ is already in every deployment, carrying the screening handover
([ADR-013](ADR-013-rabbitmq-for-the-screening-handover.md)), and Hiring already has an outbox
relay that publishes to it.

### Decision

**Hiring publishes one message per pipeline event to a durable queue, `pipeline.events`, and
Compliance & Insights consumes it.** The event types are:
- the opening's lifecycle: opened, paused, resumed, closed;
- a resume submitted (the "applied" stage);
- a resume screened;
- a candidate shortlisted.

- Messages go through the default exchange, routed by queue name, like the screening queues.
  The queue is durable and has no arguments, so either service may declare it.
- Delivery is at least once. Every event carries an `event_id`. Each entry is counted once per
  resume and stage, so a redelivered message changes nothing.
- An event Compliance can never apply goes to its own dead-letter queue, `pipeline.events.dlq`,
  with the reason in a header. Any other failure puts the message back on the queue to be
  delivered again.
- The contract is written down beside the consumer, in the message-contract file of the
  Compliance & Insights Service, as the screening contract is beside Resume Processing.

The access audit entries the gateway reports (`recordAuthorisationRejected()`) stay a REST call:
the gateway has no broker connection, and a rejected request is already answered whether or not
the audit write succeeds.

### Status

**Accepted**

### Group

Communication

---

## Details

### Assumptions

- A dashboard that lags real events by seconds is acceptable. Nobody acts on the count of
  screened resumes within the second it changes.
- Pipeline events number in the hundreds per day for a small business, far below anything the
  broker notices.
- Hiring publishes events through the outbox relay it already runs. This record decides the
  channel and the contract; how Hiring publishes is up to Hiring's owner.

### Constraints

- The course requires a message broker. ADR-013 introduced it for one handover; this is its
  second use.
- No service reads another's data store (ADR-001), so Compliance cannot read Hiring's tables to
  build the dashboard.

### Positions

1. **REST call from Hiring for each event** (`recordPipelineEvent()`), as the documents had
   it. Easy to trace, with an immediate error, but tied to Compliance being up.
2. **REST call, with Hiring retrying from its own outbox.** Removes the lost event. Hiring then
   keeps a second retry schedule that does what the broker already does.
3. **A queue on the existing broker.** *(Selected.)*
4. **Compliance polls Hiring for changes since a cursor.** No new channel is needed. Hiring
   needs a change feed it does not have, and Compliance finds out about changes late by design.

### Argument

**Against a direct REST call (1):** the dashboard is a reporting feature. Letting it slow down,
or fail, the operations it reports on gets the dependency backwards.

**Against REST with an outbox retry (2):** it would work, but it builds a second, smaller broker
inside Hiring: a retry timer, a backoff and a parking place for events that keep failing.
ADR-013 has already moved Hiring's screening handover off exactly that kind of hand-built retry.

**Against polling (4):** Hiring would need a change feed. Compliance would see changes only as
often as it polls, and staleness flags and counts would be late by that interval. It adds the
most code to get the worst freshness.

**For the broker (3):** it is already there, already operated and already watched. Hiring
already publishes to it from an outbox. Compliance being down only means messages wait. Both
redelivery and dead-lettering already work the way the team built them for screening.

### Implications

**What this buys us**

- Hiring's write path no longer depends on Compliance. A Compliance outage delays the dashboard
  and nothing else.
- No event is lost between the two services: it is either waiting in the queue or kept in the
  dead-letter queue with a reason.
- The broker now carries two flows, which makes the course's message-broker requirement more
  than a single use case.

**What it costs us**

- **The dashboard is eventually consistent.** Counts trail events by however long the queue
  takes to drain, and by much longer if Compliance has been down.
- **Event order is only per queue.** One publisher and one consumer keep events in order today.
  A second consumer replica would need ordering by opening, or the projection would have to
  tolerate events arriving out of order. A closed opening already ignores later events, so the
  worst case today is a count that arrives early.
- **One more contract to keep in step.** The event types and fields must change in one pull
  request that both owners approve, like the screening contract.
- **Hiring has work to do.** Until Hiring publishes these events, the dashboard is empty, and
  every candidate linked to an opening is held rather than expired (ADR-016). That is the safe
  failure, but it means UC-4 cannot be demonstrated end to end until Hiring publishes.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md): the dashboard is a projection maintained from
  events, and no service reads another's store. This record changes how the events travel:
  through the broker, not REST.
- [ADR-013](ADR-013-rabbitmq-for-the-screening-handover.md): the same broker, the same
  conventions (default exchange, durable queues with no arguments, a consumer-owned dead-letter
  queue).
- [ADR-002](ADR-002-async-screening-pipeline.md): at-least-once delivery with idempotent
  handling, applied here to events.

### Related requirements

- **UC-4**, monitor the hiring pipeline: FR-4.1 (counts per stage and days open), FR-4.2
  (staleness evaluated on a schedule whether or not anyone looks), FR-4.3 (flag an opening open
  longer than its expected time-to-fill), FR-4.5 (clear the flag on close).
- **UC-5**: the date of a candidate's last activity, one of the two dates retention can count
  from (FR-5.1), comes from these events.
- **NFR-09**, availability: a Compliance outage no longer reaches the use cases Hiring serves.
- Course requirement: a message broker, now used for two flows.

### Related artifacts

The Service–Operations–Collaborators table (Hiring publishes; Compliance consumes) and the
container diagram.
