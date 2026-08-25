BEGIN;

DROP SCHEMA IF EXISTS xpg_advisory_example CASCADE;

CREATE SCHEMA xpg_advisory_example;

CREATE TABLE xpg_advisory_example.job_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    worker text NOT NULL,
    lock_key bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

COMMIT;
