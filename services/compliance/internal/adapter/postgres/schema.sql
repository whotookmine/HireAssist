-- The Compliance & Insights Service's own database (ADR-003: PostgreSQL is the system of record).
-- Applied on every start; every statement is idempotent.

-- The deployment's one retention policy (FR-5.1). The check keeps it to one row.
CREATE TABLE IF NOT EXISTS retention_policy (
    id                  boolean PRIMARY KEY DEFAULT true CHECK (id),
    period_days         integer NOT NULL CHECK (period_days > 0),
    anchor              text    NOT NULL CHECK (anchor IN ('collected', 'last_activity')),
    action              text    NOT NULL CHECK (action IN ('delete', 'anonymise')),
    warning_window_days integer NOT NULL CHECK (warning_window_days >= 0),
    updated_at          timestamptz NOT NULL
);

-- The projection of job openings, built from pipeline events (UC-4).
CREATE TABLE IF NOT EXISTS openings (
    id                         text PRIMARY KEY,
    title                      text        NOT NULL DEFAULT '',
    status                     text        NOT NULL CHECK (status IN ('open', 'paused', 'closed')),
    opened_at                  timestamptz,
    expected_time_to_fill_days integer     NOT NULL DEFAULT 0,
    recruiter                  text        NOT NULL DEFAULT '',
    flagged_at                 timestamptz
);

-- One row per resume per stage reached. The key makes a redelivered event harmless. The
-- candidate id is cleared when the candidate is erased; the row, and so the count, stays.
CREATE TABLE IF NOT EXISTS pipeline_entries (
    opening_id   text NOT NULL REFERENCES openings (id),
    stage        text NOT NULL CHECK (stage IN ('applied', 'screened', 'shortlisted')),
    resume_id    text NOT NULL,
    candidate_id text,
    occurred_at  timestamptz NOT NULL,
    PRIMARY KEY (opening_id, stage, resume_id)
);
CREATE INDEX IF NOT EXISTS pipeline_entries_candidate ON pipeline_entries (candidate_id);

-- Expiries held for a Recruiter's decision (FR-5.6).
CREATE TABLE IF NOT EXISTS retention_holds (
    candidate_id   text PRIMARY KEY,
    job_opening_id text NOT NULL DEFAULT '',
    expires_at     timestamptz NOT NULL,
    held_at        timestamptz NOT NULL
);

-- What the sweep last did about a candidate: the expiry it warned about (so it warns once,
-- FR-5.2) and how long a "keep" decision lasts.
CREATE TABLE IF NOT EXISTS retention_marks (
    candidate_id text PRIMARY KEY,
    warned_for   timestamptz,
    keep_until   timestamptz
);

-- Erasures, which are also the erasure audit log (FR-5.5). candidate_id is set only while the
-- erasure is pending; the reference is random and resolves to nobody once it is cleared.
CREATE TABLE IF NOT EXISTS erasures (
    ref           text PRIMARY KEY,
    candidate_id  text,
    reason        text        NOT NULL,
    action        text        NOT NULL,
    policy_days   integer     NOT NULL,
    requested_at  timestamptz NOT NULL,
    completed_at  timestamptz,
    status        text        NOT NULL CHECK (status IN ('pending', 'erased')),
    done          text[]      NOT NULL DEFAULT '{}',
    attempts      integer     NOT NULL DEFAULT 0,
    last_error    text        NOT NULL DEFAULT '',
    last_tried_at timestamptz,
    CHECK (status = 'erased' OR candidate_id IS NOT NULL),
    CHECK (status = 'pending' OR candidate_id IS NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS erasures_one_pending ON erasures (candidate_id) WHERE status = 'pending';

-- The access audit log (FR-0.4, NFR-12): requests the gateway refused for want of a role.
CREATE TABLE IF NOT EXISTS access_rejections (
    id            text PRIMARY KEY,
    member_id     text        NOT NULL DEFAULT '',
    role          text        NOT NULL DEFAULT '',
    required_role text        NOT NULL DEFAULT '',
    method        text        NOT NULL,
    path          text        NOT NULL,
    occurred_at   timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS access_rejections_time ON access_rejections (occurred_at DESC);
