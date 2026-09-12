# ADR-003: PostgreSQL as system of record, MongoDB for AI-derived documents

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of storing hiring records alongside data the model derives from resumes, facing
> two kinds of data with opposite needs — one that must never be wrong, one whose shape changes
> every time we improve a prompt — we decided to keep the system of record in PostgreSQL and the
> derived documents in MongoDB, to achieve enforced integrity where law and auditing demand it and
> schema freedom where the model's output evolves, accepting that one candidate's data now spans
> two stores and every erasure must succeed in both.

---

## Summary

### Issue

The data splits cleanly in two, and the split is not about volume — it is about what happens when
the data is wrong.

**One half must never be wrong.** Memberships and roles decide who may see whose candidates. A job
opening's weighted criteria are what a score is defensible against; an override records that a
human disagreed with the model. The retention policy determines what the company may lawfully
keep, and the audit log is the evidence it did what it claims. This data is relational and stable,
and needs constraints, transactions and deliberate migrations. Losing referential integrity here
is not a bug, it is a compliance incident.

**The other half is derived, and its shape is unsettled.** A profile parsed from a resume is
deeply nested and extremely sparse — three jobs or eleven, a publications list or none, skills
grouped one way this month and another next. Scoring output carries per-criterion sub-scores, a
must-have check and a justification, and its structure changes every time we improve the prompt.
Modelling that relationally means a table per nested collection, a migration per prompt change,
and a wide row of nullable columns for the fields most resumes lack.

**Privacy law cuts across both.** A person must be erased from *everywhere* on a schedule, and
there are two expiry actions — delete outright, or anonymise while preserving the aggregate counts
the dashboard is built from. Anonymisation is only clean if the identifying material and the
statistics are separable in the first place.

### Decision

**PostgreSQL is the system of record.** It holds everything whose correctness is load-bearing:

- users, sessions, memberships and roles
- job openings and their weighted criteria
- screening batches and the per-resume status of each entry
- shortlist decisions, the AI score, and any human override
- candidate identity, the date the data was collected, and the retention anchor date
- the retention policy; the append-only erasure audit log, which names a candidate only by a
  pseudonymous identifier and holds no personal data; and the access audit log
- the pipeline counters the dashboard reports from

**MongoDB holds what the model derives.** Documents, read whole and replaced whole:

- the normalised candidate profile parsed from a resume
- extracted resume text
- per-criterion scoring output with its evidence and the written justification — persisted with
  the score rather than regenerated on demand
- generated interview guides

**Resume files live in object storage** (S3-compatible; MinIO locally), not in either database.
PostgreSQL holds the object key.

**The rule for future data,** recorded so the boundary does not rot:

> If getting it wrong is a correctness or compliance problem, it goes in PostgreSQL. If the model
> produced it and its shape changes when the model or the prompt changes, it goes in MongoDB.
> Nothing in PostgreSQL may depend on a MongoDB document for its correctness.

**Linking and erasure.** Every MongoDB document carries `workspace_id` and `candidate_id` and is
indexed on both, so retention deletes a person's derived data directly rather than by traversing
relationships. PostgreSQL is authoritative for the existence of a candidate; MongoDB is
authoritative for nothing.

**Ownership follows the service boundary** set by
[ADR-001](ADR-001-service-decomposition.md): each service owns its own schema and no service reads
another's tables. Two services needing the same fact exchange it in a call, not a join.

### Status

**Accepted**

- *Amended 2026-09-11 by ADR-006: the workspace concept is removed. MongoDB documents carry
  `candidate_id` alone, because one deployment holds one company's data, and the retention policy
  is per deployment.*

### Group

Data

---

## Details

### Assumptions

- Volume is modest — thousands of candidates per deployment, not millions. Neither store is chosen
  for scale; both are chosen for fit.
- Derived documents are read whole and written whole, so document semantics lose us nothing.
- MongoDB documents are regenerable from the stored resume file, right up until retention deletes
  that file. After erasure nothing is regenerable, which is the intended behaviour.
- The team can run both stores locally and in the cluster — a real load on a development machine
  alongside six services.
- Both are open source and free to run, so licensing does not enter the decision.

### Constraints

- At least two database types, one relational and one NoSQL, are required of us.
- Erasure must be complete and provable, so every store holding personal data must be reachable by
  the retention process and must appear in the audit trail.
- Per-service data ownership was fixed by [ADR-001](ADR-001-service-decomposition.md); this
  decision operates inside it.

### Positions

1. **PostgreSQL only, with `jsonb` columns** for profiles and scoring output.
2. **MongoDB only** — one store, no migrations, everything a document.
3. **PostgreSQL + Redis** as the second database type.
4. **PostgreSQL + Elasticsearch** — profiles indexed for full-text search.
5. **PostgreSQL + MongoDB** *(chosen)*.
6. **One shared database for all services**, instead of per-service ownership.

### Argument

**PostgreSQL-only with `jsonb` (1) deserved the most argument,** because it is genuinely strong:
schema freedom without a second store, real transactions across the record and the derived data,
one backup, one set of credentials, one thing to learn.

It was rejected primarily because of the *anonymise* action. When retention expires and the policy
says anonymise rather than delete, the company keeps the statistics and destroys the person. If
identifying material lives in `jsonb` on the same rows as the counts, anonymisation becomes a
surgical rewrite of every row — one missed column away from a compliance failure. With derived
documents in a separate store it is *drop this candidate's documents, keep the rows*: coarse,
verifiable, easy to prove.

Second, blast radius. Scoring output changes shape most often and is rewritten most carelessly;
keeping it out of the database that holds the retention policy and the audit log means an
experiment with the parser cannot lock or bloat the table that proves we complied with the law.

Third, the two-database requirement — listed third deliberately. Without it, position 1 would have
been defensible and we would have had a harder argument.

**MongoDB-only (2)** fails in the opposite direction. Memberships, retention policy and the audit
log are exactly the data that must not drift, and we would be hand-rolling referential integrity
in the one area where being wrong is expensive. Multi-document transactions exist, but choosing a
store whose defaults work against the guarantee you most need is a poor trade.

**PostgreSQL + Redis (3):** Redis is a cache, not a store. Using it as the second database type
means either treating a cache as durable — the classic way to lose data quietly — or nominating a
"second database" holding nothing anyone would miss.

**PostgreSQL + Elasticsearch (4)** solves a deferred problem. Full-text ranking across the
candidate archive belongs to re-matching, which is parked; nothing in the current use cases
searches resume text. Adopting a search cluster now buys a capability nothing uses and costs
memory on a cluster already carrying six services and two databases.

**One shared database (6)** would undo [ADR-001](ADR-001-service-decomposition.md) from below:
services sharing tables are not independently deployable, and the first cross-service join
silently makes two services one.

### Implications

**What this buys us**

- Constraints do work that application code cannot afford to do: a candidate cannot belong to no
  batch, a decision cannot reference a job opening that does not exist.
- Anonymisation becomes coarse and auditable rather than a field-by-field rewrite — *drop the
  documents, keep the counts*.
- Changing the parser or a prompt changes a document shape, not a schema: no migration, no
  coordinated release.
- The audit log lives in an append-only relational table where its integrity is enforced, which is
  what makes it usable as evidence.

**What it costs us**

- **There is no transaction across the two stores.** A candidate row can exist with no profile
  document and — the case that matters — a profile document can survive a successful delete.
  Erasure must therefore be a sequenced operation with a verification pass and a *deletion
  pending* state, completing within the 30 days allowed. Deleting the object-storage file is a
  third step with the same problem.
- **Two stores to back up, monitor, secure and hold credentials for**, and two places a breach can
  happen. The privacy surface is larger than with one store — the honest cost of the anonymisation
  benefit above.
- **The dashboard cannot join across the split.** Metrics must be maintained in PostgreSQL from
  services as work completes, not computed on demand. More code, and a lost update is a
  permanently wrong number until something reconciles it.
- **The split rule has to be enforced by people.** The first time someone puts a decision reason in
  MongoDB "because it was easier", the boundary starts rotting and the anonymisation argument stops
  being true. This is a review responsibility, not an assumption.
- **Two data models for four students to learn**, including the parts that bite: migrations and
  connection pooling on one side, index behaviour and document-size limits on the other.
- **Referential integrity stops at the service boundary anyway** — a foreign key cannot span
  services, so some invariants are enforced by convention and by calls whichever store holds them.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — per-service data ownership, which this record
  refines into a concrete store-by-store split.
- [ADR-002](ADR-002-async-screening-pipeline.md) — writes exactly one derived document per
  successfully completed unit of work and one status transition per entry.
- **How model access is provided** — the model version recorded with each score is stored beside
  it, which is what makes an old justification reconstructable.

### Related requirements

- **Sign-in and access management** (users, sessions, memberships, roles), **job creation**
  (openings and criteria), **screening** (batches, profiles, scores, justifications, overrides,
  decisions), **interview questions** (guides), **pipeline monitoring** (counters) and
  **retention** (policy, audit log, both expiry actions) all map onto the split above.
- **FR-0.4, FR-0.6** — roles as a foreign key, not a convention. **FR-1.7** — the opening, its
  criteria and the original description. **FR-2.5, FR-2.8, FR-2.9** — score, must-have check,
  override and shortlist decision, all in PostgreSQL so the ranked list never depends on a document
  read. **FR-2.11** — collection date. **FR-3.1** — guides retained. **FR-5.1** — retention policy.
  **FR-5.3 to FR-5.5** — erasure across both stores and object storage, anonymisation as *drop the
  documents, keep the counts*, and an audit log that names no one. **FR-5.8** — deletion pending.
- **NFR-03** — ranked list served from PostgreSQL columns within two seconds. **NFR-11** —
  encryption at rest across both stores *and* object storage: three places to configure, not one.
  **NFR-12** — access audit log, append-only. **NFR-13** — the erasure sequence completes within 30
  days. **NFR-15** — per-criterion evidence persisted with the score.
- Required of us: two database types, with a stated reason for what lives in each.

### Related artifacts

The Service–Operations–Collaborators table will name the store each service owns.

### Related principles

- *Explainability is required, not optional* — the justification is stored with the score, not
  regenerated on demand, so what a recruiter saw is what can be reviewed later.
- *Easily reversible* — collapsing MongoDB into `jsonb` later is a migration; splitting a single
  store after retention logic has been written around it is not.

---

## Notes

Recorded plainly, because a future reader will suspect it anyway: the two-database requirement is
what forced the question. What we did with it was not arbitrary — the anonymisation argument
stands on its own and would have justified the split regardless — but it is fair to say we would
probably have shipped `jsonb` first and reached this split only when anonymisation was
implemented.

Redis is likely to appear later as a cache for model responses. When it does, it is a cache and
must be treated as one: nothing may be true only in Redis. That is a note, not a decision, and
needs its own record if it happens.
