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
    'replica-two',
    'replica'
);

ALTER DATABASE postgres
SET default_transaction_read_only = on;
