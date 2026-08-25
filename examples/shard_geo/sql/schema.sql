DROP SCHEMA IF EXISTS xpg_shard_geo_example CASCADE;

CREATE SCHEMA xpg_shard_geo_example;

CREATE TABLE xpg_shard_geo_example.tenants (
    id text PRIMARY KEY,
    name text NOT NULL,
    region text NOT NULL,
    last_seen_at timestamptz NOT NULL
);
