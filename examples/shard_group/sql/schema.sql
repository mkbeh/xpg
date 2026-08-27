DROP SCHEMA IF EXISTS xpg_shard_group_example CASCADE;

CREATE SCHEMA xpg_shard_group_example;

CREATE TABLE xpg_shard_group_example.users (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT false
);
