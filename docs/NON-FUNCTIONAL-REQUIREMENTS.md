# Non-functional Requirements

Quality attributes the architecture must deliver, derived from the use cases in
[PROPOSAL.md](PROPOSAL.md) and the functional requirements in
[FUNCTIONAL-REQUIREMENTS.md](FUNCTIONAL-REQUIREMENTS.md).

Each requirement is grouped under the **quality attribute** it serves, stated so that it can be
verified, and paired with how that verification is done. IDs are stable once written.

> **Every figure here is an initial target, not a measured result.** They exist to give the
> architecture direction — a system that must answer in two seconds is built differently from
> one that may take a minute. They will be revised once load testing produces real numbers.

> **PDPA compliance is not stated as a single requirement**, because compliance is not
> verifiable as one statement. It is the combined effect of NFR-12, NFR-13 and NFR-14 together
> with the retention requirements FR-5.1 to FR-5.12.

---

## Performance

| ID | Requirement | Verified by |
|---|---|---|
| NFR-01 | The system shall parse a resume into a normalised candidate profile within 5 seconds at the 95th percentile. | Load test |
| NFR-02 | The system shall produce a screening result for one parsed profile within 20 seconds at the 95th percentile, and shall abandon and retry any attempt exceeding 60 seconds. | Load test |
| NFR-03 | The system shall return a ranked candidate list within 2 seconds for a job opening holding up to 1,000 screened candidates. | Load test |
| NFR-04 | The system shall acknowledge a submitted screening batch within 2 seconds, independently of the size of the batch. | Load test |
| NFR-05 | The system shall produce an interview guide within 30 seconds at the 95th percentile, indicating progress while it generates. | Load test |

Parsing and scoring are separated because they are different kinds of work: parsing is
deterministic text extraction, while scoring is a language-model call whose latency is dominated
by generated output. The 60-second bound in NFR-02 is what gives FR-2.7 ("retry failed
processing attempts") a definition of failure.

NFR-04 is architecturally decisive: it requires batch submission to be acknowledged before the
work is done, which is what makes the asynchronous design in FR-2.2 a requirement rather than a
choice.

## Scalability

| ID | Requirement | Verified by |
|---|---|---|
| NFR-06 | The system shall complete a screening batch of 100 resumes within 10 minutes when at least 4 screening workers are available. | Load test |
| NFR-07 | The system shall increase batch screening throughput proportionally as screening workers are added, up to at least 4 workers. | Comparative load test |
| NFR-08 | The system shall sustain 20 concurrent Recruiters, and batches of up to 200 resume files of up to 10 MB each, without breaching NFR-01 to NFR-06. | Load test |

NFR-06 states the worker count deliberately. At roughly 12 seconds per resume, 100 resumes take
about 20 minutes sequentially; the ten-minute target is only reachable through concurrency, so
NFR-07 is the mechanism by which NFR-06 is met rather than a separate claim.

**Scalability is the quality attribute this project demonstrates** for the course requirement.
NFR-07 is the measurement: throughput against worker count, under a fixed applicant burst.

## Availability & Reliability

| ID | Requirement | Verified by |
|---|---|---|
| NFR-09 | The system shall be available at least 99% of the time Monday to Friday, 08:00–20:00 ICT. | Monitoring |
| NFR-10 | The system shall not lose a submitted resume: every resume in an accepted batch shall reach a terminal state — scored, or flagged for manual review — even across a restart of any single service. | Fault-injection test |

NFR-10 is what makes the asynchronous design honest. Once a batch is acknowledged before it is
processed (NFR-04), the Recruiter has no way to tell a lost resume from a slow one, so the
system must guarantee it cannot lose one.

## Security

| ID | Requirement | Verified by |
|---|---|---|
| NFR-11 | The system shall encrypt candidate personal data at rest and shall transmit all data over TLS 1.2 or higher. | Configuration review |
| NFR-12 | The system shall record an audit log of every access to candidate personal data, and shall retain that log for at least one year. | Inspection |

Access control is not repeated here: it is stated as functional behaviour in FR-0.4 and FR-0.6.
Isolation *between customer companies* is not a requirement on the code at all — each company runs
its own deployment ([ADR-006](adr/ADR-006-single-tenant-deployment.md)).

## Privacy & Regulatory Compliance

| ID | Requirement | Verified by |
|---|---|---|
| NFR-13 | The system shall complete a valid candidate erasure request within 30 days of receiving it. | Audit log inspection |
| NFR-14 | The system shall exclude gender, age, marital status, religion, nationality and photograph from any input used to compute a match score. | Code and prompt review; paired-resume test |

NFR-14 is testable by construction: two resumes identical but for a protected attribute must
produce the same score.

## Transparency

| ID | Requirement | Verified by |
|---|---|---|
| NFR-15 | The system shall present, for every match score, the per-criterion evidence that produced it, such that a Recruiter can verify the result without reading the resume. | Inspection |

This rules out any scorer that cannot show its working, and requires per-criterion evidence to
be persisted alongside the score rather than regenerated on demand.

## Usability & Localisation

| ID | Requirement | Verified by |
|---|---|---|
| NFR-16 | The system shall provide the user interface in Thai and English, usable on desktop and tablet in current versions of Chrome, Safari and Edge. | Manual test |

## Modifiability

| ID | Requirement | Verified by |
|---|---|---|
| NFR-17 | The system shall allow the scoring model or prompt to be changed without modifying or redeploying any service other than the one that performs scoring. | Design review |

Scoring behaviour is expected to change far more often than anything else in the system. This
requirement keeps that rate of change contained within one service boundary.

---

## Architecturally significant requirements

Each of the following should be answered by an ADR.

| Requirements | Question it forces |
|---|---|
| NFR-02, NFR-06, NFR-08 | One model call per resume is slow and costly at volume, and a provider's rate limit may cap concurrency before the worker count does. Does screening need a tiered pipeline — a cheap deterministic filter first, the model only on the survivors? |
| NFR-13, NFR-14 | Resumes are personal data under the PDPA. May they be sent to a third-party model provider, or must the model be self-hosted? |
| NFR-07, NFR-10 | What holds a resume between acceptance and its score, how workers scale, and what the delivery guarantee is. Where is the terminal state recorded? |
| NFR-15, NFR-17 | An explainable score must persist its per-criterion evidence, and the scorer must sit behind a boundary that lets it change alone. Where is that boundary drawn? |
| FR-5.5 | Erasure must reach every service holding the data. Which component owns that cascade — orchestration or choreography — and how does it stay correct as services are added? |
