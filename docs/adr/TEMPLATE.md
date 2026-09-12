# ADR-NNN: <short noun phrase naming the decision>

> Copy to `ADR-NNN-short-slug.md`, delete this line and the guidance in *italics*, fill it in,
> and add a row to [INDEX.md](INDEX.md).

Based on the **Jeff Tyree & Art Akerman** decision record template, as presented in the
course's ADR slide deck — that deck is not part of this repository, so everything we rely on
from it is reproduced below. Our additions to the original template are noted at the end.

Fields marked **(optional)** may be omitted when they would be empty — do not write "N/A" in
every one of them. Everything else is required: those are the fields that make it an
architecture decision record rather than a note.

---

**Date:** YYYY-MM-DD  **Deciders:** <who was in the discussion>

> *In the context of `<use case or component>`, facing `<concern>`, we decided for `<option>`
> to achieve `<quality>`, accepting `<downside>`.*
>
> *One-sentence summary. Borrowed from the Alexandrian template — if you cannot write this
> sentence, the decision is not yet clear enough to record.*

---

## Summary

### Issue

*Why are we addressing this? State the problem and the forces at play — technical,
regulatory, cost, time, team skill. Facts, not opinions. A reader who knows nothing about the
project should understand why a decision was needed at all.*

### Decision

*The architecture's direction; the position we selected. State it as a commitment in the
present or active voice — "We use X" / "We will use X" — never "we will investigate X".*

### Status

*One of:* **Proposed** *(pending — stakeholders have not agreed yet)* · **Accepted**
*(decided/approved)* · **Deprecated**

*Set once, when the record is written. A later ADR does not reach back and change it — see
"An ADR is a record made at a point in time" below.*

### Group *(optional)*

*Category, to help organise a growing set of decisions — e.g. Data, Communication, Security,
Deployment, AI/LLM.*

---

## Details

### Assumptions

*What we are taking as true at the time of deciding — schedule, cost, technology, team
capability, expected load. These are what a future reader checks first when the decision
starts to look wrong.*

### Constraints *(optional)*

*Any additional constraint the decision itself imposes, or that bounded the choice. Course
requirements, PDPA, budget, or something an earlier ADR already fixed.*

### Positions

*The other options we considered. Name them, even the ones rejected quickly — this exists to
answer "did you think about…?" before it is asked. **An ADR with no positions is not an ADR.***

### Argument

*Why we selected this position over those. Refer to the forces in Issue and the options in
Positions: implementation cost, total cost of ownership, time to market, availability of
development resources, team familiarity, operational risk.*

### Implications

*What becomes true because of this decision — **both good and bad**. New requirements it
creates, existing requirements it modifies, constraints it imposes, scope or schedule that
must be renegotiated, training the team now needs, what it forecloses. A list of only
positive implications means the trade-off has not been examined.*

---

## Related

### Related decisions

*Other ADRs this depends on, contradicts, enables, or supersedes. Reference by ID.*

### Related requirements

*Decisions should be requirement-driven. Map this decision to what it serves: the use cases
(`UC-0`–`UC-6`, `D-1`), the non-functional requirements, and the course requirements in
`docs/course/REQUIREMENTS.md`. This is where traceability is demonstrated — do not skip it.*

### Related artifacts *(optional)*

*Architecture, design or scope documents this decision impacts — the architecture diagram,
the Service–Operations–Collaborators table, `docs/PROPOSAL.md`.*

### Related principles *(optional)*

*Any project principle this aligns with or trades against — e.g. "the human still decides",
"explainability is required, not optional", "easily reversible".*

---

## Notes *(optional)*

*Discussion notes, dissent, things raised and parked, links consulted. Useful for
reconstructing how the team got here.*

---
---

# Guidance

## What we changed from Tyree & Akerman

The 14 fields are theirs. We added three things:

1. **Date and Deciders** — the original has no traceability header. Per-member Git
   contribution is graded, and knowing *when* a decision was made matters when reading it back
   against assumptions that have since changed.
2. **A one-sentence summary** at the top, taken from the Alexandrian template. It is a cheap
   test: if the sentence will not form, the decision is not ready to be written down.
3. **Optional fields marked explicitly.** Fourteen mandatory sections on a term project
   produces filler, and filler makes the real content harder to find. Omit an optional field
   rather than padding it.

We also follow Nygard's advice on length and voice: **one to two pages**, and *write it as if
it were a conversation with a future developer.*

## What makes a bad ADR

From the lecturer's two samples in [`../reference/`](../reference/):

`adr-sample-good.pdf` names the real alternatives, says why each was rejected, records a
decision about *architecture*, and states honest negative consequences alongside the benefits.

`adr-sample-bad.pdf` fails in ways worth naming, because they are easy to repeat:

- The **Decision lists activities, not a decision** — *"We will interview people"*, *"We will
  learn about microservice architecture"*. Those are tasks on a plan. An ADR records a choice
  between options that changes the shape of the system.
- **No alternatives** appear, so nothing was really decided — there was only ever one path.
  In this template that is the empty **Positions** field.
- **Consequences describe project events** ("we made an appointment for a future meeting")
  rather than architectural implications.
- The **Issue states a product question** ("can we build a reviews hub?"), which is scoping,
  not an architectural force.

## An ADR is a record made at a point in time

**An ADR may only cite an ADR that already existed when it was written.** A record dated March
cannot reference a decision taken in September — the author did not know it, and pretending
otherwise turns a historical record into a rolling summary that nobody can trust as evidence of
what was known and when.

This has three consequences:

1. **Reference backwards only.** ADR-004 may cite ADR-002. ADR-002 may not cite ADR-004. If an
   earlier record needs to mention a question it leaves open, name the *subject* — "how candidate
   data is erased across services" — never a number that does not exist yet.
2. **Never edit an earlier ADR's body to reflect a later decision.** The argument, the positions
   and the implications are what was believed then, and they stay that way even when they turn out
   to be wrong. A later reader needs to see the reasoning that was actually used.
3. **Never edit an earlier ADR at all — not even its Status — because of a later one.** The
   records are a stack: you append, you do not reach back. An ADR is finished when it is written,
   and it is only revised when someone deliberately asks for it to be, not as a side effect of
   writing the next record.

   This means an earlier ADR will describe things that are no longer true. That is correct and
   intended: it is the record of a decision made on a date, not a description of the system today.
   A reader who needs the current position reads the whole stack, newest last, or reads
   `INDEX.md`, which is a living document and carries what each record has since changed.

4. **To change a decision, push a new record.** Name the earlier one, state exactly which part of
   it no longer holds, and give the argument for the replacement. The new record is where the
   change is stated and the only file that gets written. There is no status to flip on the old
   one, no note to add to it, nothing to strike through — it stays exactly as it was.

   This is why there is no *Superseded* status. A decision is overridden by the record that
   overrides it, which is discoverable by reading forward; marking the old file would mean
   reaching back into a finished record to point at the future.

The later ADR carries the explanation. It is the record that knew, and it is the only one that
should be written to.

## Write it to stand alone

**An ADR must make sense to someone who has only this file.** It will be read on its own — in a
review, by a new team member, pulled out of the repository entirely — so it may not depend on
another document to be understood.

- **Do not reference project documents.** No "see the proposal", no file paths, no links to the
  requirements or the changelog. If a fact from one of them matters, state the fact.
- **Requirement and use case identifiers are allowed**, because traceability is the point of
  *Related requirements* — but say what each one means the first time it appears. `FR-2.7` tells a
  stranger nothing; "retry, then manual review (FR-2.7)" tells them everything they need.
- **Other ADRs may be linked**, because they are part of the same record set and a reader who has
  one can find the rest.

## Keep it short

Aim for one to two pages. Every sentence should carry a fact, a reason or a consequence. Cut
throat-clearing, cut restatement, and cut any sentence that explains what you are about to say
next. A long ADR is not more rigorous — it is less likely to be read, and an unread record is the
same as no record.

## Rules of thumb

- If you cannot name a rejected **Position**, it is probably not an ADR.
- An ADR never cites one written after it. Only its Status is updated later.
- An ADR never points at another project document. State the fact instead.
- If the decision could be reversed with no structural change to the system, it is probably
  not an ADR.
- Write it when the decision is made, not reconstructed before submission — the rejected
  alternatives are exactly what gets forgotten.
- One decision per ADR. If the title needs an "and", split it.
- Numbers are never reused. Superseding never edits the original in any way — write a new record
  that states what it replaces, and let `INDEX.md` show which records are still in force.

## Other templates, for reference

The course's ADR deck presents several (summarised here, since the deck is not in this repo).
We chose Tyree & Akerman because it is the most thorough, and because it forces the rejected
alternatives and the requirement traceability into the open.

| Template | Character |
|---|---|
| **Tyree & Akerman** | Sophisticated; 14 fields, grouped. **← what we use** |
| Michael Nygard | Simple and popular: Title, Context, Decision, Status, Consequences |
| Alexandrian pattern | Simple with a one-sentence prologue; we borrowed that sentence |
| Business case | MBA-oriented: costs, SWOT |
| MADR | Markdown-native |
| Planguage | Quality-assurance oriented |
