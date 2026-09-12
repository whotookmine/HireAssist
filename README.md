# HireAssist

An AI-assisted hiring-support layer for companies that receive more applications than they
can screen carefully and interview well.

HireAssist is **not** a job board, **not** a job-application platform, and **not** another
applicant-tracking system. It attaches to the hiring channels a company already uses and
supplies the judgement capacity it lacks: screening every resume with a stated reason,
preparing interview questions grounded in the specific resume and role, surfacing positions
that have stalled, and keeping candidate data compliant with Thailand's PDPA.

> The human still decides. HireAssist makes sure they are deciding on evidence they actually
> had time to look at.

Term project for **Software Architecture**, Chulalongkorn University.

---

## Team

| Student ID | Name | Nickname |
|---|---|---|
| 6772055221 | Patiphon Puntusin | Me |
| 6872036421 | Thanabul Parodom | Tim |
| 6870406621 | Thanwarat Korcharoenkiat | Yo |
| 6870235621 | Rerngrit Jangsri | Frank |

---

## Documentation

| Document | What it is |
|---|---|
| [docs/PROPOSAL.md](docs/PROPOSAL.md) | **The submission document.** Project description, target customers, use cases, requirements, ADRs. Source of truth. |
| [docs/FUNCTIONAL-REQUIREMENTS.md](docs/FUNCTIONAL-REQUIREMENTS.md) | 44 functional requirements, traced to the use case each serves. |
| [docs/NON-FUNCTIONAL-REQUIREMENTS.md](docs/NON-FUNCTIONAL-REQUIREMENTS.md) | 17 non-functional requirements, grouped by quality attribute. |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Architecture diagram (version 1) with the three business use cases traced through the services. |
| [docs/diagrams/](docs/diagrams/) | Diagrams as code — the use case diagram (PlantUML source + rendered SVG) and the architecture diagram (hand-laid SVG). |
| [docs/CONTEXT.md](docs/CONTEXT.md) | Internal working notes — open questions, open decisions, glossary. Not for submission. |
| [docs/adr/](docs/adr/) | Architecture Decision Records, one decision per file, with an index. |
| [docs/course/ASSIGNMENT.md](docs/course/ASSIGNMENT.md) | The assignment brief and submission guideline this proposal is written against. |
| [docs/course/](docs/course/) | Course requirements and grading breakdown. |
| [docs/reference/](docs/reference/) | The lecturer's ADR samples (one good, one deliberately poor). |
| [CHANGELOG.md](CHANGELOG.md) | Dated summary of what changed in the docs, and why. |

---

## Use cases

| ID | Use Case | Primary Actor |
|---|---|---|
| UC-0 | Sign in | Guest |
| UC-1 | Create a job opening from natural-language requirements | Recruiter |
| UC-2 | Batch-screen resumes against a job opening | Recruiter |
| UC-3 | Generate candidate-specific interview questions | Recruiter |
| UC-4 | Monitor hiring pipeline and stale positions | Recruiter |
| UC-5 | Enforce candidate data retention | System Scheduler / Admin |
| UC-6 | Manage members and roles | Admin |

One use case — talent-pool re-matching — is deliberately deferred and recorded as **D-1** in
the proposal.

---

## Architecture decisions

Architecture Decision Records live in **[`docs/adr/`](docs/adr/)**, one decision per file, with the
index and the decisions not yet taken in [`docs/adr/INDEX.md`](docs/adr/INDEX.md).

---

## Repository status

**Documentation only.** No application code yet.

The intention is for this to become a monorepo once implementation starts, with services
living alongside these documents. Structure will be decided in an ADR before any code lands.

```
hire-assist/
├── README.md
├── .gitignore
└── docs/
    ├── PROPOSAL.md          # submission document
    ├── CONTEXT.md           # working notes
    ├── diagrams/            # diagrams as code (.puml) + rendered .svg
    ├── adr/                 # architecture decision records
    ├── course/              # course requirements & grading
    └── reference/           # lecturer-provided samples
```

---

## Working agreements

- `docs/PROPOSAL.md` is the single source of truth. If something is settled, it lives there
  and is not duplicated elsewhere.
- Every significant architectural decision gets an ADR in `docs/adr/`, using
  [TEMPLATE.md](docs/adr/TEMPLATE.md), and is listed in [INDEX.md](docs/adr/INDEX.md).
- Every documentation change gets a `CHANGELOG.md` entry saying **why**, not just what.
- Commit under your own account — per-member Git contribution is part of the project grade.
