# Load Test Results

Three load tests of the whole system, run on 2026-11-13 with Grafana k6 through the API Gateway,
the way the web app calls it. Two answer whether the API meets its targets at and beyond the
design load; the third measures **Scalability**, the quality attribute this project demonstrates.
The scripts and how to run them are in `loadtest/`.

## Method

| Test | k6 test type | Load | What it measures | Targets |
|---|---|---|---|---|
| Average load | average-load | 20 recruiters for 2 minutes, after a 30-second ramp | Browsing openings, ranked candidates and justifications; one session in ten also submits a 3-resume batch | p95 < 2 s for ranked results (NFR-03) and for a batch acknowledgement (NFR-04); under 1% failed requests |
| Stress | stress | 20, then 50, 100 and 200 recruiters, a minute at each | The same session, past the design load of 20 (NFR-08) | The same, read as where the system gives way rather than pass or fail |
| Scalability | comparative | One burst of 100 resumes, at 1, 2 and 4 Resume Processing replicas | Time from upload until the last resume is screened, and resumes per minute | 100 resumes within 10 minutes with 4 workers (NFR-06); throughput in proportion to workers (NFR-07) |

Every virtual user signs in as its own recruiter, created through the admin as UC-6 would: the
gateway limits each member to 20 requests a second, so a shared token would measure the rate
limiter instead of the system.

**Environment.** One MacBook Air (Apple M2, 8 cores, 16 GB), running the Compose stack — fourteen
containers in Docker Desktop with 8 CPUs and 11.7 GB — and k6 v2.1.0 on the same machine, so the
load generator competes with the system for the same CPU.

**The model is simulated.** The AI Service runs its canned provider with a 6-second delay on
every call. A resume takes two calls — extracting the profile, then scoring it — so twelve
seconds, the figure the NFR estimates assume. A real provider would put its rate limit and its
bill into the numbers, and those are not the architecture under test. The consequence is that the
model's own variance is absent: a real model is sometimes faster and sometimes much slower.

## Results

### Average load — 20 recruiters (NFR-03, NFR-04, NFR-08)

Three minutes in all — a 30-second ramp, two minutes at 20 recruiters, a 30-second ramp down — on
four Resume Processing replicas. Setup's sign-ins are excluded; they hash passwords and are not
part of a recruiter's session.

| Request | Count | Median | p95 | p99 | Max | Failed |
|---|---|---|---|---|---|---|
| List openings | 1,492 | 1.8 ms | 4.9 ms | 6.7 ms | 53.5 ms | 0 |
| Get an opening | 1,492 | 1.1 ms | 2.5 ms | 4.0 ms | 18.3 ms | 0 |
| **Ranked results** (NFR-03, target p95 < 2 s) | 1,492 | 5.6 ms | **11.8 ms** | 13.7 ms | 20.1 ms | 0 |
| Candidate justification | 1,492 | 5.4 ms | 10.5 ms | 14.5 ms | 28.7 ms | 0 |
| **Submit a 3-resume batch** (NFR-04, target p95 < 2 s) | 182 | 14.8 ms | **22.3 ms** | 28.9 ms | 34.0 ms | 0 |
| Batch status | 182 | 1.2 ms | 2.2 ms | 3.3 ms | 3.7 ms | 0 |
| **All requests** | 6,332 | 3.2 ms | 11.5 ms | 16.6 ms | 53.5 ms | **0%** |

Every threshold passed, every check passed, and no request failed. At the design load the API is
two orders of magnitude inside its targets: the slowest request in three minutes took 54 ms. This
is what the asynchronous design predicts — a recruiter's requests read and write small rows, and
the slow part, the model, is behind the queue where it cannot hold a request open. The test does
not show that the ranked list stays fast with 1,000 candidates (NFR-03): the opening's list grew
from 30 screened candidates to 244 as the test's own batches were screened.

### Stress — 20, 50, 100 and 200 recruiters

The same session, a minute at each level after a 30-second ramp, on the same stack. The figures
are for the minute at each level.

| Recruiters | Requests a second | Ranked results p95 | Submit batch p95 | All requests p95 | Failed |
|---|---|---|---|---|---|
| 20 | 42 | 8.2 ms | 21.1 ms | 9.4 ms | 0 |
| 50 | 105 | 12.4 ms | 24.7 ms | 10.7 ms | 0 |
| 100 | 205 | 73.7 ms | 95.9 ms | 34.8 ms | 0 |
| 200 | 211 | 1,132 ms | **14,936 ms** | 978 ms | 0 |

**No request failed** in 53,242. Throughput rose with the recruiters up to 100, then stopped: at
200 the system served about the same 210 requests a second, and the extra recruiters waited
instead. That is saturation, not failure. The ceiling belongs to the whole stack on one laptop, with
k6 on the same eight cores; this test cannot say which service would set it on separate machines.

**The first thing to break was uploading, not reading.** The ranked list stayed inside NFR-03's
2 seconds at 200 recruiters (p95 1.1 s), but a batch upload's p95 reached 14.9 seconds, seven times
NFR-04's target. The cause is in the Hiring Service, not the infrastructure: after storing a batch,
each upload request publishes every pending resume to the broker itself — one at a time, with a
database update for each — under a lock every upload shares. At 100 recruiters the lock is barely
contended and an upload takes 96 ms; at 200 the requests queue behind it. This is risk R6 in
[RISK-MATRIX.md](RISK-MATRIX.md), with its fix.

In short, the system met every target at five times the design load of 20 recruiters, and the
first target gave way at ten times.

### Scalability — 100 resumes at 1, 2 and 4 replicas (NFR-06, NFR-07)

| Resume Processing replicas | AI Service threads | 100 resumes in | Throughput | × one replica | Of linear | Upload acknowledged in |
|---|---|---|---|---|---|---|
| 1 | 10 | 302 s (5.0 min) | 19.8 a minute | 1.00 | 100% | 265 ms |
| 2 | 10 | 158 s (2.6 min) | 37.9 a minute | 1.91 | 96% | 372 ms |
| 4 | 10 | 127 s (2.1 min) | 47.1 a minute | 2.38 | 59% | 425 ms |
| 4 | 32 | 85 s (1.4 min) | 70.6 a minute | 3.57 | 89% | 359 ms |

All 400 resumes were screened; none failed.

**What the numbers mean.** A replica holds four resumes at a time, and a resume takes twelve
seconds of model time, so one replica should screen about 20 a minute — it screened 19.8. A second
replica doubled that almost exactly (96% of linear): the queue hands work to whichever replica is
free, with no coordination between them. **At four replicas throughput stopped following the
replica count**: 16 resumes wanted a model call at once, but the AI Service answers on a pool of
10 threads, which caps screening at 10 ÷ 12 s ≈ 50 a minute — it measured 47.1. Raising the pool to
32 (`AI_SERVICE_MAX_WORKERS`) brought four replicas to 70.6 a minute, 89% of linear. The remaining
11% is the work that is not the model — reading the PDF, MongoDB writes, the broker round trips,
Hiring recording each result — on one laptop that also runs k6.

So **Scalability holds, with one condition**: the AI Service's pool must be at least the screening
concurrency, replicas × 4. With the default of 10, NFR-07 fails beyond two replicas. NFR-06 — 100
resumes within 10 minutes with four workers — is met by a wide margin either way (1.4 and 2.1
minutes), under the twelve-second assumption. NFR-04 held for the largest uploads: 100 files were
acknowledged in at most 425 ms, against a 2-second target. The finding is risk R1 in
[RISK-MATRIX.md](RISK-MATRIX.md). The next ceiling, not measured here, is a real provider's rate
limit.

### Fault injection — the AI Service stopped mid-batch (NFR-10)

A batch of 40 resumes at four replicas and 32 threads, with the AI Service stopped ten seconds in
and started again twenty seconds later. It was answering again 23 seconds after the stop.

| Retries per resume | Retry budget (5, 10, 20 … s) | Screened | Failed | Retries | Batch finished in |
|---|---|---|---|---|---|
| 3 (the service's default) | about 35 s | 32 | 8 | 120 | 76 s |
| 5 | about 2.5 min | 40 | 0 | 98 | 87 s |

No resume was lost silently in either run. With three retries, 8 resumes used up their budget:
each went to the dead-letter queue and Hiring was told it failed, so the batch completed with those
8 visibly marked and replayable. That happened although the service was back after 23 seconds,
which is consistent with the gRPC client backing off its reconnection after repeated failures — the
caller sees a longer outage than the server had. With five retries, the outage only delayed the
batch. Raising `MAX_RETRIES` to 5 is the mitigation for risk R4 in [RISK-MATRIX.md](RISK-MATRIX.md).
