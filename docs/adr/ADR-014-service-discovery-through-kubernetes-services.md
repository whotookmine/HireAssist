# ADR-014: Service discovery through Kubernetes Services, resolved by the cluster DNS

**Date:** 2026-10-31
**Deciders:** Thanwarat Korcharoenkiat, who owns service discovery for the progress submission —
carrying out the discovery decision all four members accepted in ADR-001

> In the context of eleven workloads that call one another while their instances are replaced,
> rescheduled and scaled, facing a course requirement to show how service discovery is managed
> and an accepted decision to leave it to Kubernetes that nothing had yet run on, we decided that
> every service addresses every other only by the name of a Kubernetes Service — resolved by the
> cluster DNS to a stable virtual IP, behind which only Pods that pass their readiness check
> receive traffic — to achieve discovery and health checking with no registry and no discovery
> code in any service, accepting that the team must run Kubernetes, that traffic is balanced per
> connection rather than per call, and that the Compose stack used for development resolves the
> same names without checking health.

---

## Summary

### Issue

HireAssist is eleven workloads: six of our own — the web app, the API gateway, and the Identity,
Hiring, Resume Processing and AI Services — and five it depends on: two PostgreSQL databases,
MongoDB, the RabbitMQ broker and MinIO object storage. They call one another over REST, gRPC and
AMQP. An instance's address is not fixed: a container gets a new IP whenever it is replaced,
rescheduled or scaled. A caller that holds an address breaks when that happens; a caller that
holds a list of addresses must be told when the list changes, and which entries are healthy.

ADR-001 answered this in one paragraph: each service a Deployment behind a ClusterIP Service,
addressed by in-cluster DNS, with readiness probes deciding membership and no separate registry.
It also named the risk: if the team could not run a cluster, discovery would have to be
reconsidered. Six weeks later the whole system ran under Docker Compose and only Resume
Processing had a Kubernetes manifest. The course now asks for a description of how discovery is
managed, which means the decision has to be carried out, not only cited — and the details
ADR-001 did not reach have to be settled: how each service learns the names it calls, what
*ready* means for each one, and what is reachable from outside.

### Decision

**Every service finds every other by the name of a Kubernetes Service. The platform registers
the instances, resolves the names and checks the health.**

1. **One Service per workload, named after it**, in the deployment's namespace:
   `hiring.hireassist.svc:8080`, `ai-service.hireassist.svc:50051`,
   `rabbitmq.hireassist.svc:5672`. A Service is a stable virtual IP with a DNS name; behind it,
   Kubernetes keeps the list of ready Pods carrying the Service's label. This is server-side
   discovery with third-party registration: no service registers itself, and no caller does more
   than a DNS lookup.
2. **Addresses are configuration, never code.** Each service reads the names it calls from
   environment variables set in its manifest — `HIRING_URL`, `AI_SERVICE_ADDRESS`, `AMQP_URL`.
   No IP address appears anywhere.
3. **Readiness decides membership; liveness decides restarts.** Each service's readiness probe
   reports whether it can do its job, not only whether it runs: Identity while its database
   answers, Resume Processing while MongoDB and the broker are connected, the AI Service through
   its gRPC health check, which reports not serving without a model credential, and the gateway
   once it holds a verification key. A Pod that fails is taken out of its Service until it passes
   again.
4. **The datastores are StatefulSets**, one replica each behind a Service of the same name, so a
   database keeps its name and its volume across a restart.
5. **Only the gateway and the web app are published**, as NodePorts. The web app serves static
   files and forwards `/api` to the gateway alone. Every other Service is ClusterIP, reachable
   only inside the cluster, which is the deployment rule ADR-010 depends on.
6. **The screening workers are not discovered at all.** Hiring publishes each resume to the broker
   and every Resume Processing replica consumes from the same queue (ADR-013), so a new worker
   needs no registration and no caller learns about it.
7. **Compose remains the development stack**, where Docker's embedded DNS resolves the same names
   in their short form — `hiring`, `ai-service` — to container IPs. The same images run on both,
   configured by the same variables.
8. **The local cluster is kind**: one node on a laptop, built and loaded by one script.

### Status

**Accepted.** It carries out ADR-001's accepted discovery decision, and every workload now has a
manifest. The manifests have not yet been run end to end.

### Group

Deployment · Communication

---

## Details

### Assumptions

- One deployment serves one company (ADR-006), so one namespace holds all of it and no name needs
  a tenant part.
- Tens of concurrent recruiters and batches of up to 200 resumes: per-connection balancing is
  enough for the HTTP traffic at this volume.
- A one-node local cluster is enough to show the mechanism; it behaves the same across nodes.

### Constraints

- The course requires a documented approach to service discovery and an API gateway, and every
  REST, gRPC and broker connection in the system crosses this mechanism.
- ADR-001 already rejected a separate registry, and ADR-005 asks every service for a health
  endpoint, a readiness endpoint, configuration from the environment and a Kubernetes manifest.

### Positions

1. **Kubernetes Services and the cluster DNS, with readiness deciding membership** *(chosen)*.
2. **A service registry with client-side discovery**, such as Consul or Eureka: each instance
   registers itself and sends heartbeats; each caller asks the registry and picks an instance.
3. **Docker Compose alone**, for development and the demonstration.
4. **Fixed addresses**: a host and port per service in configuration, edited when they change.
5. **Kubernetes for development as well**, with no Compose stack.

### Argument

**Against a registry (2).** It is a second system to run and explain, and it puts registration and
heartbeats into every service, in three languages. An instance that crashes without deregistering
stays listed until its heartbeat expires, while Kubernetes derives membership from probes it
already runs and removes a failing Pod at the next probe. A registry earns its keep when services
span platforms; these run on one.

**Against Compose alone (3).** It resolves names, which is most of discovery, but it checks
nothing. We tested it: a container whose health check fails is still returned by Docker's DNS, so
traffic keeps reaching an instance that cannot serve it — the Compose health check only orders
startup. It is also limited to one host, and its names point straight at container IPs, which
change when a container is recreated, so a client that caches the answer, as nginx does, keeps a
dead address.

**Against fixed addresses (4).** Every replacement and every new replica becomes a configuration
change. Scaling Resume Processing — the quality attribute this project demonstrates — would mean
editing a list.

**Against Kubernetes for development (5).** Every code change would become build, load and roll
out, where Compose restarts one container in seconds. Because services hold only names, both
platforms run the same images with the same variables, so we keep the fast loop for development
and the cluster for what Compose cannot show.

**For Kubernetes Services (1).** Discovery and health checking become one mechanism the platform
already runs: the probe that says a Pod is ready is the one that puts it behind its name. Nobody
writes registration or lookup code, and a Service's virtual IP outlives every Pod behind it, so a
caller that caches its DNS answer stays correct.

### Implications

**What this buys us**

- No discovery code in any service and no registry to operate.
- Scaling is one command. A new Resume Processing replica joins the queue's consumers without
  anyone being told (NFR-07).
- An instance that cannot do its job stops receiving work and rejoins when it recovers, with no
  retry logic in its callers.
- A Pod that dies is replaced under the same name; callers notice nothing.
- The private network ADR-010 relies on is concrete: only the gateway and the web app have a
  route from outside.

**What it costs us**

- **The team runs and learns Kubernetes** — the schedule risk ADR-001 named, now paid in
  manifests for eleven workloads and a script that builds them into a cluster.
- **Balancing is per connection, not per call.** kube-proxy chooses a Pod when a connection
  opens. HTTP keep-alive, and above all gRPC's single long-lived connection, keep a caller on one
  Pod. So the AI Service runs one replica; scaling it would need a headless Service with
  balancing in the client, or a proxy that balances calls.
- **Readiness is only as good as each service's check.** Hiring serves only a liveness endpoint,
  so a Hiring Pod that has lost its database still receives requests until it has a readiness
  check of its own.
- **Development does not exercise health checking.** Compose returns unhealthy containers, so a
  fault that readiness would hide on the cluster still reaches callers locally.
- **The development stack publishes more than a deployment may.** Compose publishes the services'
  and databases' ports for tests and tools; only the cluster manifests follow the
  publish-only-the-gateway rule, so a deployment must be built from them.
- **Startup order is not guaranteed.** A service that needs its database at startup restarts until
  the database is ready; the script applies the datastores first, but after a full restart Pods
  can cycle for a while.
- **A local cluster is heavy for a laptop.** kind keeps a copy of every image inside each node,
  on top of Docker's own; a first attempt with three nodes ran out of disk.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — decided Kubernetes discovery; this record carries
  it out and answers the risk it named. It corrects one claim: kube-proxy balances *connections*
  across ready Pods, not calls.
- [ADR-005](ADR-005-per-service-language.md) — every service runs the same way, with a readiness
  endpoint and a Kubernetes manifest. Every workload now has a manifest; Hiring still lacks the
  readiness endpoint.
- [ADR-006](ADR-006-single-tenant-deployment.md) — one company per deployment, hence one
  namespace.
- [ADR-007](ADR-007-grpc-for-the-ai-service.md) — gRPC to the AI Service, which per-connection
  balancing holds to one replica.
- [ADR-010](ADR-010-verification-at-the-gateway-only.md) — the gateway alone is published; the
  manifests enforce it on the cluster.
- [ADR-013](ADR-013-rabbitmq-for-the-screening-handover.md) — the broker takes discovery out of the
  screening handover.

### Related requirements

- **Course requirements:** describe how service discovery is managed; an API gateway; one service
  each on REST, gRPC and a message broker.
- **NFR-07** — screening throughput rises in proportion to workers, the quality attribute the
  project demonstrates: a new worker needs no registration.
- **NFR-09** — available 99% of working hours: failed Pods are replaced, and traffic is kept off
  instances that are not ready.
- **UC-0 to UC-3** — signing in, creating a job opening, batch screening and interview questions:
  every call they make crosses this mechanism.

### Related artifacts

The architecture diagram and its service-discovery view, the Service–Operations–Collaborators
table, and the deployment manifests.

### Related principles

- *Easily reversible* — services hold names, not mechanisms, so the platform underneath can change
  without touching their code.

---

## Notes

Two behaviours of Docker's embedded DNS were checked with throwaway containers on 2026-10-31,
because the argument against Compose rests on them: it returns every replica of a scaled service,
and it still returns a container whose health check has failed.

ADR-001's assumption that the team can run a cluster was first tried on a laptop with little free
disk. A three-node kind cluster started, but building the images for it exhausted the disk and
Docker stopped responding. The cluster was cut to one node, which shows the same mechanism; the
system has not yet run on it.
