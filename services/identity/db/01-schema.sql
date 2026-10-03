-- 01-schema.sql — the Identity Service's tables.
-- Runs automatically the first time the database container starts (see docker-compose.yml).

-- crypt() and gen_salt() hash the demo passwords in 02-seed.sql with bcrypt, the same algorithm
-- the service uses, so the seeded members can sign in once sign-in exists.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- One row per Member: a person given credentials to sign in, holding exactly one role.
CREATE TABLE IF NOT EXISTS members (
  id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  email               TEXT        NOT NULL CHECK (email = lower(email)),
  password_hash       TEXT        NOT NULL,                     -- bcrypt; the password itself is never stored
  temporary_password  BOOLEAN     NOT NULL DEFAULT true,        -- set by an Admin; must be replaced before signing in (FR-0.5)
  role                TEXT        NOT NULL CHECK (role IN ('admin', 'recruiter')),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  removed_at          TIMESTAMPTZ                               -- set when removed; the row stays (FR-6.2)
);

-- An email belongs to at most one active member. A removed member's email is free again, so a
-- person who returns can be added as a new member.
CREATE UNIQUE INDEX IF NOT EXISTS members_active_email ON members (email) WHERE removed_at IS NULL;
