# ADR-017: The retention and staleness sweeps run on a timer inside Compliance & Insights

**Date:** 2026-11-13
**Deciders:** Patiphon Puntusin

> In the context of the two jobs that must run on a schedule whether or not anyone is using the
> system, facing a choice between a scheduler of its own and a timer inside the service that does
> the work, we decided to **run both sweeps on a timer inside the Compliance & Insights Service,
> guarded by a database lock so only one replica sweeps at a time**, to avoid another moving
> part in every deployment, accepting that a sweep only runs while that service is up.

---

## Summary

### Issue

Two use cases need work done on a schedule:
- **UC-4** evaluates every open job opening against its staleness rule "on a recurring schedule,
  independently of whether any user views the dashboard" (FR-4.2).
- **UC-5** evaluates every candidate against the retention policy on a schedule (FR-5.2). It
  warns before expiry, erases on expiry, and retries erasures that are still pending (FR-5.8).

The design drew a "System Scheduler" as the caller of both, and left open where the timer runs,
what happens with more than one replica, and how a sweep that dies half-way resumes. Both sweeps
belong to the Compliance & Insights Service, which owns all the data they read and write.

### Decision

**The Compliance & Insights Service runs both sweeps itself, on a timer:**
- once shortly after it starts;
- then every `SWEEP_INTERVAL`, one hour by default;
- staleness first, then retention.

- **One replica sweeps at a time.** Each sweep takes a PostgreSQL transaction-level advisory
  lock. A replica that cannot take it skips that sweep. The lock is released however the sweep
  ends, including when the process dies.
- **A sweep that dies half-way needs no resume step.** Each sweep re-reads the current state
  and is idempotent:
  - a flagged opening is not flagged again;
  - a warning is sent once per expiry date;
  - a pending erasure is retried, not restarted.

  The next sweep finishes what the last one did not.
- **The same sweeps can be run on demand** through two Admin endpoints, under the same lock, for
  demonstrations and for an operator who will not wait an hour.

The System Scheduler is therefore a **component** of the Compliance & Insights Service, not a
container of its own.

### Status

**Accepted**

### Group

Deployment

---

## Details

### Assumptions

- Hourly is often enough. Retention periods are counted in days, and the staleness rule in days
  of time-to-fill.
- A deployment runs one replica of Compliance (ADR-006 sizes everything for one small company),
  so the lock is a guard, not a work-sharing mechanism.
- The number of candidates per deployment is in the thousands, so one sweep finishes in seconds.

### Constraints

- One deployment per customer (ADR-006): whatever runs the schedule is multiplied by the number
  of customers.
- FR-4.2 and FR-5.2 require the sweeps to run without anyone using the system.

### Positions

1. **A timer inside Compliance & Insights.** *(Selected.)*
2. **A Kubernetes CronJob that calls the sweep endpoints.** It uses the platform's own scheduler
   and keeps a history of each run. The schedule then lives in deployment configuration, apart
   from the code that depends on it, and the local compose stack needs a separate imitation.
3. **A scheduler service of its own** that triggers sweeps in other services. It only pays off
   when more than one service has scheduled work, and today only Compliance does.
4. **A delayed message on the broker** that re-publishes itself. It reuses RabbitMQ, but a lost
   message stops the schedule silently.

### Argument

**Against a separate scheduler service (3):** it is one more process to build, deploy, upgrade
and monitor in every customer deployment, and its only job would be to call one other service.

**Against a self-scheduling message (4):** a schedule that disappears when one message is lost is
the failure FR-4.2 exists to prevent, and nothing would notice it had stopped.

**Against a CronJob (2):** it is the strongest alternative, and the one to move to if sweeps ever
need run history or must run while the service is scaled to zero. Today it would split one
feature across the code and the deployment configuration, and it does not exist in the compose
stack the demo runs on.

**For the in-process timer (1):**
- The schedule ships with the code that depends on it.
- It runs the same way locally and in a deployment.
- The advisory lock answers the replica question, and idempotent sweeps answer the "died
  half-way" question.
- The on-demand endpoints cover what a CronJob's manual trigger would.

### Implications

**What this buys us**

- No new process in any deployment, and the compose stack runs the real schedule.
- Two replicas cannot sweep at once, and a crash mid-sweep loses nothing.
- Demonstrations do not wait an hour: an Admin can run either sweep on demand.

**What it costs us**

- **A sweep runs only while Compliance is up.** A long outage delays warnings, erasures and
  flags by its length. The first sweep after restart catches up, because every rule compares
  dates rather than counting ticks.
- **No run history beyond the log.** Each sweep logs what it did, but there is no table of
  past runs. An operator who must prove that a sweep ran reads the log or the erasure records.
- **The lock holds a database connection for the length of a sweep.** It costs nothing at this
  size, but it is the first thing to revisit if sweeps grow long.
- **A notice that fails to send is not retried.** The stale flag and the warning mark are
  recorded either way, so a mail outage loses that notice rather than stopping the sweep. The
  dashboard still shows the flag.

---

## Related

### Related decisions

- [ADR-006](ADR-006-single-tenant-deployment.md): one deployment per customer, so a separate
  scheduler would be one more thing per customer.
- [ADR-016](ADR-016-orchestrated-erasure.md): pending erasures are retried by this sweep.
- [ADR-015](ADR-015-pipeline-events-on-the-broker.md): the staleness sweep reads the projection
  that the pipeline events maintain.

### Related requirements

- **FR-4.2**: staleness evaluated on a recurring schedule. **FR-4.3, FR-4.4**: flag and notify.
- **FR-5.2**: retention evaluated on a recurring schedule, with warnings before expiry.
  **FR-5.3**: erasure on expiry. **FR-5.8**: retry of failed erasures.
- **NFR-13**: erasure within 30 days, which hourly retries comfortably meet.

### Related artifacts

The container diagram: the System Scheduler is no longer a container.
