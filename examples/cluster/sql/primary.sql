CREATE SCHEMA xpg_cluster_example;

CREATE TABLE xpg_cluster_example.node_info (
    node_name text PRIMARY KEY,
    node_role text NOT NULL
);

INSERT INTO xpg_cluster_example.node_info (
    node_name,
    node_role
)
VALUES (
    'primary',
    'primary'
);

CREATE TABLE xpg_cluster_example.primary_writes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);
