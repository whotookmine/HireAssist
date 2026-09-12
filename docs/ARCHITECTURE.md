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

*The diagram is maintained as code in
**[diagrams/architecture-diagram.puml](diagrams/architecture-diagram.puml)** (PlantUML). The image
above is rendered from that file; edit the source and re-render rather than editing the image.*

---

## How to read it

- **An arrow `A → B` means "A calls B."** Responses are not drawn. Where a caller uses what it
  got from one call to make another, both arrows leave the caller — `A → B` and `A → C` — never a
  chain `A → B → C`, which would mean B calls C.
- **A plain line between a service and a data store means the service owns that store.** No
  service reads another service's store; it calls the owning service's operation instead.
- **Arrow labels** carry the protocol and the operations from the table. REST is the default at
  the gateway; the AI Service is called over gRPC; messages on RabbitMQ are named by their event.
- **External systems** sit on the right, outside the deployment boundary. Each is reached through
  an adapter inside the service that calls it (named on the arrow), so no domain logic depends on
  a provider's API directly.
- **One deployment serves one customer company** (ADR-006). The dashed boundary is that
  deployment; there is no tenant identity anywhere inside it.

## Actors

| Actor | Initiates | Reaches the system through |
|---|---|---|
| **Recruiter** | UC-1, UC-2, UC-3, UC-4 | the Recruiter Web UI → API Gateway |
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
**AI Service** `deriveCriteriaFromDescription()` (gRPC) → LLM Provider. Hiring returns the proposed
criteria; the Recruiter revises them (`reviseScreeningCriteria()`) and confirms. Hiring stores the
opening and its criteria in its PostgreSQL and publishes a *JobOpeningCreated* pipeline event that
**Compliance & Insights** consumes for the dashboard.

**UC-2 — Batch-screen resumes.** Recruiter → `submitScreeningBatch()` on **Hiring**. Hiring stores
each PDF in **Object Storage** (`storeResumeFile()`), creates the batch with one pending entry per
resume, publishes one *ResumeSubmitted* message per resume to **RabbitMQ**, and returns the batch
id. **Resume Processing** consumes each message: fetches the file (`fetchResumeFile()`), extracts
the text, calls **AI Service** `extractProfileFields()` and `scoreAgainstCriteria()` (gRPC) — the
criteria come from Hiring's `getScreeningCriteria()` — stores the profile and the full scoring
evidence in its MongoDB, and publishes *ResumeScored*. Hiring consumes *ResumeScored*, records the
score and justification against the batch entry in its PostgreSQL, and streams it to the ranked
list (`getRankedResults()`). When every entry is terminal, Hiring sends the batch-finished email
through the **Email Provider**. The Recruiter overrides scores (`overrideScore()`) and shortlists
(`recordDecision()`), all in Hiring.

**UC-3 — Generate interview questions.** Recruiter → `generateInterviewGuide()` on **Hiring** for a
shortlisted candidate. Hiring → **Resume Processing** `getCandidateProfile()` for the profile, then
→ **AI Service** `generateInterviewQuestions()` (gRPC) → LLM Provider. Hiring stores the guide in
its MongoDB; the Recruiter revises it (`reviseGuide()`) and later records notes against it
(`recordInterviewNote()`).

**UC-4 and UC-5** follow the same pattern from the other side: the **System Scheduler** calls
**Compliance & Insights** `evaluateStaleness()` and `evaluateRetention()`; the dashboard is served
from counters Compliance maintains by consuming Hiring's pipeline events; erasure is
orchestrated by Compliance calling `eraseHiringData()` on Hiring and `eraseCandidateProfile()` on
Resume Processing, and the audit entry is written in Compliance's own PostgreSQL.

## Why the broker is already in version 1

The course allows a first version with REST everywhere and no message broker. RabbitMQ appears
here anyway because it is not decoration: UC-2 accepts a batch of up to 200 resumes and must
return immediately, and each resume is minutes of model work that may fail independently. That
was decided, with the alternatives, in ADR-002, and the table already lists the *ResumeSubmitted*
and *ResumeScored* messages. Drawing the same flow as a synchronous call would show an
architecture we have decided not to build.
