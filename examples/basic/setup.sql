BEGIN;

DROP SCHEMA IF EXISTS xpg_basic_example CASCADE;

CREATE SCHEMA xpg_basic_example;

CREATE TABLE xpg_basic_example.users (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    email text NOT NULL UNIQUE,
    active boolean NOT NULL
);

COMMIT;
