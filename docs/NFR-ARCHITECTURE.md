# How the Architecture Meets the Key Non-functional Requirements

This section takes the non-functional requirements that shape the architecture most and states,
for each one, what the architecture does to meet it, which decision records it, and how it is
verified. The requirement IDs refer to `NON-FUNCTIONAL-REQUIREMENTS.md`.

| Quality attribute | Requirements | How the architecture meets it | Decision | Verified by |
|---|---|---|---|---|
| **Scalability** *(the attribute this project demonstrates)* | NFR-06, NFR-07, NFR-08 | Each resume is one RabbitMQ message. Every Resume Processing replica consumes from the same queue, so adding replicas adds throughput with no code change. | ADR-002, ADR-013 | Comparative load test: 1, 2 and 4 workers |
| **Reliability** | NFR-10 | Messages are acknowledged only after the result is stored. Transient failures are retried with backoff, and work that keeps failing is parked in a dead-letter queue instead of being dropped. | ADR-013 | Fault-injection test: AI Service stopped mid-batch |
| **Performance** | NFR-01, NFR-02, NFR-04 | Hiring acknowledges a batch as soon as the files are stored and the messages are published. Every model call has a 60-second deadline and is retried after it. | ADR-002, ADR-013 | Load test |
| **Availability** | NFR-09 | Every service runs as a Kubernetes Deployment behind a ClusterIP Service, with readiness and liveness probes. A replica that fails is restarted and taken out of rotation until it is ready again. | ADR-001 | Monitoring |
| **Security & privacy** | NFR-11, NFR-12, NFR-14 | Tokens are verified only at the gateway. The AI Service is unreachable from outside the cluster. The model never receives a raw file, and protected attributes are excluded from scoring input. | ADR-004, ADR-008, ADR-009, ADR-010, ADR-012 | Configuration and prompt review; paired-resume test |
| **Transparency & modifiability** | NFR-15, NFR-17 | Per-criterion evidence is stored with every score and returned over REST. All model access is in one AI Service behind a gRPC contract, so the model or provider changes in one place. | ADR-003, ADR-004, ADR-007 | Inspection; design review |

## Scalability (NFR-06, NFR-07, NFR-08)

At roughly twelve seconds per resume, a batch of 100 takes about twenty minutes when processed one
at a time. The ten-minute target in NFR-06 can only be reached through concurrency. Each resume is
an independent unit of work (ADR-002) carried as one message on `screening.requested` (ADR-013).
RabbitMQ shares those messages across every Resume Processing replica, each of which holds at most
four in flight. Throughput therefore grows with the replica count. In Kubernetes that is one number
in the Deployment, and it needs no code change. The practical ceiling is the model provider's rate
limit, not the design.

## Reliability (NFR-10)

A batch is acknowledged before it is processed, so a Recruiter cannot tell a lost resume from a
slow one. The system must therefore guarantee it never loses one. Four mechanisms provide that
guarantee:

1. **Late acknowledgement.** A request is acknowledged only after its result is saved and the
   broker has confirmed every outgoing message. A crash before that point leaves the message in
   the queue, and RabbitMQ delivers it again.
2. **Retry with backoff.** A transient failure, such as the AI Service being unreachable or a
   model call passing its 60-second bound, is retried after about 5, 10 and 20 seconds.
3. **Dead-letter queue.** Work that still fails is moved to `screening.requested.dlq` with its
   error, and Hiring receives a `failed` result instead of waiting forever. An operator replays the
   messages once the cause is fixed.
4. **Idempotency.** Results are keyed on `(batch_id, resume_id)`, so a redelivered message is
   answered from the stored result without calling the model a second time.

The fault-injection demonstration stops the AI Service in the middle of a batch. The affected
resumes retry and are dead-lettered. After the service is restarted and the dead-letter queue is
replayed, every resume is scored and none is lost.

## Performance (NFR-01, NFR-02, NFR-04)

NFR-04 requires a batch to be acknowledged within two seconds whatever its size. That requirement
is what makes the screening pipeline asynchronous: Hiring stores the files, publishes one message
per resume and returns immediately, and results arrive as each resume finishes. NFR-01 and NFR-02
bound each model call at 60 seconds. The gRPC client enforces that bound as a deadline, and a call
that passes it counts as a transient failure and is retried.

## Availability (NFR-09)

Every service is a Kubernetes Deployment behind a ClusterIP Service (ADR-001). Readiness probes
keep a replica out of rotation until its dependencies are connected. For Resume Processing,
`/readyz` passes only when MongoDB and RabbitMQ are both reachable. Liveness probes restart a
replica that stops responding. Because work waits in RabbitMQ, a service can restart in the middle
of a batch without dropping anything.

## Security and privacy (NFR-11, NFR-12, NFR-14)

Access tokens are short-lived JWTs (ADR-009) and are verified only at the gateway (ADR-010). The
web application's refresh token is held in an HttpOnly cookie (ADR-012). The AI Service holds the
only model credential and is not reachable from the gateway (ADR-004). It never receives a raw
resume file: Resume Processing extracts the text first. Gender, age, marital status, religion,
nationality and photograph are stripped from the input to scoring (ADR-008). NFR-14 can be tested
directly: two resumes that differ only in a protected attribute must receive the same score.

## Transparency and modifiability (NFR-15, NFR-17)

Every score is stored together with its per-criterion evidence in MongoDB (ADR-003). Hiring reads
that evidence with `GET /candidates/{id}`, so a Recruiter can check a score without reading the
resume. All model access goes through the single AI Service behind a gRPC contract (ADR-004,
ADR-007). Changing the prompt, the model or the provider touches only that service: OpenAI can
already replace the default provider through configuration.
