# ADR-018: Resume files live in a MinIO server that each deployment runs itself

**Date:** 2026-11-13
**Deciders:** Patiphon Puntusin

> In the context of storing the original resume files that every screening batch uploads, facing
> a choice between a managed cloud object store and one we run ourselves, we decided to **run a
> MinIO server inside every deployment, in production as well as locally**, to achieve a
> deployment that holds all of a customer's candidate data in one place we control and that runs
> unchanged on a laptop, accepting that keeping those files durable, backed up and erased is now
> our job rather than a cloud provider's.

---

## Summary

### Issue

A resume file is the most sensitive thing HireAssist stores: the candidate's own document, with
their name, contact details and history, and sometimes a photograph or a date of birth. Each
deployment receives them in batches (UC-2, screening a batch of resumes) and must delete them
when the retention period ends or the candidate asks for erasure (UC-5).

[ADR-003](ADR-003-polyglot-persistence.md) put the files in object storage rather than in
either database, described as "S3-compatible; MinIO locally". It did not say what stores them in
production. Two readings followed: some documents call object storage an external system, while
the code and the local environment run MinIO as one of the system's own processes. A
deployment cannot be planned, costed or drawn until one of the two is chosen.

The forces:

- **Each customer gets its own deployment** ([ADR-006](ADR-006-single-tenant-deployment.md)).
  Whatever stores the files is provisioned once per customer.
- **PDPA.** The customer is the data controller and we are the processor. Every additional
  company that holds candidate files is another processor to name, and another place where
  erasure has to be proven to have happened.
- **The demo runs on one laptop**, from one compose file, with no cloud account.
- **Both services already speak the S3 API.** Hiring writes with `minio-go` and Resume
  Processing reads with the AWS SDK. Neither depends on anything specific to MinIO.

### Decision

**Every deployment runs its own MinIO server, and the resume files live there, in production as
well as locally.** It is part of HireAssist, not an external system: it is provisioned, upgraded
and backed up with the deployment, and it sits on the deployment's private network, which only
Hiring and Resume Processing reach.

- One bucket, `resumes`. Hiring writes a file once when a batch is uploaded, Resume Processing
  reads it, and only the retention and erasure path deletes it. PostgreSQL holds the object key,
  as ADR-003 says.
- The services keep talking to it only through the S3 API, behind their object-storage
  adapters. Nothing MinIO-specific enters either service.
- The MinIO console and API are never published outside the deployment, for the same reason
  that the Hiring and Identity Services are not.

### Status

**Accepted**

### Group

Data · Deployment

---

## Details

### Assumptions

- At small-business load, a deployment holds thousands of files, not millions: a few gigabytes,
  which fits on one MinIO node with one disk.
- Files are written once and never modified, so an object store's consistency and versioning
  features are not needed.
- Customers either run HireAssist on infrastructure they choose, or accept a deployment we
  host. In both cases, keeping the files inside the deployment is something we can promise.

### Constraints

- ADR-003: resume files are kept out of the databases, and PostgreSQL holds only the key.
- ADR-006: one deployment per customer, and nothing shared between deployments.
- The local environment must not need a cloud account or credentials.

### Positions

1. **Amazon S3 in production, MinIO only locally.** A managed service with eleven nines of
   durability and nothing to operate. The production store differs from the one tested.
2. **Self-hosted MinIO everywhere.** *(Selected.)*
3. **Another managed store** (Google Cloud Storage, Azure Blob). Same trade-off as 1, with an
   API the services do not already use.
4. **No object store: files on a volume mounted into Hiring, or bytes in PostgreSQL.** One
   process fewer to run. Resume Processing could then not read a file without going through
   Hiring or sharing its volume, and PostgreSQL would fill up with binary data that ADR-003
   kept out of it on purpose.

### Argument

**Against Amazon S3 (1) and other managed stores (3):**
- The durability is real, but we would be paying for it with a second home for the most
  sensitive data in the system. That home sits under another company's account terms, in a
  region that has to be chosen and justified for each customer.
- Every customer would also need a cloud account and credentials managed for them, which works
  against the per-customer deployment ADR-006 chose.
- Testing on MinIO and running on S3 means the configuration that matters most is never
  exercised before it goes live: credentials, bucket policy and endpoint.

**Against a mounted volume or the database (4):** ADR-003 already rejected it. The two services
that need the files are separate processes, and a shared volume is a shared data store by
another name, which ADR-001's rule that no service reads another's store forbids.

**For self-hosted MinIO (2):**
- Every byte of candidate data stays inside the deployment, on one private network, under one
  set of backups and one erasure procedure.
- The demo, the integration tests and production run the same server with the same
  configuration.
- It costs one more container in each deployment, which the deployment already budgets for:
  ADR-006 counts object storage among its parts.
- The services stay on the S3 API. If a customer insists on a managed store later, changing
  endpoints and credentials is enough, and no code changes. That only makes the choice cheap to
  change; the decision is still ours to make.

### Implications

**What this buys us**

- One place holds all of a customer's candidate data, and every part of it is something we
  run. Deleting the stored resume file everywhere it is held (FR-5.3), and reporting a deletion
  that failed as pending rather than done (FR-5.8), are checked against stores we can inspect.
- No cloud provider is added as a processor of candidate files.
- Local, test and production storage are identical.

**What it costs us**

- **Durability is ours.** A single MinIO node with one disk can lose every file if that disk
  fails. Each deployment now needs backups of the MinIO volume, taken and restored alongside
  the databases.
- **Encryption at rest has to be set up by us**, with MinIO's own encryption or an encrypted
  disk. A managed store would provide it by default.
- **Upgrades and security fixes are ours**, once per deployment (ADR-006 multiplies them). The
  image must be pinned to a version rather than following `latest`, so that every deployment
  runs a known release.
- **We depend on MinIO's licence (AGPL v3) and on how its open-source edition is distributed**,
  neither of which we control. Using it unmodified over the network is allowed. If the edition
  stops being usable, staying on the S3 API keeps the exit open.
- **It is one more process to monitor.** If MinIO cannot be reached, a batch cannot be uploaded,
  and Resume Processing fails the work as temporary until MinIO is back.

---

## Related

### Related decisions

- [ADR-003](ADR-003-polyglot-persistence.md): this record states what ADR-003 left open: what
  serves its object storage in production.
- [ADR-006](ADR-006-single-tenant-deployment.md): the MinIO server is one of the parts that
  each customer deployment contains.
- [ADR-001](ADR-001-service-decomposition.md): no service reads another's store. The bucket
  belongs to Hiring; Resume Processing reads it only for the file named in the screening
  request it was sent.
- [ADR-013](ADR-013-rabbitmq-for-the-screening-handover.md): a `screening.requested` message
  carries the object key that Resume Processing fetches.

### Related requirements

- **UC-2, FR-2.7**: retry, then manual review. Uploading a batch stores each file, and a file
  that cannot be fetched is retried before it is flagged.
- **UC-5, FR-5.3**: on expiry, the stored resume file is deleted along with the rest of the
  candidate's data. **FR-5.8**: a failed deletion is retried and reported as pending.
- **NFR-13**: a valid erasure request is completed within 30 days. Deleting the file in MinIO is
  part of it.
- Course requirements: unaffected. The data stores the course requires are still PostgreSQL and
  MongoDB.

### Related artifacts

The Service–Operations–Collaborators table and the architecture diagram, which should show
object storage as part of the system rather than as an external one. The container diagram
already does. The deployment diagram should show it too.
