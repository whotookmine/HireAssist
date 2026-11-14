# Load tests

Three [k6](https://grafana.com/docs/k6/latest/) tests against the whole system, run through the
API Gateway the way the web app calls it. Two are load tests of different types; the third is the
measurement behind the quality attribute this project demonstrates, Scalability.

| Script | k6 test type | Question it answers | Targets |
|---|---|---|---|
| `api-load.js` | average-load | Does the everyday API stay inside its targets at the design load of 20 recruiters? | NFR-03, NFR-04, NFR-08 |
| `api-stress.js` | stress | Past the design load — 50, 100, 200 recruiters — where does the API slow down or fail first? | the same targets, read as findings |
| `scalability.js` + `run-scalability.sh` | comparative (Scalability) | Does screening throughput rise in proportion to Resume Processing replicas? | NFR-06, NFR-07 |

## The stack under test

`compose.loadtest.yml` layers two changes on the root `docker-compose.yml`, and nothing else
differs from the demo stack:

- **The AI Service's canned provider waits 6 seconds a call** (`AI_SERVICE_CANNED_LATENCY_S`). A
  resume takes two calls, so twelve seconds — the figure the NFR estimates assume. A real model
  is not used: its rate limit and its bill would set the result rather than our architecture,
  which is the thing under test. This is the main caveat on every number: the model's own
  variance is not in them.
- **Resume Processing publishes no host port**, so it can run as several replicas.

```sh
docker compose -f docker-compose.yml -f loadtest/compose.loadtest.yml up -d --build
```

## Running them

```sh
k6 run --out json=load-test-results/api-load.json --summary-export=load-test-results/api-load-summary.json loadtest/api-load.js
k6 run --out json=load-test-results/api-stress.json --summary-export=load-test-results/api-stress-summary.json loadtest/api-stress.js
sh loadtest/run-scalability.sh
```

Setup creates the recruiters it needs (`loadtest-recruiter-N@acme.example`) through the admin, as
UC-6 would, and every virtual user signs in as its own recruiter: the gateway limits each member to
20 requests a second, so one shared token would measure the rate limiter instead of the system.
Results go to `load-test-results/`, which Git ignores; the numbers worth keeping are written up in
`docs/LOAD-TEST.md`.

`docker compose down -v` afterwards discards the test members, openings and resumes with the rest
of the data.
