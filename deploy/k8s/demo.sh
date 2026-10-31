#!/bin/sh
# The service-discovery demonstration (ADR-014), one step at a time, for recording. Each step
# prints the command it is about to run and waits for Enter, so what is on screen is exactly what
# runs. It changes the cluster as it goes — scales Resume Processing, replaces one of its Pods,
# stops RabbitMQ for a moment — and puts everything back at the end.
#
#   deploy/k8s/demo.sh            after deploy/k8s/up.sh
#   deploy/k8s/demo.sh 6          start at step 6
set -eu
FROM=${1:-1}
n=0

step() {
  n=$((n + 1))
  title=$1
  shift
  [ "$n" -lt "$FROM" ] && return 0
  printf '\n\033[1;36m%s. %s\033[0m\n' "$n" "$title"
  for cmd in "$@"; do
    printf '\033[1m$ %s\033[0m ' "$cmd"
    read -r _ </dev/tty
    sh -c "$cmd" || true
  done
}

# The registry's own view of a Service: each Pod address behind it, and whether it is ready.
ENDPOINTS="kubectl -n hireassist get endpointslices -l kubernetes.io/service-name=resume-processing -o jsonpath='{range .items[*].endpoints[*]}{.addresses[0]}{\"  ready=\"}{.conditions.ready}{\"\\n\"}{end}'"

step "Every service is a Pod, and Pods come and go with their own IPs" \
  "kubectl -n hireassist get pods -o wide"

step "Each service has a stable name and a stable virtual IP. Only the gateway and the web app are published" \
  "kubectl -n hireassist get services"

step "Services are configured with names, never addresses" \
  "kubectl -n hireassist set env deployment/gateway --list | grep _URL" \
  "kubectl -n hireassist set env deployment/hiring --list | grep -E 'ADDRESS|RESUME_PROCESSING_URL|ENDPOINT'"

step "The cluster DNS turns a name into the Service's virtual IP" \
  "kubectl -n hireassist exec deploy/gateway -- nslookup hiring.hireassist.svc.cluster.local"

step "So the gateway reaches Hiring by name" \
  "kubectl -n hireassist exec deploy/gateway -- wget -qO- http://hiring.hireassist.svc:8080/healthz"

step "Behind the name: the Pods Kubernetes has registered for it" \
  "$ENDPOINTS"

step "Scale out. New Pods are registered by the platform; nobody is told about them" \
  "kubectl -n hireassist scale deployment/resume-processing --replicas=4" \
  "kubectl -n hireassist rollout status deployment/resume-processing" \
  "$ENDPOINTS" \
  "kubectl -n hireassist exec rabbitmq-0 -- rabbitmqctl list_queues name consumers"

step "Replace a Pod. It comes back with a new IP under the same name" \
  "kubectl -n hireassist delete \$(kubectl -n hireassist get pods -l app=resume-processing -o name | head -n 1)" \
  "kubectl -n hireassist rollout status deployment/resume-processing" \
  "$ENDPOINTS"

step "Readiness decides membership. Stop the broker, and Resume Processing stops being ready" \
  "kubectl -n hireassist scale statefulset/rabbitmq --replicas=0" \
  "kubectl -n hireassist wait --for=condition=Ready=false pod -l app=resume-processing --timeout=120s" \
  "$ENDPOINTS"

step "Bring the broker back, and they rejoin by themselves" \
  "kubectl -n hireassist scale statefulset/rabbitmq --replicas=1" \
  "kubectl -n hireassist wait --for=condition=Ready pod -l app=resume-processing --timeout=180s" \
  "$ENDPOINTS"

step "Put things back as they were" \
  "kubectl -n hireassist scale deployment/resume-processing --replicas=2"
