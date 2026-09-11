# Changelog

A running summary of what changed in this project's documentation and why.

One entry per working session, newest first, headed `YYYY-MM-DD HH:MM`. Entries record
**decisions and their reasons**, not file diffs — Git already has the diffs.

**Keep entries short.** A line or two per change: what changed, and why in a clause. If an
explanation needs a paragraph, it belongs in the ADR or the document itself, and the entry should
point there instead of repeating it. A changelog nobody scrolls through is a changelog nobody
reads.

Each entry is written once and then left alone. Later work gets a new entry — an entry describes
what a session decided, and rewriting it destroys the record of when and why something changed.

---

## 2026-09-12 00:16 — Changelog hook rewritten in POSIX sh

- The hook was Python; `python3` is not guaranteed on a teammate's machine, a shell is. Rewritten
  in POSIX `sh` with no interpreter and no `jq` dependency.
- Detection changed from "modified in the last 20 seconds" to "modified since the last check",
  using `find -newer` against a marker file — POSIX, and it drops the arbitrary time window while
  keeping the same behaviour: fires on a real change, silent on a read, reports a change once,
  silent when CHANGELOG.md changed too.
- Tested: first run, no change, read-only, single change, repeat, docs-plus-changelog, truncation
  at eight files, JSON validity, and a run under `dash` to confirm no bashisms.

---

## 2026-09-12 00:04 — README stops listing individual ADRs

- The README carried a table of every ADR, which had already drifted once — six records, a
  description of four. A README that mirrors another index is a second copy that goes stale. It now
  just says where the records live.

---

## 2026-09-11 23:52 — ADRs made self-contained and shorter; rejection derived, not recorded

- **ADRs no longer reference any project document.** They are read on their own — in review, by a
  new joiner, lifted out of the repository — so a record that depends on the proposal or the
  requirements file to make sense is broken the moment it travels. Facts are now stated in place;
  requirement ids stay for traceability but say what they mean. Rule added to the template.
- **All six compacted**, roughly 15% shorter with no loss of meaning: restatement cut, sentences
  tightened, the "one to two pages" guidance made explicit in the template.
- **Rejection is derived, not recorded.** A candidate missing a must-have is marked not qualified
  by screening; everyone else simply goes un-shortlisted. The Recruiter shortlists and can override
  a bad mark, so the reason someone did not advance is the same recorded justification for
  everyone rather than a free-typed note that varies with the reviewer.

**Outstanding:** service ownership unassigned (blocks ADR-005); erasure cascade and deployment
pipeline unwritten.

---

## 2026-09-11 23:34 — Retention extension removed (FR-5.4)

- **A Recruiter can no longer extend a candidate's retention.** Under the PDPA data may be kept
  only as long as the purpose it was collected for requires; keeping someone for a *future*
  opportunity is a new purpose needing a new basis, so a unilateral extension is exactly what the
  rule exists to prevent. Data now ages out on schedule, full stop.
- The requirement was originally *renew consent*, which was lawful. It became *extend retention*
  when consent storage was dropped earlier today — that edit quietly turned a defensible act into
  an indefensible one.
- **FR-5.9 remains the only way expiry is deferred**, and it is justified: it holds expiry for a
  candidate still in an active hiring process, where the original purpose has not ended.
- UC-5's warning window now says what it is for — act on anyone still in play before the date
  passes. 44 requirements; UC-5 is down to 8.

**Outstanding:** whether rejection is recorded by the Recruiter or derived entirely from screening
(FR-2.9) is still undecided.

---

## 2026-09-11 23:15 — Tenancy removed, ADRs 005 and 006, requirements tightened

- **ADR-006: one deployment per customer.** The workspace concept was never decided — it arrived
  with the multi-tenant framing and spread unchallenged. Single-tenant means a cross-customer leak
  cannot happen, at the cost of operating N instances. FR-0.3 withdrawn; UC-0 is now *Sign in* and
  UC-6 *Manage members and roles*; diagram re-rendered from source.
- **ADR-005: language per service — Proposed, not Accepted.** ADR-001 had asserted Go in its
  Assumptions. That assumption is struck; the languages themselves wait on service ownership.
- **ADRs live only in `adr/`.** The proposal's 144-line summary section is now a linked index, so
  a decision is stated once and cannot drift.
- **ADR-001's argument no longer cites the course requirement.** It now rests on fault isolation
  of the model path and the opposite resource profiles of parsing and scoring.
- **Six actor gaps closed.** No self-service sign-up (accounts are provisioned); erasure requests
  arrive by email and an Admin executes them; consent is obtained before upload and no longer
  stored; the *interviewing* stage is removed; closing a job records *filled* or *cancelled*.
- **Requirements tightened to 45.** UC-5 lost three restatements and UC-4 one; withdrawn
  requirements are now deleted rather than struck through, leaving gaps in the ids.
- **ADRs made time-ordered.** An ADR may only cite one that existed when it was written; later
  changes go in its Status as a dated amendment. Eleven forward references removed.
- Integrity pass: all FR, NFR, UC and link references resolve; counts agree everywhere.

**Outstanding:** service ownership unassigned (blocks ADR-005); erasure cascade and deployment
pipeline unwritten.

---

## 2026-09-11 — ADRs reconciled with the UC-0 / UC-6 split

Two strands of work ran in parallel today and were merged: the four ADRs (_First four ADRs
recorded_) and the use case work (_Use case diagram redrawn in UML, as code_, _UC-0 split_, and
_Invitation-email «extend» removed_). Each was written without knowing the other's result, so
each left statements the other has since made untrue. This entry reconciles them. The earlier
entries stay as written.

**The ADRs now credit memberships and roles to UC-6, not UC-0.** They were written against the
old UC-0, which bundled signing in with managing who belongs to a workspace. After the split,
UC-0 covers only authentication. So ADR-001's Identity & Workspace Service now serves UC-0 _and_
UC-6. ADR-003 lists workspaces and users under UC-0, and memberships and roles under UC-6. The ADR
summary in `PROPOSAL.md` says the same. ADR-003's "nothing in UC-0 through UC-5 needs to search
resume text" now reads UC-6, since there is one more active use case.

None of this changes a decision. Both ADRs already put identity, workspaces, memberships and roles
in one service, stored in PostgreSQL. UC-6 changes only which use case that behaviour traces to.
So the ADRs were corrected where they stand rather than superseded. Superseding is for a decision
that changed, and these were fixes to their traceability.

**Superseded statements in the earlier entries.** Two statements in earlier entries are no longer
true, and this entry supersedes them:

- _Use case diagram redrawn in UML, as code_ and _Invitation-email «extend» removed_ both close
  with "No ADRs are written yet." Four are now written.
- _First four ADRs recorded_ lists "the use case diagram needs redrawing in proper UML" as
  outstanding. The diagram is now redrawn in PlantUML.

The status line in `PROPOSAL.md` had carried the second statement forward. It now lists the
diagram as complete.

**The merge itself.** Git flagged only `CHANGELOG.md`, where both strands had added entries at
the same place. Both sets were kept unchanged. The use case entries sit above the ADR entry
because they reached the shared `main` later. The ADR and proposal fixes above are the conflicts
Git could not see: text that merged cleanly but no longer agreed.

**Still outstanding.** The two UC-6 questions in `CONTEXT.md`, about how an invited person sets a
password and whether the last Admin can be removed. When they are settled, the open decision on
the UC-0 session and token mechanism in `adr/INDEX.md` may need to cover the invitation flow too.
Also still open: the FR/NFR lists to be inlined into `PROPOSAL.md` before submission, the
load-test plan, the risk matrix, and the seven candidate decisions in `adr/INDEX.md`.

---

## 2026-09-11 — Invitation-email «extend» removed from the use case diagram

**_Draft Interview Invitation Email_ is gone from the diagram,** along with the extension point
_candidate shortlisted_ on UC-2 and the condition note on the arrow. This closes the gap left open
by the _Use case diagram redrawn in UML, as code_ entry below. That entry offered two ways out:
write the behaviour into UC-2 and add a requirement for it, or take the «extend» off the diagram.
We took the second.

The reason is consistency. The assignment asks for a diagram that agrees with the use cases, and
this was the only element with nothing behind it. UC-2's flows never mentioned an invitation, and no
functional requirement backed one; it survived only from the original idea list. Keeping it would have shown graders a feature the proposal does not deliver.
Adding it properly would have grown UC-2's scope for behaviour that does nothing for the core
problem of screening capacity.

The diagram now carries no «include» or «extend» relationships at all. The rationale in
`PROPOSAL.md` says so and explains why, as `docs/course/ASSIGNMENT.md` asks: they are not
required, and should not be added only to make a diagram look more complex. The provenance table in `CONTEXT.md` records
that the original _first draft email for interview meeting_ idea was dropped.

Removing the extension also cleaned up the rendered layout. The System Scheduler and Candidate
associations no longer cross the boundary at odd angles.

**Still outstanding.** The two UC-6 questions from the entry below, about how an invited person
sets their password and whether the last Admin can be removed. No ADRs are written yet.

---

## 2026-09-11 — UC-0 split: Guest signs in, Admin manages access (UC-6)

**UC-0 is now _Authenticate into a workspace_, and its actor is a new Guest.** The old UC-0 bundled
two things with different actors and different goals: every user signs in every session, while
only an Admin, occasionally, decides who belongs to the workspace. Keeping them together hid the
Admin's distinct responsibility inside a use case everyone takes part in, and made Recruiter and
Admin actors of their own sign-in, which is circular.

**The Guest establishes a convention the documents now rely on:** _Recruiter_ and _Admin_ always
mean a signed-in user. That is what lets UC-1 to UC-6 name those actors without repeating the
sign-in, and it is why UC-0 is drawn once, against the Guest, rather than as «include» arrows from
every other use case. No generalisation is drawn between Guest and Recruiter or Admin, since
signing in changes the role a person plays and a Recruiter is not a kind of Guest. The term is in
the glossary. UC-1 and UC-2 still list authentication as a precondition. Under the convention
that is redundant but not wrong, so it was left alone, and UC-0 says the other use cases _need not_
repeat the sign-in rather than that they never do.

**Managing access became UC-6, _Manage workspace access_, with Admin as its actor.** It took the
next free number rather than slotting in as UC-1. Inserting it would have shifted UC-1 to UC-5
and, through the `FR-<use case>.<n>` scheme, renumbered 44 requirements whose IDs are meant to be
stable, as well as every cross-reference to them. `CLAUDE.md` now states the rule so the next new
use case follows it. The cost is that the ID order no longer follows the order a user meets the
features.

**Requirements.** FR-0.5 (invite, remove, change role) was struck and moved unchanged to FR-6.1,
following the rule that a withdrawn ID is struck, never reused. FR-0.1 now authenticates a Guest
instead of "an Admin or Recruiter". FR-0.2, FR-0.3, FR-0.4 and FR-0.6 stay with UC-0: they govern
the session token and what every request under it may do, which is not specific to managing
members. The total is still 50.

**Diagram.** Guest was added with its association to UC-0. Recruiter no longer associates with UC-0.
Admin gained an association with UC-6. Admin still specialises Recruiter, now inheriting UC-1 to
UC-4. Guest and UC-0 sit at the bottom of the rendered image, because they are not connected to
the rest of the graph and Graphviz lays them out last. Several hidden-link layouts were tried to
move them to the top, and each one scrambled the use case order or crossed lines, so they were
dropped. Position carries no meaning in a UML use case diagram.

Nothing else in the use cases changed, as the feedback asked.

**Still outstanding.** Two gaps that splitting UC-6 out made visible, both now open questions in
`CONTEXT.md`. First, nothing covers how an invited person sets the password UC-0 checks. Second,
nothing stops the last Admin being removed or demoted, which would leave a workspace that no one
can administer. UC-6 has a single requirement, so these may justify more. The invitation-email
«extend» gap from the entry below is still open.

---

## 2026-09-11 — Use case diagram redrawn in UML, as code

**The use case diagram now lives in its own file as PlantUML**, at
`docs/diagrams/use-case-diagram.puml`, with an SVG rendered from it. `PROPOSAL.md` embeds the SVG
and links to the source. The inline Mermaid block was removed rather than kept beside it: two
copies of one diagram would drift, which is the same failure the PROPOSAL/CONTEXT split exists
to prevent. The relationships rationale and the notes stay in `PROPOSAL.md`, because they are
settled content a grader reads next to the picture.

This closes the item both earlier entries left outstanding — redraw in proper UML for submission.

**Why PlantUML and not Mermaid.** Mermaid has no use case diagram type; the draft was a
flowchart with rounded boxes standing in for use cases. It could not draw stick-figure actors,
actor generalisation, or an extension point, so it approximated UML rather than using it.
PlantUML has native use case syntax and draws all of those. It is still diagram-as-code, so the
diagram is diffable in Git and reviewed like any other document change.

The cost is rendering. GitHub renders Mermaid inline but not PlantUML, so the SVG must be
committed alongside the source and re-rendered after every edit — a stale image is a stale
diagram, and nothing enforces it. Step 4 of the use-case checklist in `CLAUDE.md` now says to
re-render and commit both files together. Anyone editing the diagram needs PlantUML locally
(`brew install plantuml`, or the jar with Java, or an editor extension).

**What the UML version shows that the draft could not:**

- **Admin is drawn as a specialisation of Recruiter** (actor generalisation), so it inherits the
  Recruiter associations UC-0 to UC-4, and only its own association to UC-5 is drawn. The draft's
  note described this relationship but the picture never showed it, and the note called Admin a
  _generalisation_ of Recruiter, which is the wrong way round — corrected to _specialises_.
- **UC-2 declares the extension point** _candidate shortlisted_, and the «extend» arrow from
  _Draft Interview Invitation Email_ carries its condition — the Recruiter chooses to invite a
  shortlisted candidate. The draft had the arrow but neither the point nor the condition.

No use case or actor was added or removed.

**Still outstanding.** _Draft Interview Invitation Email_ appears only in the diagram and its
rationale — UC-2's flows never mention it, and no functional requirement backs it. Either UC-2
gains an alternate flow and FR-2.x a requirement, or the «extend» comes off the diagram. No ADRs
are written yet.

---

## 2026-09-11 — First four ADRs recorded

*(Later the same day, on top of the* Requirements *entry below.)*

The architecture decisions that had been listed as *candidates* since the repository was created
are now decided and written up. Four records, deliberately read as a sequence: **ADR-001** draws
the service boundaries, **ADR-002** fills in the busiest one, **ADR-003** says what each side
stores, and **ADR-004** fills in the external dependency the other three are built to survive.
Every record maps its decision to the `FR-` and `NFR-` identifiers the *Requirements* entry
introduced, so traceability runs requirement → decision rather than decision → vague intent.

**Why four and not three.** The course minimum is three. A fourth was written because the LLM
decision could not honestly be folded into any of the others — it carries the PDPA argument, and
leaving it implicit would have meant a system whose central dependency was never chosen on the
record. Six were considered (adding tech stack and authentication) and cut: the team is four
students, and two thin ADRs would have diluted four substantial ones. Those remain candidates in
`docs/adr/INDEX.md`.

**ADR-001 — five services behind a gateway, discovery via Kubernetes.** Boundaries drawn by
capability, following three properties that genuinely differ: who triggers the work (a human, a
queued message, or the clock), how long it may take, and what data it owns. Rejected: the modular
monolith, one service per use case, and decomposition by technical layer. The monolith argument is
recorded honestly in the ADR's Notes — with no course constraint we would likely have started with
two deployables, and what survives the constraint is the *shape* of the split, not the count.
Consul was rejected because Kubernetes already derives the same information from readiness probes
it is running anyway; the cost is that all four members now have to learn Kubernetes, which is
recorded as the largest schedule risk the decision creates. Go is the backend language.

The service that owns UC-4 and UC-5 is named **Compliance & Insights**. The *Requirements* entry
below referred to it in passing as *Insights & Notifications*; that name was provisional, and
ADR-001 does not adopt it, because retention ownership is the service's defining responsibility
and notifications are not a service — each event's owner emits its own (FR-2.12 from Hiring,
FR-4.4 and FR-5.3 from Compliance & Insights).

**ADR-002 — one queued message per resume, on RabbitMQ.** The decision that mattered was not the
broker but the *unit of work*: per-resume rather than per-batch, so the unit of failure is the
same size as the failure. With one message per batch, a single corrupt file poisons two hundred
resumes and a retry re-pays every LLM call already made. Kafka was rejected on head-of-line
blocking — one slow resume would stall its partition, which is precisely the failure this design
exists to prevent — and a PostgreSQL job table was rejected because it would put batch contention
on the same database serving recruiter traffic, undoing ADR-001's boundary from below.

**The demonstrated quality attribute was reconsidered, and Scalability stands.** ADR-002 was
first drafted with *Reliability* as the attribute the project demonstrates — the design's most
distinctive property is that no dependency failure loses work, and UC-2's alternate flow 4a
already promised it. On rebasing onto the *Requirements* entry, which had independently chosen
Scalability (NFR-07) that morning, we kept Scalability rather than overturn a teammate's merged
decision. The reasons it was the right call, not merely the polite one: NFR-07 is easier to show
live — add workers, watch the throughput line — while a fault-injection demo is harder to make
legible in ten minutes; and nothing in the design changes either way, because the per-resume
queue is what delivers both. The reliability claim survives intact as NFR-10 and its
fault-injection test, and ADR-002 now argues that the second test is what keeps the first honest:
throughput bought by dropping work is not throughput. The reconsideration is recorded in ADR-002's
Notes.

**ADR-003 — PostgreSQL as system of record, MongoDB for AI-derived documents.** The course
requires two database types, and the ADR says plainly that the requirement forced the question —
then gives an architectural reason that stands without it. Keeping identifying material physically
separate from the counts the UC-4 dashboard is built on turns UC-5's *anonymise* action (FR-5.6)
into "drop the documents, keep the rows": coarse and provable, rather than a field-by-field
rewrite one missed column away from a compliance failure. PostgreSQL-with-`jsonb` is recorded as a
genuinely strong rejected position rather than a straw man. The accepted cost is that no
transaction spans the two stores, which is now named as the architectural cause of the *deletion
pending* state FR-5.11 already required — a case where writing the ADR explained something the
requirements had anticipated by instinct.

**ADR-004 — all model access through one AI Service, managed API only, no fallback model.** The
provider's training clause is treated as non-negotiable rather than as a preference: a provider
that trains on submitted data absorbs every resume into a weight we cannot delete from, which
would make UC-5's erasure promise false from the first batch screened. Self-hosting an open-weight
model was the position the team was most reluctant to reject — it is the strongest privacy answer —
and it lost on hardware we do not have and on materially weaker Thai-language quality, which would
have converted a privacy win into a fairness loss in the one place the product must not fail. The
AI Service is also where NFR-14's protected attributes are stripped before any prompt, so no
caller can forget to, and it is NFR-17 made structural.

**The hybrid was considered and rejected, which is worth recording because it looks like the
obvious answer.** With NFR-10 promising that no accepted resume is lost, a local fallback model
seems free. It is not: a fallback makes a candidate's score depend on which model happened to be
healthy, so two candidates in the same batch could be ranked against each other on incomparable
numbers — indefensible in a system whose proposition is a justified, comparable score. So NFR-10
is read for exactly what it says: *every resume reaches a terminal state*, not *the AI is always
available*. ADR-002 and ADR-004 are therefore a pair, and each names the other — running without a
fallback is only acceptable because no queued work is lost. The consequence is stated in both
records and belongs in the risk matrix: during a provider outage, screening produces no scores at
all.

**Aligned with the *Requirements* entry while rebasing.** ADR-002 had been drafted against the
earlier PDF-or-DOCX intake and treated an unsupported format as a permanent processing failure; it
now follows the PDF-only decision — a non-PDF file is rejected at upload (FR-2.1) and never reaches
the queue, so the permanent-failure class is only a corrupt file or a scanned image with no text
layer. NFR-02's 60-second abandon-and-retry bound is adopted as the definition of a transient
failure, and NFR-04's two-second acknowledgement as the bound on the accept path.

**Documents brought into line.** `PROPOSAL.md` gained a full *ADRs* section (summary plus the
connective tissue between the four) and moved to Draft 3, with its status line finally reflecting
that the requirements are done. `adr/INDEX.md` has the four rows, names which five of the eight
candidates from the *Requirements* entry are now answered and by which record, and lists seven
that remain — the three unanswered ones plus four the ADRs themselves surfaced: front-end
framework, the UC-0 session/token mechanism and how workspace identity travels on internal calls,
the scheduler shared by UC-4 and UC-5, and the repository structure. `CONTEXT.md` had four
decided items removed — settled things move out of CONTEXT rather than being copied — and gained
four new open *questions*, each load-bearing for a decision already made: the provider's current
data-processing terms, Thai-language scoring quality, whether recruiters accept a
progressively-filling batch, and whether the team can stand up a Kubernetes cluster early.
`README.md` lists the four records, and `course/ASSIGNMENT.md`'s status table now reads *Done* for
the ADR row.

**Still outstanding:** the use case diagram needs redrawing in proper UML for submission; the
FR/NFR lists must be inlined back into `PROPOSAL.md` before submission, as the *Requirements* entry
notes; the load-test plan and the risk matrix, both of which ADR-002 and ADR-004 now feed; and the
seven candidate decisions above.

---

## 2026-09-11 — Requirements: reconciled, extracted, and rebuilt around quality attributes

**UC-2 accepts PDF only.** Settled while reviewing draft functional requirements: DOCX was
dropped from the accepted formats, so the use case now states PDF in its description, requires
the Recruiter to upload PDFs, and has the System reject non-PDF files at validation.

Unsupported formats moved out of the *unreadable file* alternate flow, since they are now
rejected up front rather than failing during parsing. That flow keeps the two cases that
survive validation — a corrupt file, and a scanned image with no extractable text layer — so
the scanned-resume problem is still handled without needing a second file format.

**Functional requirements written up and moved out of the proposal.** They now live in
`docs/FUNCTIONAL-REQUIREMENTS.md` — 50 requirements numbered `FR-<use case>.<n>` so each traces
to the use case it serves and so adding one never renumbers the rest. `PROPOSAL.md` keeps a
pointer and a per-use-case summary table. The deferred use case D-1 has no requirements.

**Two independent drafts were reconciled.** A teammate had filled in flat-numbered `FR-01`–`FR-33`
directly in `PROPOSAL.md` while a per-use-case set was being reviewed here. The reviewed set was
taken as the base, and six requirements from the other draft were absorbed because they covered
behaviour the base set missed — five merged into existing requirements rather than added
alongside them:

- *not qualified* outcome when a must-have criterion is unmet → merged into FR-2.5
- filtering the ranked list by minimum score and by criterion → merged into FR-2.6
- collection date and lawful basis for processing → merged into FR-2.11
- retaining a generated interview guide for later retrieval → merged into FR-3.1
- erasure across every place the data is held, as a single operation leaving no partial record
  → merged into FR-5.5
- batch-completion notification → added as FR-2.12, the only one with no natural home

Two requirements from that draft were deliberately not carried over: a snapshot of the criteria
in effect when a job opening was saved, and an explicit 0–100 score range.

**Conflicts resolved in favour of the reviewed set:** PDF-only intake (the other draft allowed
DOCX), and pause/resume/close lifecycle rather than close alone.

**Staleness is measured per job opening, not per workspace.** The two drafts disagreed: a
per-job *expected time-to-fill* set by the Recruiter at creation (FR-1.6), versus a single
workspace-wide threshold in days configured by an Admin. We chose per job.

The deciding argument is that one global threshold cannot serve roles with genuinely different
hiring horizons — set low enough to catch a stalling support role and it floods alerts for a
niche senior role; set high enough for the niche role and easy roles rot unnoticed, which is
the exact problem UC-4 exists to solve. Per job also keeps the rule local: the threshold is a
field on the job opening, so Insights & Notifications can evaluate staleness from the
`JobOpeningCreated` event it already consumes, instead of reading workspace configuration owned
by another service.

The cost accepted is that the Recruiter must supply an estimate on every job opening, and a
Recruiter who does not know the answer will guess. A workspace default with a per-job override
was considered and rejected as more machinery than this project needs.

This surfaced a gap in both drafts: nothing recorded *when* an opening was opened, which
days-open depends on. Merged into FR-1.7.

**Use cases and requirements reconciled.** A full cross-check was run in both directions — every
use case behaviour against a requirement, and every requirement against use case prose — and
thirteen discrepancies were fixed in `PROPOSAL.md`. None changed what the system does; they
removed statements the requirements no longer backed, or described behaviour the requirements
had gained.

Behaviour the use cases promised but no requirement backed, now removed from the prose:

- rate limiting of failed sign-ins (UC-0)
- exporting or sharing an interview guide (UC-3) — the guide is viewed in the application
- the "resume-specific coverage is limited" fallback for a thin resume (UC-3)
- stage-to-stage conversion on the dashboard (UC-4)
- staleness alerts for an unreviewed batch and for an undecided shortlisted candidate (UC-4),
  leaving expected time-to-fill as the single rule
- the empty-state dashboard when no positions are open (UC-4)

Behaviour the requirements gained but the use cases never described, now written into the prose:

- pause, resume and close a job opening, and the date an opening was opened (UC-1). UC-4's
  alternate flow already assumed pausing existed, so this closed a dangling reference.
- marking a candidate *not qualified* when a must-have criterion is unmet, and filtering the
  ranked list by minimum score or by criterion (UC-2)
- recording the collection date and lawful basis alongside consent status (UC-2)
- notifying the Recruiter when a batch finishes processing (UC-2)
- retaining a generated interview guide for retrieval before the interview (UC-3)
- erasure reaching profile, resume file, screening results, justification text and interview
  guides as one operation leaving no partial record (UC-5)

UC-4's description was also rewritten to explain *why* the threshold is per opening rather than
per workspace, so the decision is visible to a reader who never opens the changelog.

**Non-functional requirements rewritten around quality attributes**, and moved to
`docs/NON-FUNCTIONAL-REQUIREMENTS.md`. The teammate draft had 23 grouped by loose category
(Operational, Performance, Security, Cultural and Legal, Usability); the replacement has 17
grouped by the quality attribute each serves, every one paired with how it is verified. The
brief was to keep the set small and give the architecture direction rather than aim for
completeness.

Cut, with reasons:

- *Duplicates of functional requirements* — role-based access control, workspace isolation, and
  the rule that a score is never disclosed to the candidate. All three are already FR-0.4,
  FR-0.3 and FR-2.10. A requirement stated twice eventually gets stated two different ways.
- *Not a requirement* — "shall be deployed on cloud infrastructure" is an architectural
  decision and belongs in an ADR.
- *Unverifiable* — "shall comply with the PDPA". Compliance is not testable as a statement; it
  is the combined effect of NFR-12 to NFR-14 and FR-5.1 to FR-5.12, and the document now says
  so in prose instead of pretending otherwise.
- *No architectural consequence* — the Computer Crime Act, already covered by NFR-11 and NFR-12.
- *Untestable as written* — "usable without prior training". A real goal, but with no
  measurement attached it is decoration.

Added:

- **NFR-07**, throughput rising proportionally with worker count. This is the measurement for
  the course's "demonstrate one quality attribute" requirement, and it reuses the load test
  already owed.
- **NFR-10**, no accepted resume is ever lost. Once a batch is acknowledged before processing,
  a Recruiter cannot distinguish a lost resume from a slow one, so the guarantee has to be
  explicit.
- **NFR-17**, scoring model and prompt changeable within one service boundary — the thing that
  will change most often in this system.

**Scoring latency corrected.** The first draft required parse-and-score within 5 seconds, a
figure that only makes sense for deterministic parsing. Scoring is a language-model call whose
latency is dominated by generated output — realistically 8–15 seconds, longer at the tail. The
requirement was split: parsing within 5 seconds (NFR-01), scoring within 20 seconds with a
60-second abandon-and-retry bound (NFR-02). That bound also gives FR-2.7 a definition of
"failed" that it previously lacked.

The batch figure was then checked against it rather than left to contradict it: 100 resumes at
roughly 12 seconds each is about 20 minutes sequentially, so the 10-minute target holds only
with concurrency. NFR-06 now states the worker count it assumes, which makes NFR-07 the
mechanism that delivers it instead of an unrelated claim.

**Scalability chosen as the demonstrated quality attribute**, closing an open decision in
`CONTEXT.md`. It is measurable, demonstrable in a live demo, and shares a load test with an
existing requirement.

**ADR candidates now derived from requirements.** `docs/adr/INDEX.md` lists eight, each with the
requirement that forces it, rather than the loose list of topics it held before.

**Changelog reminder automated.** A `PostToolUse` hook (`.claude/settings.json` plus
`.claude/hooks/changelog-reminder.py`) now fires whenever any file under `docs/` is modified
and injects a reminder to record the change here. Committed at project level so it applies to
the whole team, not one machine.

It decides by **file modification time**, not by inspecting the tool's arguments. The first
attempt matched the string `docs/` in the tool input, which was wrong in both directions: it
would have fired on a read such as `sed -n '1,50p' docs/PROPOSAL.md`, which mentions the path
but changes nothing, and it depended on knowing which argument of which tool carries a path.
Checking what actually changed on disk catches an edit however it was made — the Edit and Write
tools, a scripted Bash heredoc, `sed -i`, a redirect — and never fires for a read.

It stays silent when `CHANGELOG.md` was part of the same change, and remembers the last change
it reported so it does not repeat itself for the same edit.

`.gitignore` previously excluded all of `.claude/`, which would have kept the hook on one
machine. It now shares `.claude/settings.json` and `.claude/hooks/` with the team while still
ignoring `.claude/settings.local.json` and anything else personal.

**Still outstanding.** No ADRs are written yet. The use case diagram still needs redrawing in
proper UML for submission.

---

## 2026-09-10 — ADR template rebuilt, and external context brought into the repo

*(After the initial commit `b4b9a3c`.)*

**ADR template rebuilt on Jeff Tyree & Art Akerman** rather than the simpler
Context/Decision/Status/Consequences format used in the first commit. The course's ADR deck
presents several templates; we chose this one because it forces the two things the lecturer's
deliberately bad sample was missing into their own required fields — the rejected alternatives
(**Positions**) and the mapping from decision to requirement (**Related requirements**).

Three additions to the original 14 fields, documented inside the template so the deviation is
explicit:

- **Date and Deciders** header — the original carries no traceability line, and per-member Git
  contribution is graded.
- **A one-sentence Alexandrian summary** at the top. It doubles as a readiness test: if the
  sentence will not form, the decision is not clear enough to record.
- **Optional fields marked as optional.** Fourteen mandatory sections on a term project
  produces filler, and filler buries the real content. Omit rather than pad with "N/A".

Nygard's guidance on length and voice is kept — one to two pages, written as a conversation
with a future developer.

**External context brought into the repo.** `CLAUDE.md` had been pointing at
`slides/4-1-ADRs.pdf`, which lives outside the repository — a dead reference from any clone.
More seriously, the assignment brief and submission guideline existed only in email and in
class, so a future session would have had no way to know the required headings, the "at least
3 use cases" rule, the warning that terse or ambiguous use cases lose marks, or the
instruction not to add «include»/«extend» decoratively.

- `docs/course/ASSIGNMENT.md` now holds the brief and the guideline in the original Thai with
  English glosses, and a note under each clause saying what it means for our documents.
- `CLAUDE.md` gained a *Context outside this repository* section with the rule: anything that
  influences the documents must be captured inside the repo, external files are summarised
  rather than linked by path, and provenance is named so a reader does not need the source.
- The lecturer's slide decks and the syllabus stay out of the repo deliberately — they are the
  lecturer's material, and this repo holds documentation we wrote.

**Changelog policy changed.** Entries are now written once and left alone; later work gets a
new entry, even on the same day. The `2026-09-10 — Initial proposal and repository setup`
entry is frozen as the baseline covering everything up to the first commit. Previously the
rule was to append to the current day's entry, which would have meant rewriting committed
history in prose.

---

## 2026-09-10 — Initial proposal and repository setup

> **Baseline.** This entry covers everything up to the first commit. Frozen — do not edit it;
> add a new entry above instead.

**Project defined.** HireAssist — an AI-assisted hiring-support layer for companies that
receive more applications than they can screen carefully and interview well. Positioned
deliberately as an assistant layer over existing hiring channels, **not** a job board, job
application platform, or another ATS.

**Target customer defined by a ratio, not company size.** The qualifier is applications
received versus capacity to judge them properly, so a small startup and a mid-size company
with an outnumbered recruitment team are the same customer. Thai tech SMEs are the initial
beachhead, not the boundary.

**Two failure modes named as the core problem:** screening goes shallow, and interviews go
unprepared. The second was added after it became clear the interview-questions use case was
solving a problem the document never stated.

**Six use cases specified** (UC-0 to UC-5) with actors, flows, alternate flows and outcomes,
plus a use case diagram:

- UC-0 Authenticate and manage workspace access
- UC-1 Create a job opening from natural-language requirements
- UC-2 Batch-screen resumes against a job opening
- UC-3 Generate candidate-specific interview questions
- UC-4 Monitor hiring pipeline and stale positions
- UC-5 Enforce candidate data retention

**Decisions made while scoping the use cases:**

- **Rejection reasoning is not a use case.** It is a required justification output of UC-2's
  scoring and is stored on the rejection decision. Internal only — never sent automatically
  to a candidate.
- **PDPA was split.** Retention enforcement became UC-5, scheduler-driven and auditable.
  Consent gating remains a cross-cutting policy rather than a use case of its own.
- **Talent-pool re-matching deferred as D-1.** Parked, not dropped: it cannot be demonstrated
  without a deep talent pool, multiplies scoring cost across the archive, and raises
  purpose-limitation questions UC-5 must answer first. UC-2 still builds the normalised pool
  and scoring stays a separable *(profile, criteria)* behaviour so it can be reinstated
  without rework.
- **Actors reduced to Recruiter and Admin.** The separate Hiring Manager role was removed;
  Admin is a generalisation of Recruiter. Candidate is indirect, System Scheduler supporting.
- **All «include» relationships removed** from the use case diagram. Once re-matching was
  deferred, nothing shared a sub-behaviour, and factoring parts of UC-2 out purely to populate
  the diagram would have added notation without meaning. Only the conditional
  «extend» *Draft Interview Invitation Email* remains.
- **Non-customers section dropped** from Target Customers.

**Repository created.** Documentation-only for now, intended to become a monorepo once
implementation starts. Added `README.md`, `.gitignore` (which excludes `/data/` and
`/uploads/` — real candidate data must never be committed), `CLAUDE.md`, an ADR template following the
lecturer's Context/Decision/Status/Consequences sample, and an ADR index.

**Documentation split fixed.** `PROPOSAL.md` and `CONTEXT.md` had overlapping content that
went stale immediately. `PROPOSAL.md` is now the single source of truth for anything settled;
`CONTEXT.md` holds only open questions, open decisions, the glossary, and provenance.

**Still outstanding:** Functional Requirements, Non-functional Requirements, and at least
three ADRs. The use case diagram needs redrawing in proper UML notation for submission.
