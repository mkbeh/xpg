CREATE SCHEMA xpg_shard_example;

CREATE TABLE xpg_shard_example.node_info (
    node_name text PRIMARY KEY,
    node_role text NOT NULL
);

CREATE TABLE xpg_shard_example.users (
    id bigint PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE xpg_shard_example.countries (
    code text PRIMARY KEY,
    name text NOT NULL
);

INSERT INTO xpg_shard_example.countries (code, name)
VALUES
    ('NL', 'Netherlands'),
    ('DE', 'Germany');
