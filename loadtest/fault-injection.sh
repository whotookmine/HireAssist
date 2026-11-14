#!/bin/sh
# Fault injection (NFR-10, ADR-002's second test): stop the AI Service in the middle of a batch,
# bring it back, and count what was lost. Screening should slow down, retry and finish with no
# resume lost — or, past the retry budget, end visibly as failed rather than silently pending.
#
#   OUTAGE=20 BATCH_SIZE=40 sh loadtest/fault-injection.sh
set -eu
cd "$(dirname "$0")/.."

COMPOSE="docker compose -f docker-compose.yml -f loadtest/compose.loadtest.yml"
OUT=load-test-results
OUTAGE=${OUTAGE:-20}
BATCH_SIZE=${BATCH_SIZE:-40}
mkdir -p "$OUT"

since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
BATCH_SIZE=$BATCH_SIZE WORKERS="fault-$OUTAGE" k6 run --quiet \
  --out json="$OUT/fault-injection.json" \
  --summary-export="$OUT/fault-injection-summary.json" \
  loadtest/scalability.js > "$OUT/fault-injection.log" 2>&1 &
k6pid=$!

sleep 10
echo "$(date +%T) stopping the AI Service for ${OUTAGE}s"
$COMPOSE stop ai-service >/dev/null 2>&1
sleep "$OUTAGE"
echo "$(date +%T) starting the AI Service again"
$COMPOSE start ai-service >/dev/null 2>&1

wait "$k6pid" || true
echo "$(date +%T) batch finished"
grep -E 'level=info msg="\{' "$OUT/fault-injection.log" | sed 's/.*msg="//; s/" source=console//; s/\\"/"/g'
retries=$($COMPOSE logs --since "$since" resume-processing 2>/dev/null | grep -c "will retry" || true)
dead=$($COMPOSE logs --since "$since" resume-processing 2>/dev/null | grep -c "dead-letter" || true)
echo "retries scheduled: $retries, sent to the dead-letter queue: $dead"
