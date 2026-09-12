# ADR-001: Capability-aligned service decomposition behind an API gateway

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of building HireAssist as microservices, facing three workloads that differ in
> who triggers them, how long they take and what data they own, we decided to split the system
> into five services behind one API gateway — boundaries drawn by capability, not by technical
> layer — to achieve independent scaling of screening and one enforcement point for identity,
> accepting that data ownership is now distributed and that erasure and reporting must cross
> service boundaries.

---

## Summary

### Issue

Nothing else can be designed until the boundaries exist: the datastores, the model integration and
the way work is handed between services all sit on them. Four forces shape the split.

**Batch screening contains two workloads that do not belong in one process.** Accepting a batch
of resumes must return in milliseconds. Screening one of them is parsing plus model calls —
seconds normally, minutes for a long file, never when the provider is down. Share a process and
the recruiter-facing API inherits the latency and availability of the slowest external dependency
in the system.

**Retention enforcement is clock-driven and destructive.** It deletes candidate data wherever it
lives and must prove it did. That is a different responsibility from serving a screen, it runs
when nobody is watching, and it needs an owner: a privacy obligation with no clear home is one
nobody implements.

**Every request must carry a verified identity and role.** A rule enforced in five places is a
rule enforced in four places plus a bug.

**The team is four students learning this stack while building it.** Every service costs a
deployment, a pipeline, a schema and a place for a bug to hide. Over-decomposition is the most
likely way this project fails to ship.

### Decision

Five services and one gateway, aligned to capabilities:

| Service | Owns |
|---|---|
| **API Gateway** | No domain data. Terminates TLS, validates the session token, resolves workspace and role, rate-limits, routes. |
| **Identity & Workspace Service** | Accounts, workspaces, memberships, roles, sessions |
| **Hiring Service** | Job openings, criteria, screening batches, decisions, interview guides |
| **Resume Processing Service** | Candidate profiles, parsed resume text |
| **AI Service** | Prompts and the model credential — no domain data |
| **Compliance & Insights Service** | Retention policy, pipeline metrics, audit log |

**Every boundary is REST over HTTP/JSON — one protocol, no exceptions.** Client to gateway,
gateway to service, and service to service all speak the same thing. It is browser-facing and
public at the edge, readable in a network tab, debuggable without a client toolchain, and it
means four students learn one wire format instead of three.

Two boundaries would each have argued for something else, and are deliberately served by REST
anyway:

- **Hiring and Resume Processing → AI Service** is a high-volume internal call whose contract must
  not drift: *(profile, criteria) → (score, must-have check, justification)*. It is a JSON request
  and a JSON response, held stable by an OpenAPI document rather than by a generated stub.
- **Hiring → Resume Processing** is long-running work, not a query. REST cannot make it
  synchronous: Hiring hands over one resume and gets an immediate acknowledgement, and Resume
  Processing calls back with the result when it has one. Screening remains asynchronous; what
  carries the asynchrony is a separate decision.

**Service discovery is Kubernetes.** Each service is a Deployment behind a ClusterIP Service;
services address each other by in-cluster DNS (`ai-service.hireassist.svc`) and kube-proxy
balances across healthy Pods. Readiness probes decide membership, so discovery and health
checking are one mechanism rather than two that can disagree. No separate registry is deployed.

### Status

**Accepted**

- *Amended 2026-09-11 by ADR-005: the assumption of a single backend language is withdrawn. Each
  service chooses its own within shared contracts; no replacement assumption is in force while
  that record is Proposed.*
- *Amended 2026-09-11 by ADR-006: the workspace concept is removed. Each customer runs its own
  deployment, so the isolation force in Issue, the gateway's workspace resolution and the
  internal-call cost below no longer apply. Identity & Workspace becomes the Identity Service.*

### Group

Decomposition · Communication

---

## Details

### Assumptions

- Go is the backend language for all services. One language keeps four students able to read each
  other's code.
- Deployment is to Kubernetes — managed, or local for the demo. Worth re-checking early: if the
  team cannot get a cluster running, discovery must be reconsidered, because everything depends
  on it.
- Load is one company's hiring: tens of concurrent recruiters, batches of 20–200 resumes. Nothing
  here is sized for internet-scale traffic.
- The same four people operate every service. There is no platform team.

### Constraints

- An API gateway and a documented discovery approach are required of us, and so, eventually, is a
  spread of communication styles — at least one service reached by REST, one by RPC and one driven
  by a message broker. **This record knowingly does not satisfy the second half.** We are building
  the boundaries first and the protocol variety second, on the argument that a boundary in the
  wrong place is expensive to move and a protocol is not. The obligation is deferred, not denied.
- Privacy law makes the *location* of personal data a design concern: the fewer services holding
  resume content, the smaller the surface that retention, access control and breach response
  must cover.

### Positions

1. **Modular monolith plus one background worker.** Two deployables, one schema, no network
   between the parts. The least work, and defensible in production at this size.
2. **One service per use case** — eight services.
3. **Decomposition by technical layer** — API, parsing, scoring, persistence.
4. **The chosen split with the AI Service merged into Resume Processing** — four services.
5. **The chosen split with compliance folded into Hiring**, run by a scheduler inside it.
6. **A separate service registry** such as Consul, instead of Kubernetes DNS.
7. **A protocol chosen per boundary** — REST at the edge, gRPC into the AI Service, a message
   broker between Hiring and Resume Processing — instead of REST everywhere.

### Argument

**Against the monolith (1).** It puts the recruiter-facing API in the same process as work that
blocks on an external model, so a provider outage degrades sign-in and the dashboard, which do not
depend on the model at all. It also gives one worker pool to two workloads with opposite resource
profiles: parsing is CPU-bound and finishes in seconds, while scoring waits on a remote call —
cheap in CPU, long in wall-clock, bounded by the provider's rate limit rather than our hardware.
One pool must be sized for one or the other, which makes throughput hard to scale and harder to
demonstrate.

**Against one service per use case (2).** Creating a job, screening against it and generating
interview questions are three views of the same two aggregates — the job opening and the
candidate profile. Splitting them puts a network call and an eventual-consistency problem between
"the criteria" and "the score against the criteria" and buys nothing: they deploy together, change
together, scale together. Eight pipelines, eight schemas, hypothetical benefit.

**Against layered services (3).** A layer is not a capability. Adding one field to a criterion
would touch the API, scoring and persistence services — every change a coordinated three-service
release. This is the split that looks organised on a diagram and behaves worst in practice.

**Against merging the AI Service (4).** Extracting criteria from a paragraph and writing an
interview guide are interactive; neither is batch work. Merging would route interactive calls
through a batch worker, or duplicate prompt and credential handling in two codebases. One service
for every model call is also what makes a single point of control possible: one place holds the
credential, caps spending, and changes when the provider changes.

**Against folding compliance into Hiring (5).** A retention sweep would compete with recruiter
traffic in one process. More importantly, retention must be auditable in isolation: *which
component may delete candidate data?* should have a one-word answer.

**Against a separate registry (6).** It means running and explaining a second discovery mechanism
when the platform already provides one, and it puts registration in application code, where a
service that crashes without deregistering leaves a stale entry. Kubernetes derives the same
information from probes it already runs. The cost is that the team must learn Kubernetes.

**Against a protocol per boundary (7).** This is the one we expect to revisit, and it is the one
we are most likely to be told is wrong. Each specialised choice is individually defensible: a
generated stub cannot drift the way a hand-written JSON client can, and a broker gives retry,
backpressure and dead-lettering for free rather than as code we write. What decided it is that
none of those benefits arrives before the boundaries themselves are proven correct, and all of
them arrive with a bill paid up front — a second wire format, a code-generation step in every
build, broker infrastructure to run locally on four laptops, and three failure modes to learn
instead of one.

The boundaries are the expensive thing to get wrong. A boundary in the wrong place is re-owned
data, a re-cut schema and a coordinated release; a protocol in the wrong place is one adapter
rewritten behind an interface that already exists. So we place the boundaries now, run them all
over the protocol every one of us can already debug, and change a protocol when a boundary
demonstrates it needs one — the AI call when contract drift actually bites, the screening
handover when our own retry logic starts reimplementing a broker badly. The risk we accept is
that "first" becomes "only", and that the day we do need a broker we will be retrofitting it into
code that assumed a synchronous acknowledgement.

**For the chosen split.** Its three boundaries fall where three properties genuinely differ:
*who triggers the work* — a human, a queued message, or the clock; *how long it may take* —
milliseconds, minutes, or a full sweep; and *what it owns* — the transactional hiring record, the
derived profile, or the retention policy and audit log. A boundary separating things that differ
on all three will still be in the right place in six months.

### Implications

**What this buys us**

- Screening scales by adding Resume Processing replicas without touching the recruiter-facing
  API. This is the quality attribute the project demonstrates.
- The gateway is the one place a request is bound to a workspace and a role, so isolation is
  structural rather than a convention repeated in five codebases.
- The AI Service is the entire blast radius of the model credential and the entire token cost
  centre, making the provider a configuration choice rather than a rewrite.
- Scoring stays a separable *(profile, criteria)* operation callable by anything — the property
  any future re-matching over the candidate archive depends on.

**What it costs us**

- **Retention must erase data it does not own.** The policy lives in one service, the data in
  two others, so erasure becomes a multi-service protocol with a verification step. "Deleted"
  becomes eventually consistent — precisely what a regulator asks about.
- **The dashboard cannot be a query.** Metrics span services, so they must be maintained
  incrementally from domain events. Numbers can lag, and a lost event means a permanently wrong
  count unless something reconciles.
- **No distributed transactions.** An accepted batch whose messages are never consumed is
  inconsistent state nothing detects on its own; a reconciliation sweep becomes a requirement.
- **Workspace identity must travel on every internal call.** If only
  the gateway checks, an internal caller with the wrong id silently reads another company's
  candidates. Every contract needs the field and every service must enforce it, not merely
  receive it.
- **The team must learn Kubernetes** — the largest schedule risk this decision creates, landing
  on all four members. Holding the protocol count at one is partly how we afford it.
- **Six deployables plus two databases is heavy for a laptop.** Local development needs resource
  limits or a reduced profile, or the demo machine becomes the constraint.
- **One protocol everywhere means the hard parts are ours to write.** REST gives us no retry, no
  backpressure, no delivery guarantee and no dead-letter path; wherever a boundary needs those, we
  build them by hand and test them ourselves. The first place that bites is the screening
  handover, and the honest risk is that we rebuild a broker slowly and badly instead of running
  one.
- **The contract between two services is a document, not a compiler error.** With no generated
  stubs, a field renamed on one side fails at runtime in the other. Contract tests have to do the
  work a code generator would otherwise have done for free.

---

## Related

### Related decisions

None precede this one. It is the first architectural decision recorded, and every later one
inherits the structure it sets. It **opens**, without settling, four questions — each a decision
in its own right:

- How work crosses the Hiring → Resume Processing boundary: what holds a resume between handover
  and result, the delivery guarantee, and retry.
- Where the scoring model runs and who provides it.
- What each side of these boundaries stores.
- What the services are written in, and how cross-cutting concerns are handled if that differs.

*(Later amendments are listed under Status.)*

### Related requirements

- **Sign-in and access management** (UC-0, UC-6) are served by the Identity & Workspace Service
  with authorisation at the gateway; **job creation, screening and interview questions**
  (UC-1–UC-3) by Hiring, Resume Processing and the AI Service; **pipeline monitoring and
  retention** (UC-4, UC-5) by Compliance & Insights.
- **FR-0.3, FR-0.6** — workspace scoping and role enforcement, bound to the request at the
  gateway.
- **NFR-04, NFR-07** — a batch is acknowledged in seconds while screening scales by adding
  replicas; the second is the demonstrated quality attribute.
- **NFR-10** — no failure in the batch path may lose accepted work or take the interactive path
  down with it.
- **NFR-17** — the scoring model and prompt change inside the AI Service alone.
- **FR-5.3, FR-5.8** — erasure must reach every service holding candidate data, which this
  decomposition turns into a cross-service protocol.

### Related artifacts

The architecture diagram and the Service–Operations–Collaborators table derive directly from this
decomposition.

### Related principles

- *The human still decides* — the gateway and Identity Service exist so the decision-maker is
  always an identified person.
- *Easily reversible* — merging two of these services later is a refactor; splitting a monolith
  later is a project.

---

## Notes

The strongest objection was that a modular monolith is the better engineering choice at this size,
and that we are decomposing because we were told to. Partly true, and worth recording: without
that constraint we would likely have started with two deployables and split when a boundary hurt.
What survives it is the *shape* of the split — these boundaries follow trigger, latency and
ownership, not the requirement to use three protocols.

The Argument was rewritten after that objection to rest on the product's own evidence alone: fault
isolation of the model path, and the opposite resource profiles of parsing and scoring. This
record should stand if the constraint were removed. The dissent is kept because it was real and
may be right.

We considered splitting the communication decision into its own record, then kept it here. With
one protocol on every boundary there is no per-boundary rationale to separate out — the choice is
a property of the decomposition, made once. If a boundary later takes a different protocol, *that*
is its own record.

The service owning monitoring and retention is named **Compliance & Insights** rather than
*Insights & Notifications* because retention ownership is its defining responsibility, and because
notifications are not a service — each event's owner emits its own. A shared notification channel
may exist later; it would be infrastructure, not a boundary.
