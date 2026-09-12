# ADR-002: One independently retried unit of work per resume, held in a claimable work table

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of batch resume screening over a single wire protocol, facing per-file processing
> that takes seconds to minutes and depends on an external model that will fail, we decided to make
> each resume an independently claimable row in the screening service's own work table, handed over
> by an acknowledged REST call and answered by a REST callback, with bounded retry and a terminal
> failure state, to achieve throughput that rises with worker count and a batch that survives
> partial failure and a model outage without losing work, accepting that results become eventually
> consistent, that a resume may occasionally be processed — and paid for — twice, and that we are
> writing by hand what a message broker would have supplied.

---

## Summary

### Issue

Batch screening is what the project exists for, and it cannot be built the obvious way.

A recruiter drops 20–200 resumes onto an open position. Each must be parsed out of a PDF,
normalised into a profile, scored against weighted criteria, and given a written justification.
Scoring is at least one model call: seconds when healthy, tens of seconds for a long resume,
unbounded when the provider is rate-limiting. A batch of 200 is minutes of work in the good case.
No HTTP request survives that, and no recruiter should watch a spinner while it happens.

**Failure is the normal path, not the exceptional one.** A corrupt file or a scanned image with
no text layer; the scoring service unavailable; rate limits, timeouts, truncated responses. In a
batch of 200, something failing is close to certain.

That makes the unit of failure the sharp question. A recruiter who uploads 200 resumes and gets
nothing because file 147 was a scanned image has been failed by the system. Getting 199 results
and one entry marked *needs manual review* is not a degraded outcome — it is the correct one.

**Every crossing between services is REST**, decided when the boundaries were drawn. So whatever
holds a resume between "accepted" and "scored" has to live inside a service, not between them.
This record is about what that thing is.

This is also where the demonstrated quality attribute lives: throughput must rise in proportion as
workers are added, and no accepted resume may ever be lost. Both claims are falsifiable here, and
the second keeps the first honest.

### Decision

**Each resume is one row of work. The batch is a count, not a unit of work.**

1. **Accept and return.** Hiring validates the upload, rejecting anything that is not a PDF,
   stores the files, creates a batch with one `pending` entry per resume, and responds
   `202 Accepted` with the batch ID within two seconds regardless of batch size.
2. **Hand each resume over individually.** Hiring calls Resume Processing once per resume —
   *(batch id, resume id, file reference, criteria snapshot, callback address)*. The call returns
   as soon as the work is durably recorded; it does not wait for a score. Handover is retried by
   Hiring until acknowledged, so an entry is never left with no owner.
3. **The work table is the queue.** Resume Processing writes each handover as a row in its own
   PostgreSQL schema, holding the request, its state, its attempt count and its next-eligible
   time. Nothing else stores in-flight screening work, and no other service reads this table.
4. **Consume in parallel.** N replicas of Resume Processing poll their own table, claiming a
   bounded number of eligible rows at a time with `SELECT … FOR UPDATE SKIP LOCKED` so no two
   workers take the same resume. One row is one resume: parse → score → persist → call back.
   Concurrency changes by changing N, and nothing else.
5. **Guarantee delivery, not exactly-once.** A claim is a row-level lock, not a removal. A worker
   that crashes mid-resume releases its lock with the row still unfinished, so another worker
   picks it up — at-least-once, deliberately. The same holds for the callback: it is retried with
   backoff until Hiring acknowledges it.
6. **Make redelivery harmless.** Results are upserted on `(batch_id, resume_id)` in both services,
   so a re-processed resume or a repeated callback overwrites its own earlier result rather than
   creating a second one.
7. **Classify failures before retrying.** A *permanent* failure — a corrupt file, a scanned image
   with no text layer — is never retried; the row goes to `needs_manual_review` with the reason. A
   *transient* failure — 429, 5xx, a broken connection, or a scoring attempt past its 60-second
   bound — is retried a bounded number of times, the row's next-eligible time pushed out by
   exponential backoff with jitter.
8. **Park what does not recover.** Once the retry budget is spent the row moves to a terminal
   `failed` state with its last error and the entry is marked `needs_manual_review`. Failed rows
   stay in the table and are listable and replayable by an operator; nothing is silently scored
   zero and nothing is deleted.
9. **Completion is computed.** A batch is complete when every entry is terminal — `scored` or
   `needs_manual_review` — and reaching that state triggers the batch-finished notification. There
   is no all-or-nothing outcome. A periodic sweep re-arms rows left claimed past a lease bound and
   re-sends handovers for entries Hiring still shows as `pending`, which is the defence against
   work that vanishes on either side of the boundary.
10. **Stream results to the screen.** Each callback updates the shortlist view over server-sent
    events, so the ranking fills in as results land.

**There is no message broker.** Durability comes from a database transaction, ordering does not
matter, and the retry, backoff and terminal-failure behaviour above is application code in one
service.

### Status

**Accepted**

### Group

Communication · Scalability · Reliability

---

## Details

### Assumptions

- A batch is 20–200 resumes, each file at most 10 MB and usually far smaller. Nothing here
  assumes tens of thousands of files.
- Every service-to-service call is REST over HTTP/JSON, and no broker is deployed.
- The model provider will be unavailable or rate-limiting some of the time, with no fallback to
  switch to. Waiting is the only recovery.
- Scoring one resume is independent of scoring any other, so ordering does not matter and
  parallelism is free.
- Re-running a resume through the model costs money but is otherwise harmless — the output is
  regenerable, not authoritative.
- Polling a table at this rate is negligible load. At batches of 200 and a handful of workers,
  claim queries are indexed single-row lookups, not a scan.
- Recruiters tolerate a batch that fills in over minutes provided they can see it progressing.
  This is an assumption about users that we have not validated.

### Constraints

- The numbers this design must hit are fixed: a two-second acknowledgement, 100 resumes in ten
  minutes on four workers, throughput proportional to worker count, and no accepted resume lost.
  Two load tests with an explanation of the results are required of us.
- One protocol on every boundary, and the Hiring / Resume Processing boundary itself, were fixed
  by [ADR-001](ADR-001-service-decomposition.md). A broker-driven service is expected of us
  eventually; that record defers it deliberately, and this one is written inside that deferral.
- Stated screening behaviour is binding: return a batch ID immediately, process each resume
  independently, update the ranking as results arrive, flag unreadable files without affecting the
  rest, retry a failing scorer and fall back to *needs manual review* rather than a zero score,
  and tell the recruiter when the batch is done.

### Positions

1. **Synchronous REST** — the recruiter waits for the batch.
2. **One handover per batch** — Resume Processing loops over 200 resumes inside one accepted call.
3. **A claimable work table inside Resume Processing** *(chosen)* — `SELECT … FOR UPDATE
   SKIP LOCKED`, retry and terminal states as rows.
4. **An in-memory queue inside Resume Processing** — the handover call pushes onto a channel, a
   worker pool drains it, nothing is written until there is a result.
5. **RabbitMQ** — a task queue with per-message acknowledgement, retry and dead-lettering.
6. **Apache Kafka** — a partitioned, replayable log.
7. **A shared work table in Hiring's database**, polled by Resume Processing.

### Argument

**Why not synchronous (1).** It fails on the first batch: the request outlives any sane gateway
timeout, there are no partial results, there is nowhere to retry from, and the recruiter's browser
becomes the thing holding the state of a five-minute job. It also makes the model's availability
the API's availability.

**Why per-resume rather than per-batch (2).** The decision that matters most here, and the one
easy to get wrong because per-batch looks simpler — one call instead of two hundred. With one
handover per batch, a single corrupt file fails the unit and a retry re-processes all 200 resumes,
re-paying every model call already made. There is no concurrency across replicas within a batch
without building a second, in-process work distributor. Progress cannot be reported. And a batch
stuck behind one pathological file occupies a worker for the duration. Per-resume work makes the
failure unit the same size as the failure: one bad resume damages one result.

**Why a durable table over an in-memory queue (4).** This is the difference between a design that
loses work and one that does not. An in-memory queue makes the handover a lie: the call is
acknowledged, the process restarts, and twenty resumes the recruiter was told were accepted no
longer exist anywhere. There is nothing to reconcile against, because nothing recorded that they
were owed. A row committed before the acknowledgement is what makes *no accepted resume is lost* a
claim we can test by killing a worker mid-batch, rather than a hope.

**Why a table over a broker (5, 6).** Under one protocol on every boundary, a broker is not
available to us for free — it is a second piece of infrastructure and a second delivery model,
adopted at the moment the boundaries themselves are still unproven. `SELECT … FOR UPDATE SKIP
LOCKED` is a well-understood pattern that gives the properties this workload actually needs:
independent units, at-least-once delivery, per-unit retry with backoff, a terminal state for work
that will not succeed, and — the part a broker makes harder — the work item and its result in the
same transaction as the domain data.

The cost is real and we state it plainly: retry, backoff, lease expiry, the reconciliation sweep
and an operator's view of failed work are all code we write and test, and every one of them
arrives free with RabbitMQ. There is also a ceiling. Polling costs a query per worker per
interval, claim contention rises with worker count, and a table is a poor fit for fan-out or for
consumers in other services. None of those binds at 200 resumes and four workers. If any of them
starts to bind, the fix is a broker, and this record is the thing it supersedes.

**Why Kafka specifically was never close (6).** Kafka is an ordered, replayable log; our workload
is unordered, independent tasks needing per-message retry and somewhere to put work that will not
succeed. Consumption is per-partition and ordered, so one resume taking five minutes blocks every
message behind it: head-of-line blocking, exactly the failure this record exists to prevent.

**Why the table lives in Resume Processing, not Hiring (7).** A shared table is two services
writing one schema, which makes them one deployable wearing two names — the coupling the
decomposition exists to prevent. Under the split we have, in-flight screening work is Resume
Processing's own state. Hiring holds the entry and its status; Resume Processing holds the work.
The two are reconciled by handover and callback, not by a join.

**Where the criteria come from.** The criteria snapshot travels in the handover body and is stored
on the work row. Resume Processing never calls Hiring to fetch criteria. A call per resume would
add a synchronous dependency to the path that exists to have none, and criteria can be edited
while a batch runs — the snapshot pins one version to one resume, so every justification is
defensible against the criteria actually used. The snapshot is working state, discarded with the
row once the entry is terminal; the criteria themselves belong to the job opening and stay in
Hiring.

**How this delivers the quality attribute.** Scalability here is one falsifiable claim: *add
workers and throughput rises in proportion.* Per-resume rows are what make it true — no in-process
distributor to saturate, no batch-level lock, no ordering constraint, and `SKIP LOCKED` means the
fourth worker steps over the rows the first three hold rather than waiting behind them. The
ceiling is the provider's rate limit, not our design. The comparative load test runs the same
100-resume burst at one, two and four workers and plots completion time against worker count.

The second load test keeps the first honest: fault injection with the provider deliberately failed
— zero lost rows, retries backing off, work that exhausts its budget ending in the `failed` state
and nowhere else, and the batch completing once the provider returns. Throughput bought by
dropping work is not throughput, and a scalability demonstration that cannot show the second graph
has not shown the first.

### Implications

**What this buys us**

- A batch degrades resume by resume instead of failing whole. Unreadable files and a failing
  scorer stop being special-case error handling and become ordinary states of a row.
- Throughput is a deployment parameter: more replicas, more concurrency, no code change — which
  is what makes the quality attribute demonstrable live rather than only in a report.
- A model outage becomes latency rather than data loss, which is what makes running without a
  fallback survivable at all.
- No new infrastructure. The durability, the transaction and the operator's view come from a
  database the service already runs, and the whole pipeline can be inspected with one SQL query.
- The work item, the profile and the score commit together, so there is no window in which a
  resume is marked done but its result is missing.

**What it costs us**

- **We are hand-building queue semantics.** Backoff, lease expiry, attempt counts, the
  reconciliation sweep and a replay path for failed rows are ours to write, ours to test, and ours
  to get wrong. A broker ships all of it. This is the central cost of the decision and it should
  not be minimised: the most likely serious bug in this system lives in that code.
- **No operational console.** There is no management UI showing depth, consumer count and
  redelivery. We need our own view of pending, claimed, failed and stalled work, or failures
  become invisible — and the fault-injection demonstration needs something to show.
- **At-least-once means occasionally twice.** A worker that dies after calling the model but
  before committing re-scores that resume and we pay again. The upsert makes the data correct; it
  does not make the bill correct. A duplicated callback is harmless for the same reason.
- **Two handover failure modes, not one.** The call from Hiring can fail after the row is written,
  and the callback can fail after the result is stored. Both are retried, and both need the
  receiving side to be idempotent — which is why the upsert key is stated in the decision rather
  than left to implementation.
- **Results are eventually consistent, and the UI must say so.** There is no moment when the
  answer is complete unless the screen tracks it, so the ranked list must show progress and a
  partial ordering, computed at read time from whatever has landed.
- **"Is this batch finished?" is a derived question** that can be answered wrongly. An entry whose
  handover was lost stays `pending` forever; the reconciliation sweep exists solely to bound that,
  and it must be written and tested or the guarantee is decorative.
- **Screening load lands on a database.** Claim queries and result writes share Resume
  Processing's PostgreSQL with its own reads. Contention is bounded by keeping the table narrow
  and indexed on *(state, next\_eligible\_at)*, but under load test the database is measuring
  itself, and that has to be accounted for when the numbers are read.
- **This design has a known end.** Fan-out to a third consumer, cross-service events, or worker
  counts high enough for claim contention all point at a broker. Adopting one later means
  rewriting the handover path in two services, not swapping a library.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — established the Hiring / Resume Processing
  boundary this work crosses, the AI Service the worker calls, and the single-protocol rule that
  ruled the broker out for now.
- **How model access is provided** — the absence of a fallback is why the retry and terminal-state
  policy here must be as explicit as it is.
- **Where results are stored** — the per-resume result written on each successful claim.
- **Still open:** an operator-facing view and replay path for failed work, and the reconciliation
  sweep's schedule and lease bounds. Both are named here as obligations, not designed.

### Related requirements

- **Batch screening** in full: immediate batch ID, concurrent parse and score, streamed results,
  the batch-finished notification, and both failure paths — an unreadable file, and a scorer that
  is unavailable and must retry before falling back to *needs manual review* rather than a silent
  zero. Pipeline monitoring is fed by the callbacks recorded here.
- **FR-2.1** — PDF only, validated before handover. **FR-2.2** — acknowledge without waiting.
  **FR-2.4** — each resume independent, which the per-resume row makes structural.
  **FR-2.6** — ranking updated as results arrive. **FR-2.7** — retry, then manual review, excluded
  from the ranking rather than scored. **FR-2.12** — batch-finished notification from the computed
  terminal state.
- **NFR-02** — the 60-second abandon-and-retry bound defines a transient failure here.
  **NFR-04** — two-second acknowledgement regardless of batch size. **NFR-06, NFR-07, NFR-08** —
  throughput, its proportionality to worker count (**the demonstrated quality attribute**), and
  the burst it must sustain. **NFR-10** — no accepted resume lost, verified by fault injection.
- Required of us: the two load tests and their explanation, the demonstrated quality attribute,
  and the risk matrix — to which this record contributes the hand-built-retry and provider-outage
  risks. The broker-driven service expected of us is not delivered here.

### Related artifacts

The load-test plan and the risk matrix, when written, both derive from this record.

### Related principles

- *Explainability is required, not optional* — an unscored resume is marked *needs manual review*
  with a reason instead of receiving a zero that would silently rank it last.
- *The human still decides* — work the system cannot judge is handed back to a person, not guessed
  at.
- *Easily reversible* — the work table is deliberately shaped like a queue, so replacing it with
  one changes where rows live, not what the states mean.

---

## Notes

This was first drafted on RabbitMQ, with per-message acknowledgement, quorum queues and a
dead-letter exchange. It was rewritten the same day when we decided to run every boundary over
REST and defer the broker: the queue moved inside the service and became a table. What did not
change is the shape of the decision — one unit of work per resume, at-least-once, idempotent
results, classified retries, a terminal state for work that will not succeed. Those are the parts
that were actually argued. The broker was the implementation of them, and it is the part we
expect to reinstate first.

It was also first drafted with *Reliability* as the demonstrated quality attribute, on the
argument that never losing work is the design's most distinctive property. It was re-pointed at
*Scalability* to match the measurement already chosen. The reliability claim survives as the
fault-injection test, which the Argument now treats as the check on the scalability numbers rather
than as a separate claim.

We considered a retry budget generous enough to ride out any plausible outage — hours rather than
minutes — so nothing would ever reach manual review for availability reasons. Rejected because it
hides the outage: a recruiter would see a batch that never finishes and no explanation. A bounded
budget followed by an explicit *needs manual review* tells the truth, and the truth is recoverable
— rows can be replayed once the provider is healthy.
