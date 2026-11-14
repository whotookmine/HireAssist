// Load test 1 — average load (k6 test type "average-load").
//
// Twenty recruiters (NFR-08) work at once for two minutes: browsing openings, reading ranked
// candidates and justifications, and submitting small batches. The question is whether the
// everyday API stays inside its targets at the load the system is designed for.
//
//   k6 run loadtest/api-load.js
import { recruiterJourney, recruiterTokens, createOpening, seedScreenedCandidates } from './lib/hireassist.js';
import { RESUMES } from './lib/resumes.js';

const RECRUITERS = Number(__ENV.RECRUITERS || 20);

export const options = {
  setupTimeout: '10m',
  scenarios: {
    recruiters: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: RECRUITERS }, // ramp up
        { duration: '2m', target: RECRUITERS }, // hold at the design load
        { duration: '30s', target: 0 }, // ramp down
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    'http_req_duration{name:ranked results}': ['p(95)<2000'], // NFR-03
    'http_req_duration{name:submit batch}': ['p(95)<2000'], // NFR-04
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export function setup() {
  const tokens = recruiterTokens(RECRUITERS);
  const opening = createOpening(tokens[0], 'Load test — average load');
  // Something to rank: 30 screened candidates before the clock starts.
  seedScreenedCandidates(tokens[0], opening, 30, RESUMES, 600);
  return { tokens, opening };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  recruiterJourney(token, data.opening, RESUMES, 0.1);
}
