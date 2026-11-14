#!/bin/sh
# The Scalability measurement (NFR-06, NFR-07): the same 100-resume burst at 1, 2 and 4 Resume
# Processing replicas, then at 4 replicas once more with the AI Service's worker pool raised
# from its default of 10 to 32 — the run that shows what the default pool caps.
#
#   sh loadtest/run-scalability.sh          (the stack must be up with loadtest/compose.loadtest.yml)
#
# Each run writes its log, its k6 time series (--out json) and its end-of-test summary to
# load-test-results/, which Git ignores.
set -eu
cd "$(dirname "$0")/.."

COMPOSE="docker compose -f docker-compose.yml -f loadtest/compose.loadtest.yml"
OUT=load-test-results
mkdir -p "$OUT"

# Waits until the AI Service in the current container has logged that it is listening.
wait_for_ai() {
  for _ in $(seq 1 120); do
    if docker logs "$($COMPOSE ps -q ai-service)" 2>&1 | grep -q "AI Service listening"; then return 0; fi
    sleep 1
  done
  echo "the AI Service did not start" >&2
  exit 1
}

# Waits until screening.requested has exactly $1 consumers and no messages left over.
wait_for_consumers() {
  for _ in $(seq 1 120); do
    line=$($COMPOSE exec -T rabbitmq rabbitmqctl list_queues -q name consumers messages 2>/dev/null |
      awk '$1 == "screening.requested" { print $2, $3 }')
    if [ "$line" = "$1 0" ]; then return 0; fi
    sleep 1
  done
  echo "screening.requested never reached $1 consumers with an empty queue (last: $line)" >&2
  exit 1
}

run() { # replicas, AI Service workers, label
  echo "── $3: $1 Resume Processing replica(s), AI Service max_workers=$2"
  AI_SERVICE_MAX_WORKERS=$2 $COMPOSE up -d --scale resume-processing="$1" ai-service resume-processing >/dev/null 2>&1
  wait_for_ai
  wait_for_consumers "$1"
  WORKERS="$1" k6 run --quiet \
    --out json="$OUT/scalability-$3.json" \
    --summary-export="$OUT/scalability-$3-summary.json" \
    loadtest/scalability.js 2>&1 | tee "$OUT/scalability-$3.log"
}

run 1 10 1-replica
run 2 10 2-replicas
run 4 10 4-replicas
run 4 32 4-replicas-ai32
