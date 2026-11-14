# Risk Matrix

The architecture's main risks, assessed with **FMEA** (Failure Modes and Effects Analysis), the
method the course teaches for risk analysis in software architecture. Each risk is a way the
system can fail (*failure mode*), what a user or the company would suffer (*effect*), and why it
would happen (*cause*). It is then scored from 1 to 10 three times:

| Score | Meaning | 1 | 10 |
|---|---|---|---|
| **S** — Severity | How bad the effect is | negligible | critical |
| **O** — Occurrence | How likely the cause is | rare | frequent |
| **D** — Detection | How likely the failure escapes notice until it hurts | caught at once | undetected |

**RPN** (Risk Priority Number) = S × O × D, from 1 to 1,000, ranks the risks. Each risk is traced
to the decision that created or accepted it. Three Occurrence scores come from measurements rather
than judgement — R1 from the Scalability test, R6 and R7 from the stress test, all described in
[LOAD-TEST.md](LOAD-TEST.md), where a fault-injection run also tests R4's mitigation.

## The matrix

| ID | Failure mode | Effect | Cause | S | O | D | RPN |
|---|---|---|---|---|---|---|---|
| **R1** | Adding Resume Processing replicas stops raising screening throughput | A large batch takes longer than NFR-06 allows; the extra replicas are paid for and idle | The AI Service runs one replica with a fixed pool of 10 worker threads, so at most 10 model calls run at once however many workers wait (ADR-007, ADR-014) | 6 | 8 | 5 | **240** |
| **R2** | The Kubernetes manifests fail on their first real deployment | Discovery and health checking, designed in ADR-014, do not work when the system goes live; release slips | The system has only ever run under Docker Compose; eleven workloads have manifests, but the Compliance & Insights Service, its database and Mailpit, added since, have none; the first kind cluster ran out of disk | 6 | 6 | 6 | **216** |
| **R3** | Candidate data outlives its consent or an erasure request | A PDPA breach: legal penalty and lost customer trust | Copies of a candidate live in five places — Hiring's PostgreSQL and MongoDB, Resume Processing's MongoDB, MinIO, and the model provider | 9 | 3 | 7 | **189** |
| **R4** | The model provider is down or throttles us mid-batch | No scores while it lasts; past the retry budget, resumes land in *needs manual review* | One hosted provider, with no automatic fallback (ADR-004) | 7 | 4 | 3 | **84** |
| **R5** | A screening message is lost, or processed twice | Lost: a candidate is never screened and the batch never completes. Twice: the model is paid twice | RabbitMQ sits on the screening path and delivers at least once (ADR-013) | 7 | 3 | 4 | **84** |
| **R6** | Batch uploads slow to many seconds at peak load | A recruiter waits up to 15 s for an upload to be accepted, breaching NFR-04; one who retries submits the batch twice | Every upload publishes the outbox inline, under one lock in the Hiring Service, one resume at a time | 5 | 3 | 4 | **60** |
| **R7** | The API Gateway fails or saturates | Every screen of the web app stops at once | One gateway replica; its rate limiter keeps its counters in memory, so a second replica would double every member's allowance | 8 | 2 | 2 | **32** |

## Why each score, and what reduces it

**R1 — the AI Service caps throughput (RPN 240).** *Severity 6*: batches slow down, nothing is
lost. *Occurrence 8*, measured: with 4 replicas each running 4 resumes at once, 16 calls wait on
10 threads, and the Scalability test reached only 59% of linear throughput (47.1 resumes a minute
where about 79 were expected). Any deployment that scales past two replicas with the default
setting meets it. *Detection 5*: nothing alerts; a recruiter only sees a slower batch.
**Mitigation:** size the AI Service's pool to the screening concurrency (`AI_SERVICE_MAX_WORKERS`
≥ replicas × prefetch) — measured to restore it to 89% of linear (70.6 a minute) — and alert on
queue depth. Running the AI Service as several replicas would need gRPC balancing in the client or
a proxy, because Kubernetes balances per connection (ADR-014).

**R2 — the cluster has never run (RPN 216).** *Severity 6*: the design does not fail, its
deployment does, and the Compose stack keeps working. *Occurrence 6*: eleven workloads' manifests
have never been applied together, three newer workloads have none, and a first run usually finds
something. *Detection 6*: nothing
checks them until someone deploys. **Mitigation:** write the missing manifests, then run
`deploy/k8s/up.sh` on a one-node kind cluster before any release, with enough free disk; both platforms already run the same images
with the same variables, which narrows what can differ.

**R3 — personal data outlives its purpose (RPN 189).** *Severity 9*: PDPA is the design force
the proposal puts first. *Occurrence 3*: erasure and retention are implemented, not planned.
*Detection 7*: a copy left behind is invisible unless someone audits for it. **Mitigation:** an
erasure stays pending until every holder confirms it (ADR-016); retention sweeps run on a
schedule (ADR-017); every access is in the audit log; the provider must not train on or keep the
data (ADR-004), and its current terms are still to be confirmed.

**R4 — the model provider fails (RPN 84).** *Severity 7*: screening, the product's core, stops.
*Occurrence 4*: hosted model APIs have outages and rate limits outside our control. *Detection 3*:
the failure is immediate and loud — gRPC errors, retries in the log, entries marked failed.
**Measured:** with the AI Service stopped for 20 seconds mid-batch, the default three retries
(about 35 seconds of backoff) left 8 of 40 resumes failed — visibly, in the dead-letter queue —
while five retries screened all 40. **Mitigation:** each failed call is retried through the
broker's retry queue; work that still fails goes to a dead-letter queue and Hiring is told, so a
resume is never silently stuck; raise `MAX_RETRIES` to 5 so the budget outlasts a short outage;
the provider can be switched between Anthropic and OpenAI by configuration.

**R5 — a message is lost or doubled (RPN 84).** *Severity 7*: a candidate silently skipped is
unfair to them. *Occurrence 3*: queues and messages are durable. *Detection 4*: a lost resume
leaves its batch incomplete, which the progress view shows. **Mitigation:** a message is
acknowledged only after its result is stored and the broker has confirmed the outgoing result;
results are keyed on the batch and resume, so a redelivery replays rather than duplicates.

**R6 — uploads queue behind one lock (RPN 60).** Found by the stress test. *Severity 5*: nothing
is lost, but an upload that hangs for seconds invites a second click, and a second batch is
screened — and paid for — twice. *Occurrence 3*: it took 100 to 200 recruiters at once, five to
ten times the design load; at 100 the upload p95 was 96 ms, at 200 it was 14.9 s. *Detection 4*:
it is visible only as slow uploads; nothing alerts. The cause is in the Hiring Service: after
saving a batch, each upload request publishes every pending resume itself, while holding a lock
that every other upload waits on, with a broker publish and a database update per resume.
**Mitigation:** answer 202 once the batch and its outbox rows are committed and leave publishing to
the background relay, or skip the inline publish when another request already holds the lock;
publish in bulk and mark the rows in one update.

**R7 — the gateway fails (RPN 32).** *Severity 8*: everything goes through it. *Occurrence 2*:
the stress test did not make it fail — 53,242 requests up to 200 recruiters, none failed; the
system saturated near 210 requests a second and slowed down instead. *Detection 2*: its health
check and every user notice at once. **Mitigation:** a second replica behind its Service,
once the rate-limit counters move to a shared store — until then a second replica would double
each member's limit.
