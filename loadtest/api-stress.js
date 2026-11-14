// Load test 2 — stress (k6 test type "stress").
//
// The same recruiter session as the average-load test, but the number of recruiters climbs past
// the design load of 20 (NFR-08) to 50, 100 and 200. The question is where the API starts to
// slow down or fail, and which part gives way first.
//
//   k6 run loadtest/api-stress.js
import { recruiterJourney, recruiterTokens, createOpening, seedScreenedCandidates } from './lib/hireassist.js';
import { RESUMES } from './lib/resumes.js';

const PEAK = Number(__ENV.PEAK || 200);

export const options = {
  setupTimeout: '15m',
  scenarios: {
    recruiters: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 20 }, // the design load
        { duration: '1m', target: 20 },
        { duration: '30s', target: 50 },
        { duration: '1m', target: 50 },
        { duration: '30s', target: 100 },
        { duration: '1m', target: 100 },
        { duration: '30s', target: PEAK },
        { duration: '1m', target: PEAK },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
    },
  },
  // The same targets as the average-load test. Here a breach is a finding, not a failure.
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{name:ranked results}': ['p(95)<2000'],
    'http_req_duration{name:submit batch}': ['p(95)<2000'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export function setup() {
  const tokens = recruiterTokens(PEAK);
  const opening = createOpening(tokens[0], 'Load test — stress');
  seedScreenedCandidates(tokens[0], opening, 30, RESUMES, 600);
  return { tokens, opening };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  recruiterJourney(token, data.opening, RESUMES, 0.1);
}
