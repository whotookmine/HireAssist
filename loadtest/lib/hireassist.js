// Shared helpers for the HireAssist load tests. Every call goes through the API Gateway, the
// way the web app calls it, so a result measures the gateway, the token check and the service
// behind it together.
import http from 'k6/http';
import { check, fail, group, sleep } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8000';

// The Identity Service's synthetic seed admin (docker-compose.yml lists the demo sign-ins).
const ADMIN = { email: 'nok@acme.example', password: 'nok-admin-demo-password' };
// Load-test recruiters are created by the admin like any member (UC-6), with a temporary
// password they replace before signing in (FR-0.5). Both are at least 15 characters.
const TEMP_PASSWORD = 'loadtest-temporary-password';
const RECRUITER_PASSWORD = 'loadtest-recruiter-password';

const JSON_HEADERS = { 'Content-Type': 'application/json' };

// Setup expects some refusals — a sign-in probe for a member not created yet, a 429 it waits out
// — so they must not count towards http_req_failed, which measures the test itself.
const SETUP_OK = (...codes) => http.expectedStatuses(...codes, 429);

export function auth(token, name) {
  return {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    tags: name ? { name } : {},
  };
}

// The gateway limits each caller to 20 requests a second (burst 40). Before sign-in the caller
// is the client address, which every virtual user here shares — so setup waits out a 429
// instead of failing. The measured part of a test never retries: a 429 there is a result.
function patient(send) {
  for (let attempt = 0; attempt < 30; attempt++) {
    const res = send();
    if (res.status !== 429) return res;
    sleep(0.5);
  }
  fail('the gateway kept rate-limiting setup after 30 attempts');
}

function signIn(email, password) {
  return patient(() =>
    http.post(`${BASE_URL}/auth/sign-in`, JSON.stringify({ email, password }), {
      headers: JSON_HEADERS,
      tags: { name: 'setup: sign in' },
      responseCallback: SETUP_OK(200, 401, 403),
    }),
  );
}

// Returns an access token for each of loadtest-recruiter-1 … n, creating the members first if
// they do not exist yet. Each virtual user signs in as its own recruiter, because the gateway's
// rate limit is per member: twenty people are twenty buckets, not one.
export function recruiterTokens(n) {
  const admin = signIn(ADMIN.email, ADMIN.password);
  if (admin.status !== 200) fail(`admin sign-in failed: ${admin.status} ${admin.body}`);
  const adminToken = admin.json('access_token');

  const tokens = [];
  for (let i = 1; i <= n; i++) {
    const email = `loadtest-recruiter-${i}@acme.example`;
    let res = signIn(email, RECRUITER_PASSWORD);
    if (res.status !== 200) {
      // 401: not created yet. 403: created, but the temporary password is still in place.
      const created = patient(() =>
        http.post(
          `${BASE_URL}/members`,
          JSON.stringify({ email, password: TEMP_PASSWORD, role: 'recruiter' }),
          { ...auth(adminToken, 'setup: create member'), responseCallback: SETUP_OK(201, 409) },
        ),
      );
      if (created.status !== 201 && created.status !== 409) {
        fail(`creating ${email}: ${created.status} ${created.body}`);
      }
      const changed = patient(() =>
        http.post(
          `${BASE_URL}/auth/change-password`,
          JSON.stringify({ email, current_password: TEMP_PASSWORD, new_password: RECRUITER_PASSWORD }),
          { headers: JSON_HEADERS, tags: { name: 'setup: change password' }, responseCallback: SETUP_OK(200) },
        ),
      );
      if (changed.status !== 200) fail(`replacing the temporary password of ${email}: ${changed.status} ${changed.body}`);
      res = signIn(email, RECRUITER_PASSWORD);
      if (res.status !== 200) fail(`signing in as ${email}: ${res.status} ${res.body}`);
    }
    tokens.push(res.json('access_token'));
  }
  return tokens;
}

export function createOpening(token, title) {
  const res = patient(() =>
    http.post(
      `${BASE_URL}/job-openings`,
      JSON.stringify({
        title,
        description: 'Synthetic opening created by the HireAssist load test.',
        criteria: [
          { name: 'Production backend experience in Go or Node.js', classification: 'must-have', weight: 50 },
          { name: 'Relational data modelling and SQL', classification: 'nice-to-have', weight: 30 },
          { name: 'Operating distributed systems', classification: 'nice-to-have', weight: 20 },
        ],
        expected_time_to_fill_days: 30,
      }),
      { ...auth(token, 'setup: create opening'), responseCallback: SETUP_OK(201) },
    ),
  );
  if (res.status !== 201) fail(`creating an opening: ${res.status} ${res.body}`);
  return res.json('id');
}

// multipart/form-data with one "resumes" part per file, which is what the Hiring Service reads.
// Built by hand because a k6 form object cannot repeat a field name.
function multipart(files) {
  const boundary = `----hireassist-loadtest-${Math.random().toString(16).slice(2)}`;
  const ascii = (s) => {
    const out = new Uint8Array(s.length);
    for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
    return out;
  };
  const parts = [];
  for (const f of files) {
    parts.push(ascii(`--${boundary}\r\nContent-Disposition: form-data; name="resumes"; filename="${f.name}"\r\n`));
    parts.push(ascii('Content-Type: application/pdf\r\n\r\n'));
    parts.push(new Uint8Array(f.data));
    parts.push(ascii('\r\n'));
  }
  parts.push(ascii(`--${boundary}--\r\n`));
  const body = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  for (const p of parts) {
    body.set(p, offset);
    offset += p.length;
  }
  return { body: body.buffer, contentType: `multipart/form-data; boundary=${boundary}` };
}

// Submits `count` resumes, cycling through the synthetic demo PDFs under distinct file names.
export function submitBatch(token, openingId, count, resumes, name) {
  const files = [];
  for (let i = 0; i < count; i++) {
    const r = resumes[i % resumes.length];
    files.push({ name: `loadtest-${String(i + 1).padStart(3, '0')}-${r.name}`, data: r.data });
  }
  const { body, contentType } = multipart(files);
  return http.post(`${BASE_URL}/job-openings/${openingId}/batches`, body, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': contentType },
    tags: { name: name || 'submit batch' },
    timeout: '120s',
  });
}

export function batchStatus(token, openingId, batchId) {
  return http.get(`${BASE_URL}/job-openings/${openingId}/batches/${batchId}`, auth(token, 'batch status'));
}

// Submits a batch and waits for every resume in it to finish, for a test that needs screened
// candidates to read. Setup only.
export function seedScreenedCandidates(token, openingId, count, resumes, maxSeconds) {
  const res = submitBatch(token, openingId, count, resumes, 'setup: submit batch');
  if (res.status !== 202) fail(`seeding a batch: ${res.status} ${res.body}`);
  const batchId = res.json('id');
  const deadline = Date.now() + maxSeconds * 1000;
  while (Date.now() < deadline) {
    const st = patient(() =>
      http.get(`${BASE_URL}/job-openings/${openingId}/batches/${batchId}`, {
        ...auth(token, 'setup: batch status'),
        responseCallback: SETUP_OK(200),
      }),
    );
    if (st.status === 200 && st.json('complete')) return;
    sleep(2);
  }
  fail(`the seed batch did not complete within ${maxSeconds}s`);
}

// One recruiter's working session, used by the average-load and the stress test alike: look at
// the openings, open one, read its ranked candidates (NFR-03), open a candidate's justification,
// and now and then submit a small batch (NFR-04).
export function recruiterJourney(token, openingId, resumes, batchShare) {
  group('browse', () => {
    const list = http.get(`${BASE_URL}/job-openings`, auth(token, 'list openings'));
    check(list, { 'list openings 200': (r) => r.status === 200 });

    const one = http.get(`${BASE_URL}/job-openings/${openingId}`, auth(token, 'get opening'));
    check(one, { 'get opening 200': (r) => r.status === 200 });

    const ranked = http.get(`${BASE_URL}/job-openings/${openingId}/results`, auth(token, 'ranked results'));
    check(ranked, { 'ranked results 200': (r) => r.status === 200 });

    if (ranked.status === 200) {
      const rows = ranked.json('ranked') || [];
      if (rows.length > 0) {
        const pick = rows[Math.floor(Math.random() * rows.length)];
        const detail = http.get(
          `${BASE_URL}/job-openings/${openingId}/results/${pick.resume_id}`,
          auth(token, 'candidate justification'),
        );
        check(detail, { 'candidate justification 200': (r) => r.status === 200 });
      }
    }
  });

  if (Math.random() < batchShare) {
    group('submit', () => {
      const res = submitBatch(token, openingId, 3, resumes, 'submit batch');
      check(res, { 'submit batch 202': (r) => r.status === 202 });
      if (res.status === 202) {
        const st = batchStatus(token, openingId, res.json('id'));
        check(st, { 'batch status 200': (r) => r.status === 200 });
      }
    });
  }

  sleep(1 + Math.random() * 2); // think time between screens
}
