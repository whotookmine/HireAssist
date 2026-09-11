# ADR-002: One queued message per resume for batch screening

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of batch resume screening, facing per-file processing that takes seconds to
> minutes and depends on an external model that will fail, we decided to make each resume an
> independently queued unit of work on RabbitMQ with bounded retry and a dead-letter path, to
> achieve throughput that rises with worker count and a batch that survives partial failure and a
> model outage without losing work, accepting that results become eventually consistent and that a
> resume may occasionally be processed — and paid for — twice.

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

This is also where the demonstrated quality attribute lives: throughput must rise in proportion as
workers are added, and no accepted resume may ever be lost. Both claims are falsifiable here, and
the second keeps the first honest.

### Decision

**Each resume is one message. The batch is a count, not a unit of work.**

1. **Accept and return.** Hiring validates the upload, rejecting anything that is not a PDF,
   stores the files, creates a batch with one `pending` entry per resume, publishes one
   `screening.requested` message per resume, and responds `202 Accepted` with the batch ID within
   two seconds regardless of batch size.
2. **Consume in parallel.** Resume Processing runs N replicas with a bounded prefetch. One message
   is one resume: parse → `ScoreProfile` over gRPC → persist → publish `screening.result.ready`.
   Concurrency changes by changing N, and nothing else.
3. **Guarantee delivery, not exactly-once.** Durable quorum queues, persistent messages, publisher
   confirms, and manual acknowledgement only after the result is committed. A consumer that
   crashes mid-resume has not acknowledged, so the message is redelivered — at-least-once,
   deliberately.
4. **Make redelivery harmless.** Results are upserted on `(batch_id, resume_id)`, so a redelivered
   message overwrites its own earlier result rather than creating a second one.
5. **Classify failures before retrying.** A *permanent* failure — a corrupt file, a scanned image
   with no text layer — is never retried; the entry goes to `needs_manual_review` with the reason.
   A *transient* failure — 429, 5xx, a broken connection, or a scoring attempt past its 60-second
   bound — is retried a bounded number of times with exponential backoff and jitter, using a delay
   queue and a per-message attempt count.
6. **Dead-letter what does not recover.** Once the retry budget is spent the message goes to
   `screening.dlq` and the entry is marked `needs_manual_review` with the last error. Dead-lettered
   work is visible and replayable; it is never silently scored zero and never silently dropped.
7. **Completion is computed.** A batch is complete when every entry is terminal — `scored` or
   `needs_manual_review` — and reaching that state triggers the batch-finished notification. There
   is no all-or-nothing outcome. A periodic reconciliation sweep re-publishes entries left
   `pending` past a threshold, the only defence against a message that vanishes without trace.
8. **Stream results to the screen.** `screening.result.ready` events drive the shortlist view over
   server-sent events, so the ranking fills in as results land.

**The broker is RabbitMQ**, with quorum queues for durability, a dead-letter exchange for
exhausted work, and a delay queue for backoff.

### Status

**Accepted**

### Group

Communication · Scalability · Reliability

---

## Details

### Assumptions

- A batch is 20–200 resumes, each file at most 10 MB and usually far smaller. Nothing here
  assumes tens of thousands of files.
- The model provider will be unavailable or rate-limiting some of the time, with no fallback to
  switch to. Waiting is the only recovery.
- Scoring one resume is independent of scoring any other, so ordering does not matter and
  parallelism is free.
- Re-running a resume through the model costs money but is otherwise harmless — the output is
  regenerable, not authoritative.
- Recruiters tolerate a batch that fills in over minutes provided they can see it progressing.
  This is an assumption about users that we have not validated.

### Constraints

- At least one service must communicate through a message broker, and two load tests with an
  explanation of the results are required of us.
- The numbers this design must hit are fixed: a two-second acknowledgement, 100 resumes in ten
  minutes on four workers, throughput proportional to worker count, and no accepted resume lost.
- The service boundary this queue crosses was fixed by
  [ADR-001](ADR-001-service-decomposition.md).
- Stated screening behaviour is binding: return a batch ID immediately, process each resume
  independently, update the ranking as results arrive, flag unreadable files without affecting the
  rest, retry a failing scorer and fall back to *needs manual review* rather than a zero score,
  and tell the recruiter when the batch is done.

### Positions

1. **Synchronous REST** — the recruiter waits for the batch.
2. **One message per batch** — the consumer loops over 200 resumes inside a single message.
3. **RabbitMQ** *(chosen)* — a task queue with per-message acknowledgement, retry and
   dead-lettering.
4. **Apache Kafka** — a partitioned, replayable log.
5. **NATS JetStream** — light, Go-native, fast to stand up.
6. **A PostgreSQL job table polled by workers** (`SELECT … FOR UPDATE SKIP LOCKED`), no broker.

### Argument

**Why not synchronous (1).** It fails on the first batch: the request outlives any sane gateway
timeout, there are no partial results, there is nowhere to retry from, and the recruiter's browser
becomes the thing holding the state of a five-minute job. It also makes the model's availability
the API's availability.

**Why per-resume rather than per-batch (2).** The decision that matters most here, and the one
easy to get wrong because per-batch looks simpler. With one message per batch, a single corrupt
file fails the message and a retry re-processes all 200 resumes, re-paying every model call
already made. There is no concurrency inside a batch without building a second, in-process work
distributor. Progress cannot be reported. And a batch stuck behind one pathological file blocks
the consumer holding it. Per-resume messages make the failure unit the same size as the failure:
one bad resume damages one result.

**Why RabbitMQ over Kafka (4).** Kafka is an ordered, replayable log; our workload is unordered,
independent tasks needing per-message retry and somewhere to put work that will not succeed.
Kafka gives neither natively — retry and DLQ are assembled from extra topics and consumer logic.
Worse, consumption is per-partition and ordered, so one resume taking five minutes blocks every
message behind it: head-of-line blocking, exactly the failure this record exists to prevent.
RabbitMQ's model — a message, an ack, a bounded prefetch, a dead-letter exchange — is the shape of
the problem, and costs less to run when six services and two databases already share a cluster.

**Why RabbitMQ over NATS JetStream (5).** JetStream would work. Two things decided it: nobody has
used either, so familiarity was not a tiebreaker, and RabbitMQ's management UI makes queue depth,
consumer count, redelivery and the dead-letter queue visible in a browser. We have to
*demonstrate* reliability, not merely have it, and showing the DLQ filling during an injected
outage and draining afterwards is worth more than a metric in a log file.

**Why a broker at all, over a database job table (6).** Honestly, `SKIP LOCKED` is a good pattern
and one fewer moving part would be defensible. It loses on two counts. A message broker is
required of us; and independently, a polling table puts batch contention on the same database that
serves recruiter-facing reads — the coupling [ADR-001](ADR-001-service-decomposition.md) drew a
boundary to avoid. Under load test the database would be measuring itself, and the two-second
ranked list would compete with the batch for the same locks.

**How this delivers the quality attribute.** Scalability here is one falsifiable claim: *add
workers and throughput rises in proportion.* The per-resume message is what makes it true — no
in-process distributor to saturate, no batch-level lock, no ordering constraint, so the fourth
consumer is as useful as the first until the provider's rate limit, rather than our design,
becomes the ceiling. The comparative load test runs the same 100-resume burst at one, two and four
workers and plots completion time against worker count.

The second load test keeps the first honest: fault injection with the provider deliberately failed
— zero lost messages, retries backing off, the dead-letter queue receiving exactly the work that
exhausted its budget, and the batch completing once the provider returns. Throughput bought by
dropping work is not throughput, and a scalability demonstration that cannot show the second graph
has not shown the first.

### Implications

**What this buys us**

- A batch degrades resume by resume instead of failing whole. Unreadable files and a failing
  scorer stop being special-case error handling and become ordinary states of an entry.
- Throughput is a deployment parameter: more consumers, more concurrency, no code change — which
  is what makes the quality attribute demonstrable live rather than only in a report.
- A model outage becomes latency rather than data loss, which is what makes running without a
  fallback survivable at all.
- The dead-letter queue turns silent failure into visible failure — the most valuable property in
  a system whose output a recruiter is trusting.

**What it costs us**

- **At-least-once means occasionally twice.** A consumer that dies after calling the model but
  before acknowledging re-scores that resume and we pay again. The upsert makes the data correct;
  it does not make the bill correct.
- **Results are eventually consistent, and the UI must say so.** There is no moment when the
  answer is complete unless the screen tracks it, so the ranked list must show progress and a
  partial ordering, computed at read time from whatever has landed.
- **"Is this batch finished?" is a derived question** that can be answered wrongly. A message lost
  between publish and consume leaves an entry `pending` forever; the reconciliation sweep exists
  solely to bound that, and it must be written and tested or the guarantee is decorative.
- **The broker is a single point of failure for screening.** RabbitMQ down means no batch
  progresses. We accept a single node and record it as a known risk rather than pretending a
  cluster is in scope.
- **The dead-letter queue needs an owner and a replay tool.** Without one it becomes a place
  failures go to be forgotten — worse than not having it, because it converts a visible failure
  into a filed one.
- **Publisher confirms plus per-resume messages make the accept path chattier:** 200 confirmed
  publishes inside a request that must return quickly. Publishing must be batched or moved behind
  the response, or the immediate-`202` claim quietly stops being true at the top of the range.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — established the Hiring / Resume Processing
  boundary this queue crosses and the AI Service the consumer calls.
- **How model access is provided** — the absence of a fallback is why the retry and dead-letter
  policy here must be as explicit as it is.
- **Where results are stored** — the per-resume result written on each successful consume.

### Related requirements

- **Batch screening** in full: immediate batch ID, concurrent parse and score, streamed results,
  the batch-finished notification, and both failure paths — an unreadable file, and a scorer that
  is unavailable and must retry before falling back to *needs manual review* rather than a silent
  zero. Pipeline monitoring consumes the domain events published here.
- **FR-2.1** — PDF only, validated before the queue. **FR-2.2** — acknowledge without waiting.
  **FR-2.4** — each resume independent, which the per-resume message makes structural.
  **FR-2.6** — ranking updated as results arrive. **FR-2.7** — retry, then manual review, excluded
  from the ranking rather than scored. **FR-2.12** — batch-finished notification from the computed
  terminal state.
- **NFR-02** — the 60-second abandon-and-retry bound defines a transient failure here.
  **NFR-04** — two-second acknowledgement regardless of batch size. **NFR-06, NFR-07, NFR-08** —
  throughput, its proportionality to worker count (**the demonstrated quality attribute**), and
  the burst it must sustain. **NFR-10** — no accepted resume lost, verified by fault injection.
- Required of us: the message-broker service, the two load tests and their explanation, the
  demonstrated quality attribute, and the risk matrix — to which this record contributes the
  single-broker and provider-outage risks.

### Related artifacts

The load-test plan and the risk matrix, when written, both derive from this record.

### Related principles

- *Explainability is required, not optional* — an unscored resume is marked *needs manual review*
  with a reason instead of receiving a zero that would silently rank it last.
- *The human still decides* — work the system cannot judge is handed back to a person, not guessed
  at.

---

## Notes

This was first drafted with *Reliability* as the demonstrated quality attribute, on the argument
that never losing work is the design's most distinctive property. It was re-pointed at
*Scalability* the same day, to match the measurement already chosen. Nothing in the Decision
changed — the per-resume queue delivers both — and the reliability claim survives as the
fault-injection test, which the Argument now treats as the check on the scalability numbers rather
than as a separate claim.

We considered a retry budget generous enough to ride out any plausible outage — hours rather than
minutes — so nothing would ever reach manual review for availability reasons. Rejected because it
hides the outage: a recruiter would see a batch that never finishes and no explanation. A bounded
budget followed by an explicit *needs manual review* tells the truth, and the truth is recoverable
— entries can be replayed once the provider is healthy.

Redis Streams was raised and dismissed quickly: we do not run Redis, and adopting a datastore in
order to use its queueing as a side effect is the wrong order of reasoning.
