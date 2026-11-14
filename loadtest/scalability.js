// Load test 3 — the Scalability measurement (NFR-06, NFR-07): one burst of 100 resumes, timed
// from upload until the last resume is screened. run-scalability.sh repeats it at 1, 2 and 4
// Resume Processing replicas; throughput should rise in proportion to the replica count.
//
//   WORKERS=2 k6 run loadtest/scalability.js     (WORKERS only labels the result)
import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Counter, Gauge, Trend } from 'k6/metrics';
import { BASE_URL, auth, createOpening, recruiterTokens, submitBatch } from './lib/hireassist.js';
import { RESUMES } from './lib/resumes.js';

const BATCH_SIZE = Number(__ENV.BATCH_SIZE || 100);
const WORKERS = __ENV.WORKERS || 'unlabelled';

const ackTime = new Trend('batch_ack_time', true); // NFR-04: acknowledged within 2 s
const completionTime = new Trend('batch_completion_time', true); // NFR-06: 100 within 10 min
const finishedSoFar = new Gauge('batch_finished_so_far'); // the progress curve, one point a second
const throughput = new Gauge('screening_throughput_per_min');
const failedResumes = new Counter('resumes_failed');

export const options = {
  setupTimeout: '2m',
  scenarios: {
    burst: { executor: 'per-vu-iterations', vus: 1, iterations: 1, maxDuration: '25m' },
  },
  thresholds: {
    batch_ack_time: ['max<2000'],
    resumes_failed: ['count==0'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(95)'],
};

export function setup() {
  const [token] = recruiterTokens(1);
  const opening = createOpening(token, `Load test — ${BATCH_SIZE} resumes, ${WORKERS} worker(s)`);
  return { token, opening };
}

export default function ({ token, opening }) {
  const started = Date.now();
  const res = submitBatch(token, opening, BATCH_SIZE, RESUMES, 'submit batch');
  ackTime.add(res.timings.duration);
  if (!check(res, { 'batch accepted (202)': (r) => r.status === 202 })) fail(`submit: ${res.status} ${res.body}`);
  const batchId = res.json('id');

  let counts = null;
  for (;;) {
    sleep(1);
    const st = http.get(`${BASE_URL}/job-openings/${opening}/batches/${batchId}`, auth(token, 'batch status'));
    if (st.status !== 200) continue; // a slow poll is not the system under test
    counts = st.json('counts');
    finishedSoFar.add(counts.screened + counts.needs_review + counts.failed);
    if (st.json('complete')) break;
    if (Date.now() - started > 24 * 60 * 1000) fail('the batch did not complete within 24 minutes');
  }

  const seconds = (Date.now() - started) / 1000;
  completionTime.add(seconds * 1000);
  throughput.add((BATCH_SIZE / seconds) * 60);
  failedResumes.add(counts.failed);
  console.log(
    JSON.stringify({
      workers: WORKERS,
      batch: BATCH_SIZE,
      seconds: Math.round(seconds * 10) / 10,
      per_minute: Math.round((BATCH_SIZE / seconds) * 600) / 10,
      ack_ms: Math.round(res.timings.duration),
      screened: counts.screened,
      needs_review: counts.needs_review,
      failed: counts.failed,
    }),
  );
}
