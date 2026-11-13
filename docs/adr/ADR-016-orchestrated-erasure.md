# ADR-016: Compliance & Insights orchestrates every erasure and keeps it pending until each holder confirms

**Date:** 2026-11-13
**Deciders:** Patiphon Puntusin

> In the context of erasing a candidate's personal data from every service that holds it
> (UC-5), facing a choice between one service directing the erasure and each holder reacting to
> an "erase" event on its own, we decided that **the Compliance & Insights Service calls each
> holder in turn and records which have confirmed, keeping the erasure pending and retrying until
> all have**, so that one place can say whether a person has been erased, accepting that it must
> know every holder by name.

---

## Summary

### Issue

A candidate's personal data is held in three places:
- the Resume Processing Service holds the candidate profile, the parsed resume text and the
  justifications;
- the Hiring Service holds the screening results, the shortlist decisions and the interview
  guides;
- Hiring's object storage holds the original resume file.

When the retention period ends, or an Admin asks for a named candidate to be erased (FR-5.7),
the data has to go from all of them. The system must then be able to say that it has gone
(FR-5.5, the audit entry). If any part fails, it must say "deletion pending", not "erased", and
try again (FR-5.8). A valid erasure request must be completed within 30 days (NFR-13).

No transaction spans services, so the erasure cannot be atomic. The decision is who keeps track
of it.

### Decision

**The Compliance & Insights Service orchestrates every erasure.**

1. It creates an erasure record with a random reference, the reason (expired, requested, or a
   held expiry resolved), the expiry action and the retention period in force.
2. It asks each holder to erase the candidate over REST. Hiring is asked for screening data,
   decisions, guides and the resume file. Resume Processing is asked for the profile, the text
   and the justifications. A holder answers success once it holds nothing about the candidate,
   including when it held nothing to begin with, so asking twice is safe.
3. It records which holders confirmed. If any did not, the erasure stays **pending** with the
   failure recorded, and every scheduled sweep asks the outstanding holders again.
4. When the last holder confirms, Compliance first removes the candidate id from everything
   it keeps itself, then marks the erasure **erased** and deletes the candidate id from the
   record. What remains is the audit entry: the reference, what was erased, when, and under
   which policy, with nothing that resolves to the person.

A second request for a candidate whose erasure is still pending retries that erasure; it does
not start another. The pipeline counts survive, because they never held a person, only a resume
reached a stage (FR-5.4).

### Status

**Accepted**

### Group

Data · PDPA

---

## Details

### Assumptions

- There are two holders today and they change rarely. Adding a holder is a code change in
  Compliance, on purpose (see Implications).
- Holders can make their erasure idempotent: deleting what is already gone succeeds.
- A pending erasure that is retried hourly finishes well within 30 days unless a holder is
  broken. A broken holder is something people should notice, and the pending record is how they
  do.

### Constraints

- PDPA: erasure must be complete and must be shown to be complete.
- FR-5.5: the audit entry may not carry the candidate's name, contact details, files, profile
  values, scores or justification text, and its identifier must not resolve to the person once
  erasure is complete.
- No service reads another's store (ADR-001), so Compliance cannot delete from another
  service's database itself.

### Positions

1. **Orchestration: Compliance calls each holder and tracks completion.** *(Selected.)*
2. **Choreography: Compliance publishes "candidate erasure requested", each holder erases and
   publishes "erased".** Holders are decoupled from Compliance. Compliance must still collect
   the confirmations to know when it is done, so it ends up tracking the same state, built
   from events instead of answers.
3. **Each holder runs its own retention sweep** against a policy it is given. There is no
   cross-service erasure at all, and no single place knows whether a person is gone.

### Argument

**Against holders sweeping on their own (3):**
- Nobody could answer "has candidate X been erased?" without asking every holder and trusting
  each to have run the same rule on the same day.
- The audit entry FR-5.5 requires would have no author.
- A held expiry, a candidate in an active hiring process (FR-5.6), needs one decision applied
  everywhere, which this option cannot express.

**Against choreography (2):** it decouples the holders, but the requirement that makes erasure
hard is knowing it finished, and that needs a list of who must answer. With choreography that
list exists anyway, implicitly, as the set of "erased" events Compliance waits for. A holder
that never subscribed would leave the erasure pending forever with no error naming it. With
orchestration, a missing holder is a named failure on the record.

**For orchestration (1):**
- The record shows which holders are done and why the others are not.
- Retries are the sweep that already runs.
- Making the candidate id unresolvable happens in one place and at one moment.
- It is also the simplest version to test: each holder is one REST call with a defined success.

### Implications

**What this buys us**

- One record answers "erased or not?" for every candidate, with the reason, the policy and the
  date (FR-5.5).
- A failure is visible and retried, never silent (FR-5.8): an erasure stays "pending" and names
  the holder that failed.
- Holders implement one idempotent operation each and need no knowledge of retention.

**What it costs us**

- **Compliance must know every holder.** A new service that stores candidate data must be added
  to Compliance's holder list, or its data is never erased. This is deliberate, because a list
  can be reviewed, but it is a step someone can forget.
- **Erasure is not atomic across services.** Between the first holder's confirmation and the
  last, some data is gone and some is not. The record says "pending" during that window, which
  is what FR-5.8 asks for, but FR-5.3's "single operation" is met by the record, not by the
  stores.
- **Holders must not treat 404 loosely.** Resume Processing's "not found" means it holds nothing,
  and counts as success. Hiring's contract is a `204` only, because a Hiring that does not
  serve the route also answers 404, and reading that as success would report data as erased
  that is still held.
- **Anonymisation and deletion behave the same at the holders today.** Both remove the personal
  data, and only Compliance's anonymous counts are kept. Whether an anonymised record should
  keep more at a holder (for example a screening score without its person) is open, and would
  change the holder contracts.
- **While pending, the record names the candidate.** The id is needed to retry. It is removed
  only on completion, so a holder that stays broken keeps a link to the person in Compliance
  until it is fixed.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md): no service reads another's store, which is why
  each holder erases its own data.
- [ADR-003](ADR-003-polyglot-persistence.md): candidate data spans PostgreSQL, MongoDB and
  object storage. Every MongoDB document carries the candidate id so a holder can delete one
  person's data directly.
- [ADR-018](ADR-018-self-hosted-minio-for-resume-files.md): the resume file is in the
  deployment's own MinIO, and Hiring deletes it as part of its erasure.
- [ADR-015](ADR-015-pipeline-events-on-the-broker.md): until Hiring publishes pipeline events,
  the sweep cannot tell whether a candidate is still in an active process. It then holds the
  expiry instead of erasing.

### Related requirements

- **UC-5:**
  - FR-5.3: on expiry, everywhere the data is held;
  - FR-5.4: anonymisation keeps the aggregate counts;
  - FR-5.5: the audit entry, with a pseudonymous reference;
  - FR-5.6: held expiry;
  - FR-5.7: immediate erasure by an Admin;
  - FR-5.8: failures retried and reported as pending.
- **NFR-13:** an erasure request is completed within 30 days.

### Related artifacts

The Service–Operations–Collaborators table (`eraseCandidate()`, `eraseHiringData()`,
`eraseCandidateProfile()`), the UC-5 call sequence, and the container diagram.
