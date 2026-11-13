#!/bin/sh
# Runs HireAssist on a local Kubernetes cluster (ADR-014): creates the kind cluster if it is not
# there, builds every service image and loads it into the cluster, creates the Secrets and the
# ConfigMap the manifests refer to, and applies the manifests — datastores first, then the
# services. Safe to run again: it rebuilds, reloads and replaces the running Pods.
#
#   deploy/k8s/up.sh                          then open http://localhost:30080
#   kind delete cluster --name hireassist     removes all of it, data included
#
# Credentials are the local development ones the Compose stack uses; never reuse them. As in
# docker-compose.yml, AI_SERVICE_PROVIDER, AI_SERVICE_API_KEY and AI_SERVICE_MODEL choose the AI
# Service's model; unset, its answers are canned.
set -eu
cd "$(dirname "$0")/../.."

CLUSTER=hireassist
NS=hireassist
IMAGES="hireassist/ai-service:dev hireassist/hiring:dev hireassist/resume-processing:2.0.0
hireassist/identity:dev hireassist/gateway:dev hireassist/web:dev"

say() { printf '\n\033[1m%s\033[0m\n' "$*"; }

say "1/5 cluster"
if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind create cluster --config deploy/k8s/kind-config.yaml
fi
kubectl config use-context "kind-$CLUSTER" >/dev/null

say "2/5 images"
build() { tag=$1; shift; echo "building $tag"; docker build -q -t "$tag" "$@" >/dev/null; }
build hireassist/ai-service:dev -f deploy/k8s/ai-service.Dockerfile .
build hireassist/hiring:dev services/hiring
build hireassist/resume-processing:2.0.0 -f services/resume-processing/Dockerfile .
build hireassist/identity:dev -f deploy/dev-keys/identity.Dockerfile .
build hireassist/gateway:dev -f deploy/dev-keys/gateway.Dockerfile .
build hireassist/web:dev services/web
# shellcheck disable=SC2086 # one word per image
kind load docker-image --name "$CLUSTER" $IMAGES

say "3/5 secrets and configuration"
kubectl apply -f deploy/k8s/manifests/00-namespace.yaml
# Create or update, so a second run leaves an unchanged Secret alone.
apply_secret() {
  name=$1
  shift
  kubectl -n "$NS" create secret generic "$name" "$@" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
}
apply_secret datastores \
  --from-literal=POSTGRES_PASSWORD=hireassist \
  --from-literal=IDENTITY_DB_PASSWORD=identity \
  --from-literal=RABBITMQ_USER=hireassist \
  --from-literal=RABBITMQ_PASSWORD=hireassist \
  --from-literal=MINIO_ROOT_USER=hireassist \
  --from-literal=MINIO_ROOT_PASSWORD=hireassist-local
apply_secret hiring \
  --from-literal=DATABASE_URL='postgres://hireassist:hireassist@postgres.hireassist.svc:5432/hiring?sslmode=disable' \
  --from-literal=MONGO_URL='mongodb://mongo.hireassist.svc:27017' \
  --from-literal=RABBITMQ_URL='amqp://hireassist:hireassist@rabbitmq.hireassist.svc:5672/' \
  --from-literal=OBJECT_STORE_ACCESS_KEY=hireassist \
  --from-literal=OBJECT_STORE_SECRET_KEY=hireassist-local
apply_secret resume-processing \
  --from-literal=MONGO_URL='mongodb://mongo.hireassist.svc:27017' \
  --from-literal=AMQP_URL='amqp://hireassist:hireassist@rabbitmq.hireassist.svc:5672' \
  --from-literal=S3_ACCESS_KEY=hireassist \
  --from-literal=S3_SECRET_KEY=hireassist-local
apply_secret identity \
  --from-literal=DATABASE_URL='postgres://identity:identity@identity-db.hireassist.svc:5432/identity?sslmode=disable'
apply_secret ai-service \
  --from-literal=AI_SERVICE_PROVIDER="${AI_SERVICE_PROVIDER:-canned}" \
  --from-literal=AI_SERVICE_API_KEY="${AI_SERVICE_API_KEY:-}" \
  --from-literal=AI_SERVICE_MODEL="${AI_SERVICE_MODEL:-}"
# The signing key pair is made once, so running this again does not sign everyone out: the
# private half for the Identity Service, the public half for the gateway (ADR-009).
if ! kubectl -n "$NS" get secret identity-signing-key >/dev/null 2>&1; then
  keys=$(mktemp -d)
  openssl ecparam -name prime256v1 -genkey -noout -out "$keys/key.pem"
  openssl ec -in "$keys/key.pem" -pubout -out "$keys/key.pub" 2>/dev/null
  apply_secret identity-signing-key --from-file=JWT_PRIVATE_KEY="$keys/key.pem"
  apply_secret gateway-verification-key --from-file=JWT_PUBLIC_KEYS="$keys/key.pub"
  rm -rf "$keys"
fi
kubectl -n "$NS" create configmap identity-db-init --from-file=services/identity/db/ \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

say "4/5 datastores and broker"
kubectl apply -f deploy/k8s/manifests/10-datastores.yaml
for s in postgres identity-db mongo rabbitmq minio; do
  kubectl -n "$NS" rollout status "statefulset/$s" --timeout=300s
done

say "5/5 services"
running=$(kubectl -n "$NS" get deployments -o name)
kubectl apply -f deploy/k8s/manifests/ -f services/resume-processing/k8s/
# The tags did not change but the images may have: replace the Pods that were already running.
if [ -n "$running" ]; then
  # shellcheck disable=SC2086 # one word per deployment
  kubectl -n "$NS" rollout restart $running
fi
for d in ai-service identity hiring resume-processing gateway web; do
  kubectl -n "$NS" rollout status "deployment/$d" --timeout=300s
done

say "HireAssist is running on Kubernetes"
kubectl -n "$NS" get pods -o wide
cat <<EOF

  web app   http://localhost:30080   somchai@acme.example / somchai-recruiter-password
  gateway   http://localhost:30800
  RabbitMQ  kubectl -n $NS port-forward svc/rabbitmq 15672, then http://localhost:15672

  The service-discovery walkthrough, one step at a time: deploy/k8s/demo.sh
EOF
