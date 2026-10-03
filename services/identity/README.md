# Identity Service

Owns the **members** of one HireAssist installation and their roles: UC-0 (sign in) and UC-6
(manage members and roles). Go with the standard library's `net/http`, PostgreSQL through `pgx`.

**Status:** member CRUD works. Signing in, the JWT access token and refresh tokens (the session
ADR, still proposed) are not built yet, so **every endpoint is open**. Run it only on your own
machine.

| | |
|---|---|
| API | http://localhost:8081 |
| Database | PostgreSQL 16 in Docker, `localhost:5433`, database `identity` |
| Owner | Thanwarat Korcharoenkiat |

## Run it

Needs Docker Desktop running and Go 1.27 or newer. `jq` is needed only for `make demo`.

```sh
make db-up      # start PostgreSQL; the first start creates the table and 2 demo members
make run        # start the service — leave this terminal open
```

In a second terminal:

```sh
make db-watch   # the members table, refreshed every 2 seconds (Ctrl+C stops)
make db-reset   # back to the 2 demo members — run before each recording
make demo       # every call in order with curl, Enter between steps
```

`make` alone lists every command. `make db-down` stops the database and keeps the data;
`docker compose down -v` wipes it.

## Endpoints

| Operation | Method and path | Success | Errors |
|---|---|---|---|
| `listMembers()` | `GET /members` | 200, active members only | |
| `getMember()` | `GET /members/{id}` | 200, removed members too | 400 id not a UUID · 404 |
| `createMember()` | `POST /members` | 201 and `Location` | 400 · 409 email already active |
| `changeMemberRole()` | `PATCH /members/{id}` | 200 | 400 · 404 · 409 removed or last admin |
| `setMemberPassword()` | `PUT /members/{id}/password` | 200 | 400 · 404 · 409 removed |
| `removeMember()` | `DELETE /members/{id}` | 204, no body | 400 · 404 · 409 removed or last admin |
| liveness | `GET /healthz` | 200 while the process runs | |
| readiness | `GET /readyz` | 200 while the database answers | 503 |

Every error is JSON, `{"error": "...", "details": [...]}`. `details` lists every validation
problem at once.

```json
{
  "id": "143ae1e8-9733-4d17-9ced-5075459ab63b",
  "email": "ploy@acme.example",
  "role": "recruiter",
  "temporary_password": true,
  "created_at": "2026-10-02T18:47:54.659135+07:00",
  "updated_at": "2026-10-02T18:47:54.659135+07:00",
  "removed_at": null
}
```

`POST /members` takes `email`, `password` and `role`. `PATCH /members/{id}` takes `role` only.
`PUT /members/{id}/password` takes `password`.

## Rules

| Rule | Why |
|---|---|
| An email is trimmed and lower-cased, and belongs to at most one active member | `Ploy@Acme.example` and `ploy@acme.example` are the same person |
| A password is at least 15 characters and at most 72 bytes; no forced capitals, digits or symbols | NIST's current guidance for a password that is the only factor; bcrypt reads only 72 bytes |
| A password an Admin sets is temporary | the member must replace it before signing in (FR-0.5), once sign-in exists |
| Removing a member keeps the row, marked removed | past actions must still name the person (FR-6.2) |
| The last active admin cannot be removed or demoted | nothing in the product could create an admin again (FR-6.3); changes that could do it run one at a time, so two admins demoting each other cannot both succeed |
| Only `role` can be changed with PATCH; email is fixed | FR-6.1 |

## Demo members

Synthetic, from `db/02-seed.sql`, with fixed ids so every recording starts the same.

| Email | Role | Password | id |
|---|---|---|---|
| nok@acme.example | admin | nok-admin-demo-password | `00000000-0000-4000-8000-000000000001` |
| somchai@acme.example | recruiter | somchai-recruiter-password | `00000000-0000-4000-8000-000000000002` |

## Recording the CRUD demo (under 5 minutes)

**Before pressing record:** `make db-up`, then `make run` in one terminal, then `make db-reset`
and `make db-watch` in a second. Import `HireAssist-Identity.postman_collection.json` into
Postman (File → Import). Put Postman in the top two-thirds of the screen and the `db-watch`
terminal below it, so the table refreshes by itself after every call.

| Time | Send | Point out |
|---|---|---|
| 0:00 | — | the `make run` terminal: the service is up on 8081 and connected to PostgreSQL |
| 0:20 | 01 READ ALL | 200, the two demo members, the same rows as the table below |
| 0:45 | 02 CREATE | 201, the email stored lower-case, `temporary_password` true, a new row in the table |
| 1:20 | 03 and 04 | 400 lists every problem at once; 409 for an email that already belongs to someone |
| 1:50 | 05 READ ONE | 200, one member by id |
| 2:10 | 06 UPDATE role | 200, the role in the table turns to admin and `updated` moves |
| 2:40 | 07 UPDATE password | 200, the password is temporary again |
| 3:00 | 08 DELETE | 204; the row stays, with `removed` filled in |
| 3:20 | 09 and 10 | the removed member can still be read by id, but is gone from the list |
| 3:50 | 11 DELETE last admin | 409, the installation must keep an admin |
| 4:15 | — | the `make run` terminal: one JSON log line per request, with its status |

Each Postman request carries tests, so the Test Results tab turns green when the status is right.

## Files

| File | What it is |
|---|---|
| `main.go` | Configuration, the database connection, the routes, and a graceful shutdown |
| `members.go` | The six member handlers, the readiness probe, and the error-to-status mapping |
| `store.go` | The `Member` type and one SQL function per operation |
| `validate.go` | The rules for emails, passwords, roles and ids, and password hashing |
| `http.go` | JSON in and out, error bodies, and the request log |
| `db/01-schema.sql` | The `members` table; runs on the first database start |
| `db/02-seed.sql` | The two demo members; also what `make db-reset` runs |
| `docker-compose.yml` | The PostgreSQL container |
| `Makefile` | The commands above |
| `demo.sh` | The walkthrough behind `make demo` |
| `HireAssist-Identity.postman_collection.json` | The same calls for Postman, in demo order, each with tests |

## If something does not start

- `DATABASE_URL is not set` — start the service with `make run`, which passes it.
- `cannot reach PostgreSQL` — Docker Desktop is not running, or `make db-up` was not run.
- `address already in use` on 8081 — another copy is running; `PORT=8082 make run` moves it,
  and the `base` variable in Postman must move with it.
- The table is missing after `make db-up` — the init scripts only run on a fresh volume:
  `docker compose down -v && make db-up`.
