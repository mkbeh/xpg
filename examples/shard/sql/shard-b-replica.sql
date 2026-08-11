INSERT INTO xpg_shard_example.node_info (node_name, node_role)
VALUES ('shard-b-replica', 'replica');

ALTER DATABASE postgres
SET default_transaction_read_only = on;
