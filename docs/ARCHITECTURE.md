# Architecture Diagram — version 1

*Microservice Design with Collaborations.* This page is the diagram half of the deliverable; the
operations half is the [Service–Operations–Collaborators table](SERVICE-OPERATIONS-COLLABORATORS.md).
The two describe the same services and must agree — the table lists every operation, this page
shows who calls whom.

This is the first version of the architecture. It will be revised in later progress checkpoints;
what it must already do is show every component and operation the three core business use cases
need, end to end: **UC-1** create a job opening, **UC-2** batch-screen resumes, **UC-3** generate
interview questions.

![HireAssist architecture diagram](diagrams/architecture-diagram.svg)

*The diagram is a hand-laid SVG, **[diagrams/architecture-diagram.svg](diagrams/architecture-diagram.svg)**,
drawn in the style of the FTGO example used in the course. It is plain text: edit the labels,
boxes and arrows in the file directly and commit it — there is no separate source to re-render.*

---

## How to read it

- **An arrow `A → B` means "A calls B."** Responses are not drawn. Where a caller uses what it
  got from one call to make another, both arrows leave the caller — `A → B` and `A → C` — never a
  chain `A → B → C`, which would mean B calls C.
- **A plain line between a service and a data store means the service owns that store.** No
  service reads another service's store; it calls the owning service's operation instead.
- **Arrows are not labelled with operations** — the table lists them, and the traces below walk
  through each use case. **Every call in version 1 is REST**, as the course asks for a first
  version. The protocols the ADRs decided — gRPC into the AI Service (ADR-001) and RabbitMQ
  between Hiring and Resume Processing (ADR-002) — will replace the corresponding arrows in a
  later version without changing which service calls which.
- **External systems** sit on the right, outside the deployment boundary. Each is reached through
  an adapter inside the service that calls it, so no domain logic depends on a provider's API
  directly.
- **One deployment serves one customer company** (ADR-006). The dashed boundary is that
  deployment; there is no tenant identity anywhere inside it.

## Actors

| Actor | Initiates | Reaches the system through |
|---|---|---|
| **Recruiter** | UC-1, UC-2, UC-3, UC-4 | the HireAssist Web UI → API Gateway |
| **Admin** | everything a Recruiter can, plus UC-5 (retention policy) and UC-6 (members and roles) | the same UI — Admin specialises Recruiter |
| **System Scheduler** | UC-4 staleness checks and UC-5 retention evaluation, on a timer | calls Compliance & Insights directly; no UI |
| *Candidate* | nothing — an indirect actor with no interface | their resume arrives as a file the Recruiter uploads; not drawn |

Signing in (UC-0) is not a business use case. A Guest signs in through the gateway, which asks the
Identity Service to validate the session on every later request. The Identity Service is drawn
because the table lists it, but it is connected only to the gateway — no other service calls it.

## Services and what each owns

| Service | Business capability | Use cases | Owns (private store) |
|---|---|---|---|
| **API Gateway** | Single entry point: TLS, session validation, rate limiting, routing | — | nothing |
| **Identity Service** | Who may sign in and with which role | UC-0, UC-6 | PostgreSQL — users, roles, sessions |
| **Hiring Service** | The hiring record: openings and criteria, screening batches and their results, shortlist decisions, interview guides | UC-1, UC-2, UC-3 | PostgreSQL — job openings, criteria, batches, screening results, decisions; MongoDB — interview guides |
| **Resume Processing Service** | Turning one resume file into a scored candidate profile | UC-2 (per-resume work) | MongoDB — candidate profiles, parsed text, scoring evidence and justifications |
| **AI Service** | Every call to the language model, behind domain operations | UC-1, UC-2, UC-3 | nothing — prompts and the model credential only |
| **Compliance & Insights Service** | Retention enforcement and the pipeline dashboard | UC-4, UC-5 | PostgreSQL — retention policy, pipeline metrics, erasure audit log |

Resume files themselves live in object storage, written by Hiring when a batch is uploaded and
read by Resume Processing when it works; neither database holds them.

## The three business use cases, traced

The check the course asks for: *Actor → operation → responsible service → collaborators → where
the data lands.*

**UC-1 — Create a job opening.** Recruiter → `createJobOpening()` on **Hiring**. Hiring →
**AI Service** `deriveCriteriaFromDescription()` → LLM Provider. Hiring returns the proposed
criteria; the Recruiter revises them (`reviseScreeningCriteria()`) and confirms. Hiring stores the
opening and its criteria in its PostgreSQL; **Compliance & Insights** later reads the
*JobOpeningCreated* pipeline event through Hiring's `getPipelineEvents()` for the dashboard.

**UC-2 — Batch-screen resumes.** Recruiter → `submitScreeningBatch()` on **Hiring**. Hiring stores
each PDF in **Object Storage** (`storeResumeFile()`), creates the batch with one pending entry per
resume, calls **Resume Processing** `screenResume()` once per resume, and returns the batch id.
For each resume, Resume Processing fetches the file (`fetchResumeFile()`), extracts the text,
calls **AI Service** `extractProfileFields()` and `scoreAgainstCriteria()` — the criteria come from
Hiring's `getScreeningCriteria()` — stores the profile and the full scoring evidence in its
MongoDB, and calls Hiring `recordScreeningResult()`. Hiring records the score and justification
against the batch entry in its PostgreSQL and streams it to the ranked list
(`getRankedResults()`). When every entry is terminal, Hiring sends the batch-finished email
through the **Email Provider**. The Recruiter overrides scores (`overrideScore()`) and shortlists
(`recordDecision()`), all in Hiring.

**UC-3 — Generate interview questions.** Recruiter → `generateInterviewGuide()` on **Hiring** for a
shortlisted candidate. Hiring → **Resume Processing** `getCandidateProfile()` for the profile, then
→ **AI Service** `generateInterviewQuestions()` → LLM Provider. Hiring stores the guide in
its MongoDB; the Recruiter revises it (`reviseGuide()`) and later records notes against it
(`recordInterviewNote()`).

**UC-4 and UC-5** follow the same pattern from the other side: the **System Scheduler** calls
**Compliance & Insights** `evaluateStaleness()` and `evaluateRetention()`; the dashboard is served
from counters Compliance maintains from Hiring's `getPipelineEvents()` and Identity's
`listAuthorisationRejections()`; erasure is
orchestrated by Compliance calling `eraseHiringData()` on Hiring and `eraseCandidateProfile()` on
Resume Processing, and the audit entry is written in Compliance's own PostgreSQL.

## Why version 1 is REST-only

The course asks that a first version use REST throughout and leave message brokers for a later
checkpoint. Version 1 does that: every arrow is a REST call, including the per-resume hand-off
from Hiring to Resume Processing (`screenResume()` / `recordScreeningResult()`) and the way
Compliance & Insights learns about pipeline events (`getPipelineEvents()`). This is a presentation
choice for the first version, not a reversal of the decisions: ADR-001 chose gRPC for the AI
Service boundary and ADR-002 chose RabbitMQ, with one message per resume, for screening. When
those are drawn in, the Hiring ↔ Resume Processing arrows become messages through a broker and the
AI Service arrows become gRPC; no service gains or loses a responsibility.
