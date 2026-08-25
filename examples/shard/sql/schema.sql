DROP SCHEMA IF EXISTS xpg_shard_example CASCADE;

CREATE SCHEMA xpg_shard_example;

CREATE TABLE xpg_shard_example.users (
    id bigint PRIMARY KEY,
    name text NOT NULL
);
