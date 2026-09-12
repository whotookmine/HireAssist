# Architecture Decision Records

Each significant architectural decision is recorded here using
[TEMPLATE.md](TEMPLATE.md), which follows the **Jeff Tyree & Art Akerman** decision record
template from the course slides — grouped as *Summary · Details · Related · Notes*, with
optional fields marked in the template.

The course requires **at least 3 ADRs** as part of the project proposal submission.

---

## Index

| ID | Decision | Status | Date |
|---|---|---|---|
| [ADR-001](ADR-001-service-decomposition.md) | Capability-aligned service decomposition behind an API gateway | Accepted | 2026-09-11 |
| [ADR-002](ADR-002-async-screening-pipeline.md) | One independently retried unit of work per resume, held in a claimable work table | Accepted | 2026-09-11 |
| [ADR-003](ADR-003-polyglot-persistence.md) | PostgreSQL as system of record, MongoDB for AI-derived documents | Accepted | 2026-09-11 |
| [ADR-004](ADR-004-llm-access.md) | All model access through one AI Service, on a managed API that does not train on our data | Accepted | 2026-09-11 |
| [ADR-005](ADR-005-per-service-language.md) | Each service chooses its own language and framework, within shared contracts | **Proposed** | 2026-09-11 |
| [ADR-006](ADR-006-single-tenant-deployment.md) | One deployment per customer company — the workspace concept is removed | Accepted | 2026-09-11 |

The four are best read in order: ADR-001 draws the boundaries, ADR-002 fills in the busiest one,
ADR-003 says what each side stores, and ADR-004 fills in the dependency the others are built to
survive. ADR-002 and ADR-004 are a deliberate pair — running without a fallback model is only
acceptable because no accepted work is lost (NFR-10).

Of the eight candidates listed below on 2026-09-11, five are answered by these records: the
model-provider question by ADR-004; work-queue topology, worker scaling and delivery guarantee by
ADR-002; the scorer boundary by ADR-001 and ADR-004, with per-criterion evidence persistence by
ADR-003; service discovery by ADR-001; and the datastore split by ADR-003.

---

## Candidate decisions

Identified from the architecturally significant requirements in
[../NON-FUNCTIONAL-REQUIREMENTS.md](../NON-FUNCTIONAL-REQUIREMENTS.md) and from open questions in
[../CONTEXT.md](../CONTEXT.md).

| Decision | Forced by |
|---|---|
| Tiered screening pipeline — deterministic filter before the model? | NFR-02, NFR-06, NFR-08 |
| Erasure cascade — orchestration or choreography, and how the verification pass works across the two stores ADR-003 introduced | FR-5.3, FR-5.8, NFR-13 |
| Resume ingestion channel — upload only, or an automated adapter | Open question |
| Front-end framework and the shape of the recruiter-facing web application | Course requirement (UI for demonstration), NFR-16 |
| Session and token mechanism for UC-0 | FR-0.1, FR-0.2, FR-0.6 |
| Deployment and upgrade pipeline for N customer instances | ADR-006 |
| Scheduler shared by UC-4 and UC-5 — where the timer runs, behaviour with more than one replica, resuming a sweep that dies half-way | FR-4.2, FR-5.2 |
| Repository structure once implementation starts — monorepo layout, and where the shared OpenAPI definitions live | Open question, ADR-005 |
| Primary and secondary owner for each service | ADR-005 rule 5 |

---

## Conventions

- File name: `ADR-NNN-short-slug.md`, numbered sequentially from `001`.
- Numbers are never reused, even if an ADR is superseded or withdrawn.
- Superseding an ADR does not edit the original — set its status to
  *Superseded by ADR-NNN* and write a new one.
- Add a row to the index in the same commit that adds the ADR.
