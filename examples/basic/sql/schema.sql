CREATE SCHEMA IF NOT EXISTS xpg_basic_example;

CREATE TABLE IF NOT EXISTS xpg_basic_example.users (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    email text NOT NULL UNIQUE,
    active boolean NOT NULL
);
