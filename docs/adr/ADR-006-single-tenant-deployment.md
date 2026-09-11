# ADR-006: One deployment per customer company

**Date:** 2026-09-11
**Deciders:** Patiphon Puntusin, Thanabul Parodom, Thanwarat Korcharoenkiat, Rerngrit Jangsri

> In the context of selling HireAssist to companies that each hold their own candidates' personal
> data, facing a choice between one shared installation serving every customer and one
> installation per customer, we decided to **deploy a separate instance of the whole system for
> each customer company** to achieve isolation by construction rather than by code, accepting that
> operating, upgrading and monitoring the product is now multiplied by the number of customers.

---

## Summary

### Issue

Every use case reads or writes candidate personal data belonging to one company. Until now the
documents assumed a shared installation in which a **workspace** identified the company, and in
which isolation between companies was a property the code had to maintain: a workspace id on the
session (FR-0.1), on every row and document, and on every internal call and broker message.

That assumption was never decided. It arrived with the multi-tenant SaaS framing in *Target
Customers* and propagated into the requirements and four ADRs without anyone weighing it.

The forces:

- **A leak between customers is the worst failure this product can have.** One company reading
  another's candidates is a PDPA breach involving a third party's personal data, reported by our
  customer, caused by us. There is no partial version of this failure.
- **Enforcement would be spread wide.** [ADR-001](ADR-001-service-decomposition.md) notes that
  tenant identity must travel on every gRPC call and broker message and be enforced by each
  service, not only at the gateway. [ADR-005](ADR-005-per-service-language.md) makes it worse: the
  check would exist in several codebases, in more than one language.
- **We are four part-time developers**, none with production multi-tenancy experience, on a
  deadline.
- **Customer count is small and slow-growing.** The proposal targets SMEs bought one at a time,
  not self-service signups arriving overnight.
- **Customers differ in what they may accept.** Some will require that their candidate data never
  shares a database with another company's.

### Decision

**Each customer company gets its own deployment of HireAssist** — its own services, its own
PostgreSQL and MongoDB, its own object storage and broker. A deployment serves exactly one
company, and there is no concept of a tenant inside it.

Consequently:

- **The workspace concept is removed.** A session identifies a user and a role, not a company
  (FR-0.1). There is no workspace id on a row, a document, a gRPC call or a broker message.
- **Isolation between companies is a property of deployment, not of code.** No query filter, no
  row-level policy, and no per-call tenant check stands between one company's data and another's,
  because the other company's data is not in the database.
- **FR-0.3 is withdrawn.** It required data access to be scoped to the workspace in the session.
  There is nothing left for it to scope.
- **Configuration that was per workspace is now per deployment** — the retention policy (FR-5.1)
  most of all.
- **Provisioning a customer means provisioning an instance**, and creating its first Admin account
  (UC-0). That is a development-team operation, not a feature.

Multi-tenancy is not ruled out for the future; it is out of scope for this project.

### Status

**Accepted**

### Group

Deployment · Security

---

## Details

### Assumptions

- Customers are onboarded individually, by us, at a rate measured in weeks rather than minutes.
- The number of customers during this project is one — the demo — and small afterwards.
- A deployment is cheap enough to run per customer. The system is five services, two databases, a
  broker and object storage; at SME load none of them needs to be large.
- Nobody needs to query across customers. No cross-customer reporting, benchmarking or shared
  talent pool is in scope. *(This is the assumption most likely to be wrong commercially — see
  Implications.)*

### Constraints

- Four part-time developers with no production multi-tenancy experience and a fixed deadline.
- PDPA liability for a cross-customer leak falls on us as processor and on our customer as
  controller.
- Everything must still run on one laptop for the demo — a single deployment, which this decision
  makes the normal case rather than a special one.

### Positions

1. **Shared installation, workspace id on every row and every call.** What the documents assumed.
   One deployment to operate, one upgrade to run, and cross-customer reporting stays possible.
   Isolation is a property the code must maintain everywhere, forever.
2. **Shared installation, database per customer.** One set of running services, but each
   customer's data in its own database, chosen per request from the session. Removes the
   query-filter class of bug but keeps a routing decision on every call.
3. **One deployment per customer.** *(Selected.)*
4. **One deployment per customer now, designed so a shared installation is possible later** —
   carry a tenant id through the schemas and contracts but always populate it with one value.

### Argument

**Against the shared installation (1):** the cost of getting it wrong is unbounded and the cost of
getting it right is paid in every service, every query and every message, forever. A workspace
filter omitted from one query in one service leaks candidates between companies, and nothing about
the system makes that omission loud — the query returns results, the screen renders, and only the
data is wrong. ADR-001 already flags that gateway-only enforcement is insufficient; ADR-005 puts
the same check in several languages. For a team of four with no prior multi-tenancy experience,
that is the highest-risk property in the whole design, and it is risk we are not obliged to take.

**Against database-per-customer in a shared installation (2):** it removes the worst failure mode
and keeps one set of services to operate, which is genuinely attractive. It was rejected because
the routing decision is still made on every request from session data, so a bug in resolving the
connection has the same consequence as a missing filter; and because the operational benefit —
one upgrade — matters at a customer count we do not have.

**Against carrying a dormant tenant id (4):** it buys future optionality at the cost of present
clarity. A field that always holds one value is not exercised, so it is not correct: the first
time a second value appears, every code path that ignored it is a bug. Recording this decision is
a cheaper way to keep the option than writing unused plumbing.

**For one deployment per customer (3):** isolation stops being something the code must achieve and
becomes something the deployment already is. The failure mode that worried us most cannot occur,
because the data is not present to leak. It also simplifies what remains: no tenant id in schemas,
contracts or messages; no per-call resolution; one less thing every service must get right in
whichever language it is written in. At our customer count, the operational cost of separate
instances is smaller than the engineering cost of the guarantee it replaces.

### Implications

**What this buys us**

- The cross-customer leak cannot happen. This is a guarantee by construction, which is the only
  kind worth making about personal data.
- Schemas, contracts and broker messages lose a field, and every service loses a check.
- A customer who demands their data never share a database with another company's is satisfied by
  the architecture, not by an explanation.
- Per-customer configuration — retention policy above all — is configuration, not data.
- Restoring one customer's backup, or deleting one customer entirely, is an operation on one
  instance rather than a surgical query across shared tables.

**What it costs us**

- **Operating the product now multiplies by customer count.** Deployments, upgrades, migrations,
  certificates, backups, monitoring and on-call all scale linearly. Ten customers is ten upgrades.
  This is the cost this decision buys the guarantee with, and it lands on a four-person team.
- **Every customer must be on a version we still support.** Without a deployment pipeline that can
  upgrade them all, they drift apart and a bug fix has to be applied N times.
- **Infrastructure cost per customer has a floor.** Two databases, a broker and object storage
  cost something even at zero load, so a small customer may be unprofitable at a low price.
- **No cross-customer anything.** No aggregate benchmarking ("your time-to-fill versus similar
  companies"), no shared talent pool, no product analytics without a separate pipeline that
  exports from each instance. If a future product decision needs any of those, this decision is
  what has to change.
- **Moving to multi-tenancy later is a migration, not a refactor.** Data from N databases must be
  merged and a tenant identity introduced everywhere at once. We accept this rather than pay for
  it in advance.
- **ADR-001's gateway loses one of its two stated jobs.** It still terminates TLS, validates the
  session token, resolves the role and routes; it no longer resolves a workspace or enforces
  tenant isolation. That record is amended accordingly.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — its Issue and Implications treated tenant
  isolation as a reason for a single gateway and as a cost of decomposition. This record removes
  that force; the gateway keeps its other reasons. The service named *Identity & Workspace
  Service* becomes the **Identity Service**.
- [ADR-003](ADR-003-polyglot-persistence.md) — documents no longer carry a `workspace_id`;
  `candidate_id` alone locates a person's derived data for erasure.
- [ADR-005](ADR-005-per-service-language.md) *(Proposed)* — its highest-risk implication was
  workspace-identity enforcement implemented in several languages. That risk is removed by this
  decision, which makes per-service language choice meaningfully safer.
- [ADR-004](ADR-004-llm-access.md) — per-workspace token budgets become per-deployment budgets.
- **Still open:** the deployment and upgrade pipeline this decision makes necessary — how N
  instances are provisioned, configured and kept on a supported version.

### Related requirements

- **FR-0.1** — the session identifies a user and a role, not a company.
- **~~FR-0.3~~** — withdrawn by this decision.
- **FR-5.1** — retention policy is per deployment.
- **NFR-12** — the access audit log remains, per deployment.
- **NFR-09** — availability is now per customer instance: a smaller blast radius, and N things to
  monitor.
- What is required of us is unaffected; the architecture is unchanged in shape.

### Related artifacts

The sign-in and access-management use cases, the actor descriptions, the use case diagram, and the
deployment configuration once implementation starts.

### Related principles

- *A guarantee by construction beats a guarantee by discipline.*
- *Do not build plumbing for a scale you do not have* — the dormant tenant id was rejected on
  exactly this ground.
- *Pay operational cost to remove a correctness risk when the team cannot carry the risk.*

---

## Notes

The workspace concept was never decided. It arrived with the multi-tenant framing and spread into
the requirements and four ADRs unchallenged, surfacing only when the actors' actual capabilities
were listed and nobody could say who creates a workspace — the first Admin appeared from nowhere.
That is the second decision found hiding in an assumption, after the backend language.

The strongest objection is commercial rather than technical: single-tenant products are harder to
sell cheaply, and the assumption that nobody needs cross-customer data is the one most likely to
be overturned by a real customer conversation — aggregate benchmarking is exactly the kind of
thing an SME would find valuable. If that happens, this is the record to revisit, and revisiting
it means a migration.
