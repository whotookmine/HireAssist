# Functional Requirements

Derived from the use cases in [PROPOSAL.md](PROPOSAL.md). Each requirement is a single
verifiable statement of *what* the system does, carrying no design or technology choice.

**Numbering** is `FR-<use case>.<n>` so that every requirement traces to the use case it serves,
and so that adding one does not renumber the rest. **IDs are stable once written and never
reused**, so a withdrawn requirement leaves a gap in the sequence rather than causing a
renumbering. This document states what the system does today; why a requirement was withdrawn is
recorded in `../CHANGELOG.md` and, where a decision caused it, in the ADR that made it.

**One exception has already been taken.** On 2026-09-12, while nothing had been submitted, every
block was compacted so that no gaps remained — withdrawals in UC-0, UC-4 and UC-5 had left the
sequence reading 0.1–0.2, 0.4, 0.6 · 4.1–4.4, 4.6–4.7 · 5.1–5.2, 5.5–5.7, 5.9–5.11. Every
reference was rewritten in the same change. The stability rule applies from that date onward: a
requirement withdrawn from here on leaves a gap and the gap stays.

These are referenced from the ADRs and from the Service–Operations–Collaborators table.

**Terms** — *Candidate Profile*, *Screening Batch*, *Talent Pool*, *Retention Policy* and the
rest are defined in the glossary in [CONTEXT.md](CONTEXT.md). A Candidate Profile is the
normalised record parsed from a resume: contact details, work history, skills and education.

The deferred use case **D-1 (talent-pool re-matching)** has no requirements here; it is out of
scope for this term project.

**One deployment serves one customer company** ([ADR-006](adr/ADR-006-single-tenant-deployment.md)),
so no requirement below mentions a tenant, and isolation between companies is not something the
code enforces.

---

## UC-0 — Sign in

| ID | Requirement |
|---|---|
| FR-0.1 | The system shall authenticate a Guest by email address and password, and issue a session token identifying the user and their role, under which the user acts as a Recruiter or an Admin. |
| FR-0.2 | The system shall reject any request presenting a missing, invalid, or expired session token. |
| FR-0.3 | The system shall support two roles, Admin and Recruiter, where Admin holds all Recruiter permissions in addition to its own. |
| FR-0.4 | The system shall reject any request requiring a role higher than the requester's assigned role, and record the rejected attempt. |

## UC-1 — Create a job opening from natural-language requirements

| ID | Requirement |
|---|---|
| FR-1.1 | The system shall accept a free-text role description in Thai or English as the input to job opening creation. |
| FR-1.2 | The system shall derive from that description a proposed job title and a set of screening criteria, each classified as must-have or nice-to-have and assigned a weight. |
| FR-1.3 | The system shall present the derived proposal to the Recruiter for review before the job opening is created. |
| FR-1.4 | The system shall allow the Recruiter to add, edit or remove any criterion and change its classification and weight before confirmation. |
| FR-1.5 | The system shall identify the parts of a description it could not interpret, and shall allow criteria to be entered manually either to fill those gaps or to replace the derived proposal entirely. |
| FR-1.6 | The system shall record an expected time-to-fill for each job opening. |
| FR-1.7 | The system shall store a confirmed job opening in the open state, recording the date it was opened and retaining the original free-text description. |
| FR-1.8 | The system shall allow a Recruiter or Admin to pause and resume a job opening, and to close it with a recorded outcome of either *filled* or *cancelled*. |

## UC-2 — Batch-screen resumes against a job opening

| ID | Requirement |
|---|---|
| FR-2.1 | The system shall accept a batch of resume files in PDF format submitted against one open job opening. |
| FR-2.2 | The system shall acknowledge a submitted batch with a batch identifier without waiting for processing of that batch to complete. |
| FR-2.3 | The system shall parse each resume into a normalised candidate profile. |
| FR-2.4 | The system shall process each resume in a batch independently, such that failure of one resume does not prevent processing of the others. |
| FR-2.5 | The system shall produce for each resume a screening result comprising a score against the job opening's weighted criteria, a must-have check stating which mandatory criteria are met and which are not, and a written justification citing evidence drawn from that resume. Where any must-have criterion is not met, the system shall mark the candidate not qualified and record which criteria caused it. |
| FR-2.6 | The system shall present screening results ranked by score, updating the ranking as further results become available, and shall allow the list to be filtered by minimum score and by individual criterion. |
| FR-2.7 | The system shall retry failed processing attempts and shall flag any resume it cannot parse or score as requiring manual review, excluding it from the ranking rather than assigning it a score. |
| FR-2.8 | The system shall allow the Recruiter to override a screening score, and shall retain the original score alongside the override. |
| FR-2.9 | The system shall allow the Recruiter to shortlist a screened candidate. A candidate who is neither shortlisted nor marked not qualified remains undecided; the system shall not require a rejection to be recorded by hand. |
| FR-2.10 | The system shall not transmit a rejection reason to the candidate it concerns. |
| FR-2.11 | The system shall add every screened candidate to the talent pool, recording the date their personal data was collected. |
| FR-2.12 | The system shall notify the Recruiter when a screening batch has finished processing. |

## UC-3 — Generate candidate-specific interview questions

| ID | Requirement |
|---|---|
| FR-3.1 | The system shall generate an interview guide for a candidate shortlisted against a job opening, derived from both the candidate's profile and the job opening's criteria, and shall retain the guide so that it can be retrieved again before the interview. |
| FR-3.2 | The system shall include questions probing claims made in the resume, questions addressing gaps against must-have criteria, and questions covering role-specific depth. |
| FR-3.3 | The system shall tag each generated question with the criterion it tests and the resume evidence it was derived from. |
| FR-3.4 | The system shall allow the Recruiter to add, edit, remove or regenerate questions, including regenerating a guide with a stated emphasis. |
| FR-3.5 | The system shall allow interview notes to be recorded against each question in a guide. |

## UC-4 — Monitor hiring pipeline and stale positions

| ID | Requirement |
|---|---|
| FR-4.1 | The system shall present, for each open job opening, the number of candidates at each pipeline stage (applied, screened, shortlisted), days open against its expected time-to-fill, and whether the opening is currently flagged. |
| FR-4.2 | The system shall evaluate every open job opening against its staleness rule on a recurring schedule, independently of whether any user views the dashboard. |
| FR-4.3 | The system shall flag a job opening that has been open longer than its expected time-to-fill. |
| FR-4.4 | The system shall notify the Recruiter responsible for a job opening when that opening is flagged. |
| FR-4.5 | The system shall clear a flag when the job opening is closed, or when the flagged condition is resolved. |
| FR-4.6 | The system shall exclude a paused job opening from staleness evaluation. |

## UC-5 — Enforce candidate data retention

| ID | Requirement |
|---|---|
| FR-5.1 | The system shall allow an Admin to configure a retention period, an anchor date of either the date the data was collected or the date of last activity, and an expiry action of deletion or anonymisation. |
| FR-5.2 | The system shall evaluate candidate records against the retention policy on a recurring schedule, flagging those entering the warning window before expiry and notifying the responsible Recruiter with the affected candidates and their expiry date. |
| FR-5.3 | The system shall, on expiry, delete or anonymise everywhere it is held the candidate's personal data — the candidate profile, the stored resume file, screening results and their justification text, and generated interview guides — according to the configured expiry action, as a single operation that leaves no partially erased record in place. |
| FR-5.4 | The system shall, when anonymising, remove all identifying data while preserving the aggregate counts used for pipeline metrics, such that no later processing can surface the person. |
| FR-5.5 | The system shall write an audit entry for each erasure recording what was erased, when, and under which retention policy, excluding from that entry the candidate's name, contact details, resume file and its contents, candidate profile field values, screening scores and justification text; each entry shall reference a candidate only by a pseudonymous internal identifier that cannot be resolved to a person once erasure is complete. |
| FR-5.6 | The system shall hold expiry for a candidate in an active hiring process and flag the record for the Recruiter to decide. |
| FR-5.7 | The system shall allow an Admin to execute an erasure for a named candidate immediately, rather than waiting for the next scheduled evaluation. |
| FR-5.8 | The system shall, where erasure of stored data fails, retain the record, log the failure, retry, and report the record as deletion pending rather than as erased. |

## UC-6 — Manage members and roles

| ID | Requirement |
|---|---|
| FR-6.1 | The system shall allow an Admin to create a member account with an initial password, remove a member, and change a member's role. |

---

**44 requirements** — UC-0: 4 · UC-1: 8 · UC-2: 12 · UC-3: 5 · UC-4: 6 · UC-5: 8 · UC-6: 1.
