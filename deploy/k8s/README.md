# HireAssist on Kubernetes

The whole system on a local Kubernetes cluster, where the services find each other through
Kubernetes Services and the cluster DNS (ADR-014). Day-to-day development stays on the root
`docker-compose.yml`; this is for what Compose cannot show — readiness deciding which instances
receive traffic, and a deployment that publishes nothing but the web app and the gateway.

## Run it

Needs Docker, [kind](https://kind.sigs.k8s.io/) and `kubectl`, and room for Docker to grow by
about 8 GB: kind keeps its own copy of every image inside the cluster's node.

```sh
deploy/k8s/up.sh                          # create the cluster, build, load, apply, wait
open http://localhost:30080               # somchai@acme.example / somchai-recruiter-password
kind delete cluster --name hireassist     # remove all of it, data included
```

`up.sh` is safe to run again after changing a service: it rebuilds the images, loads them and
replaces the running Pods. As with Compose, `AI_SERVICE_PROVIDER`, `AI_SERVICE_API_KEY` and
`AI_SERVICE_MODEL` choose the AI Service's model; unset, its answers are canned.

## What is here

| File | Holds |
|---|---|
| `kind-config.yaml` | A one-node cluster, with the web app's and the gateway's NodePorts mapped to `localhost:30080` and `localhost:30800` |
| `manifests/00-namespace.yaml` | The `hireassist` namespace: one company's deployment (ADR-006) |
| `manifests/10-datastores.yaml` | PostgreSQL ×2, MongoDB, RabbitMQ and MinIO, each a StatefulSet with a volume |
| `manifests/20-…50-*.yaml` | The AI Service, Hiring, Identity, the gateway and the web app |
| `../../services/resume-processing/k8s/` | Resume Processing's manifest, kept with its service |
| `ai-service.Dockerfile` | An image for the AI Service, which Compose runs from source instead |
| `up.sh` | Brings it all up; creates the Secrets and the signing key pair |
| `demo.sh` | The service-discovery walkthrough, one step per Enter, for recording |

The images for the gateway and Identity are the Compose stack's (`deploy/dev-keys/`), started
without their key wrapper: here the keys come from Secrets.

## How the services find each other

Each workload is a Deployment or StatefulSet behind a Service of the same name, and each service
reads the names it calls from environment variables — `HIRING_URL=http://hiring.hireassist.svc:8080`,
`AI_SERVICE_ADDRESS=ai-service.hireassist.svc:50051`. The cluster DNS turns a name into the
Service's virtual IP, and kube-proxy passes the connection to a Pod that passes its readiness
probe. The architecture page has the diagram and the full list of names; ADR-014 has the reasons.

## The walkthrough

`demo.sh` prints each command before running it and waits for Enter; `demo.sh 6` starts at step
6. It shows, in order: the Pods and their changing IPs; the Services and their stable virtual
IPs, with only the gateway and the web app published; the names in the services' configuration;
a DNS lookup from inside the gateway; the Pods registered behind Resume Processing; scaling to
four replicas, which the Service and the broker's consumer count both pick up; a replaced Pod
returning under the same name with a new IP; and readiness at work — RabbitMQ stopped, Resume
Processing dropping out of its Service, and rejoining when the broker returns. It puts everything
back at the end.

## Known gaps

- **Not yet run end to end.** These files were written on 2026-10-31 without a working cluster —
  the first attempt filled the laptop's disk — so the first run may need fixes.
- **Hiring has no readiness check of its own** — its probe is its liveness endpoint, so a Hiring
  Pod that has lost its database still receives requests.
- **The AI Service runs one replica.** Its callers each hold one long-lived gRPC connection, which
  a ClusterIP Service balances per connection, not per call (ADR-014).
- **Startup order is not guaranteed.** `up.sh` applies the datastores first; after a full restart
  of the cluster, services that need their database at startup restart until it is ready.
