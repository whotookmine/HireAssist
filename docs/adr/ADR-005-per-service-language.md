# ADR-005: Each service chooses its own language and framework, within shared contracts

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of five services owned by four developers whose strengths and ecosystems differ,
> facing a choice between one backend language for everything and a language chosen per service,
> we propose that **each service picks its own language and framework within a fixed set of shared
> contracts and operational rules**, to achieve ecosystem fit where it matters and to let each
> member work and learn where they are productive, accepting duplicated cross-cutting code, a
> heavier build, and a worse bus factor than a single-language system would have.
>
> *Proposed, not yet accepted — the languages themselves are deliberately left open.*

---

## Summary

### Issue

[ADR-001](ADR-001-service-decomposition.md) recorded, in its Assumptions, that "Go is the backend
language for all services". That was never decided. It was written as an assumption, carried no
alternatives and no argument, and it is not what the team intends to do — members differ in what
they know and in what they want to learn, and two of the five services sit in an ecosystem where
Go is the weaker choice.

It also sat awkwardly against ADR-001's own reasoning. One of the three arguments for splitting
the system at all is that four developers with different strengths can own services and work in
parallel. If every service is written in the same language regardless of fit, that argument is
decorative: the boundaries exist but nobody works differently because of them.

The forces:

- **Ecosystem fit is not uniform across the services.** Parsing PDFs and calling a language model
  are Python's home ground — `pypdf`/`PyMuPDF` for extraction, mature provider SDKs, tokenisation
  and prompt tooling. Writing those two in Go means reimplementing or wrapping what already
  exists. The other three services are ordinary transactional web services that any mainstream
  stack does well.
- **The team is four part-time developers with different strengths**, under a deadline, who also
  need to *learn* from this project — it is a course, not only a delivery.
- **Contracts already cross every boundary** — every service call is REST over HTTP/JSON, at the
  gateway and between services alike. They must be explicit whether or not the languages differ.
- **Whatever we choose, four people must still operate it**, correlate its logs, and cover for
  each other in demo week.

### Decision

**Each service chooses its own language and framework. Its owner decides.**

*No language is fixed by this record.* What is settled is that a single mandated language is not
assumed, and what conditions a per-service choice must meet. The choices themselves wait on
service ownership, which is not yet assigned.

That freedom is bounded by rules that apply to every service regardless of language:

1. **Contracts are written once and generated from, never hand-written twice.** Every boundary —
   the gateway's and every service-to-service call — is defined by an OpenAPI document, and
   clients and servers are generated from it in each language. A contract written by hand in two
   languages drifts. With no schema-carrying wire format underneath, the OpenAPI document is the
   only thing preventing that, so it is authoritative rather than descriptive: it is written
   before the endpoint, and a contract test on each side proves both still match it.
2. **Schema definitions live in one place**, versioned with the system, not copied into each
   service. Exactly where is part of the still-open repository-structure decision.
3. **One observability standard.** Structured JSON logs with a shared field set, and OpenTelemetry
   traces propagating the same correlation id, carried as an HTTP header on every call. Every
   mainstream language has an OTel SDK; without this, a
   resume's journey cannot be followed across five services in three languages.
4. **Every service runs the same way.** A Dockerfile, a health endpoint and a readiness endpoint,
   startup configuration from the environment, and a Kubernetes manifest. The platform must not
   need to know what is inside the container.
5. **Every service has a primary and a secondary owner**, and they are named in the repository.
   The secondary must be able to build, run and deploy the service. Polyglot makes this rule
   mandatory rather than advisable: a teammate cannot improvise in a language they have never
   used, on the morning of a demo.
6. **A new language needs a reason recorded here.** Ecosystem fit or an owner's genuine strength
   qualifies; curiosity about a fourth stack in week nine does not.

**Allocation is deliberately not decided here.** The table below records where the ecosystem
argument is strong and where it is not, as input to the choice — not as the choice:

| Service | Ecosystem pull | Note |
|---|---|---|
| **AI Service** | Strong — toward Python | Provider SDKs, tokenisation and prompt tooling are first-class there; this service changes most often (NFR-17) and benefits most from the shortest edit-run loop |
| **Resume Processing** | Strong — toward Python | PDF text extraction and Thai/English text handling have the strongest libraries there (FR-2.3, NFR-01) |
| **Hiring Service** | Weak | Ordinary transactional service with the largest domain surface; the owner's fluency will matter more than the ecosystem |
| **Identity & Workspace Service** | Weak, with a caveat | Security-sensitive and small; whatever is chosen should have mature, well-trodden auth libraries |
| **Compliance & Insights** | None | Scheduled work and event consumption; no unusual library needs |
| **API Gateway** | None | May be an off-the-shelf gateway rather than code we write |

### Status

**Proposed.**

The principle and the guardrails are ready; the choices are not. Before this can move to
*Accepted*:

1. Each service needs a **primary and secondary owner** (rule 5) — the owner is who makes the
   choice, so nothing can be chosen until ownership exists.
2. The team confirms it accepts the cost of more than one language, and agrees an upper bound on
   how many.
3. Each proposed language is checked against the gate in *Assumptions* — a maintained OpenAPI
   generator, an OpenTelemetry SDK, and clients for whichever of PostgreSQL and MongoDB that
   service needs.

Until then, ADR-001's assumption that a single language is used is withdrawn, and no replacement
is in force.

### Group

Implementation · Team

---

## Details

### Assumptions

- Each of the five services has, or will have, a primary owner among the four members.
- The team is willing to pay a per-language cost for cross-cutting concerns rather than share one
  internal library.
- Every language chosen has a maintained OpenAPI code generator, an OpenTelemetry SDK, and a
  client for PostgreSQL and MongoDB as its service requires. This is true of every mainstream
  candidate, but it is a gate on any unusual one.
- No more than three languages across the system. This is not a rule, but beyond three the costs
  below stop being manageable for four people.

### Constraints

- Four part-time developers and a fixed deadline. Time spent fighting an unfamiliar toolchain is
  time not spent on the deliverable.
- Everything must run on a laptop for the demo, alongside PostgreSQL and MongoDB.
- Contracts cross language boundaries regardless of this decision, because the services call each
  other whatever they are written in.

### Positions

1. **One language for all services — Go.** What ADR-001 assumed. Cross-cutting concerns are
   written once and shared as an internal module; any member can read and fix any service; one
   toolchain, one CI configuration, one set of base images.
2. **One language for all services — Python.** Same benefits, and it suits the two AI-facing
   services; weaker for the transactional services and for long-running processes.
3. **Language per service, chosen by its owner, within shared contracts.** *(Selected.)*
4. **Two languages by tier** — Python for the AI-facing pair, one fixed language for the other
   three, mandated rather than chosen.
5. **Per-service choice with no guardrails.** The same freedom without the contract, telemetry
   and packaging rules.

### Argument

**Against a single language (1 and 2):** whichever is chosen is the wrong one for part of the
system. Go for the AI Service means wrapping provider SDKs and PDF libraries that already exist
in Python; Python for Hiring and Identity means giving up static typing on the services with the
largest domain surface. It also hollows out one of ADR-001's three reasons for decomposing at
all: if nobody works differently because of the boundaries, the team-structure argument was
never real.

**Against mandating two languages by tier (4):** it captures most of the ecosystem benefit and is
genuinely close to what we chose. It was rejected because the mandate buys little that the
guardrails do not already buy, and it overrides the owner on the three services where ecosystem
fit is not the deciding factor — exactly where an owner's fluency is what determines whether the
service ships. Position 3 reaches the same allocation by ecosystem where ecosystem matters, and
by the person doing the work where it does not.

**Against unbounded polyglot (5):** freedom without generated contracts is how two services end
up disagreeing about a field name in week ten, and without a shared telemetry standard a failed
resume cannot be traced across three runtimes. The guardrails are what make the freedom
affordable; they are not bureaucracy attached to it.

**For per-service choice (3):** the ecosystem argument is decisive on two services and irrelevant
on three, which is precisely the shape a per-service decision fits. The contracts that would carry
the cost of polyglot already exist for other reasons — one REST contract per boundary, gateway and
internal alike — so the marginal cost is generating clients rather than inventing an integration
style. It is thinner cover than it looks: a single wire format with no schema in it means the
OpenAPI document is doing all the work, and rule 1 is load-bearing rather than tidy. And it makes ADR-001's team-structure argument
true: each member works and learns where they are strongest, which for a course project is part of
the point rather than a concession.

### Implications

**What this costs us, accepted with open eyes:**

- **Cross-cutting code is written once per language, not once.** Authentication middleware,
  structured logging, tracing setup, error mapping, retry and backoff, and workspace-identity
  propagation all get a second implementation. Some of it will diverge, and the divergence will
  be discovered in integration rather than in review.
- **The bus factor gets worse, and the mitigation is mandatory rather than advisable.** A
  teammate can read unfamiliar code in a language they know; they cannot debug an unfamiliar
  framework in a language they have never written, under demo pressure. Rule 5 exists because of
  this decision.
- **Build, CI and images multiply.** A pipeline, a dependency manager and a base image per
  language, each with its own security updates.
- **Role enforcement is now implemented more than once.** ADR-001 warns that enforcement cannot
  live only at the gateway; with several languages the check exists in several codebases and must
  be verified in each. This is the highest-risk consequence of this decision: a privilege mistake
  in one implementation is one the other implementations would have prevented.
- **Local development is heavier.** Contributors need more than one toolchain installed, or a
  container-only workflow — which slows the edit-run loop on the services they are not carrying.
- **A shared fix is no longer one change.** A bug in correlation-id propagation is fixed in every
  language that has it.

**What it buys us:**

- Each service uses the ecosystem that actually fits its work, most clearly on the two services
  where the model and the file parsing live.
- Each member works and learns where they are productive, and four people can work in parallel
  without contending on one toolchain.
- The contract discipline the guardrails impose would improve the system even if every service
  shared a language.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — its Assumptions asserted a single backend
  language; that assumption is **withdrawn and replaced by this record**. Its argument for
  decomposition from team structure depends on this decision to be more than decorative, and its
  choice of one protocol on every boundary is what keeps the generated-contract cost to a single
  toolchain per language.
- [ADR-002](ADR-002-async-screening-pipeline.md) — the screening handover and its result callback
  now cross language boundaries, so both contracts and the correlation id must be defined outside
  any one service.
- [ADR-003](ADR-003-polyglot-persistence.md) — each service already owns its own schema and no
  service reads another's tables, so differing database clients per language cost nothing extra.
- [ADR-004](ADR-004-llm-access.md) — one service holding the model credential is what allows the
  provider SDK to exist in exactly one language.
- **Still open, and directly affected:** the repository structure and where the shared OpenAPI
  definitions live; and the assignment of a primary and secondary owner to each
  service, which this decision makes a prerequisite rather than a nicety.

### Related requirements

- **NFR-17** — the scoring model or prompt changes without touching any other service; a Python
  AI Service shortens that loop.
- **NFR-01, FR-2.3** — resume parsing, the other ecosystem-driven choice.
- **NFR-10** — no accepted resume lost, which depends on retry and acknowledgement behaviour now
  implemented in more than one language.
- **FR-0.3, FR-0.4, NFR-12** — role enforcement and the access audit log, plus workspace
  isolation while it stood, now implemented in several codebases.
- A spread of communication styles is expected of us eventually; whatever protocols arrive later,
  this decision requires their contracts to be generated rather than hand-written.

### Related artifacts

- [ADR-001](ADR-001-service-decomposition.md), whose single-language assumption this record withdraws
- The repository structure and CI configuration, once implementation starts

### Related principles

- *Contracts before convenience* — a boundary is only as good as the definition that crosses it.
- *Ecosystem fit where it matters, the owner's fluency where it does not.*
- *No freedom without a corresponding rule* — the guardrails are what make per-service choice
  affordable rather than reckless.

---

## Notes

This record exists because a reviewer asked where the language had been decided, and it had not
been: it sat in ADR-001's Assumptions as a settled fact with no alternatives and no argument.
Worth noting how quietly that happened — Assumptions is exactly where a decision can hide.

This record is deliberately left *Proposed*. The team's position is that services may differ in
language, which is enough to withdraw ADR-001's assumption; it is not enough to choose. Recording
the reasoning now and the choice later is the point of having a status field — the alternative
would be either an Accepted record asserting languages nobody has agreed to, or no record at all
and the same assumption quietly reappearing somewhere else.

The strongest objection to this record is that three languages across five services is a lot for
four part-time developers, and that a single language would ship faster. That is probably true of
the first month. It was not chosen because the ecosystem cost falls on the two services that are
hardest to get right and change most often, and because a single language would have quietly
removed one of the three reasons ADR-001 gives for decomposing at all.
