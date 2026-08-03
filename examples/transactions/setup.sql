BEGIN;

DROP SCHEMA IF EXISTS xpg_transactions_example CASCADE;

CREATE SCHEMA xpg_transactions_example;

CREATE TABLE xpg_transactions_example.orders
(
    id     bigint PRIMARY KEY,
    status text NOT NULL
);

CREATE TABLE xpg_transactions_example.promo_redemptions
(
    code     text PRIMARY KEY,
    order_id bigint NOT NULL
);

INSERT INTO xpg_transactions_example.promo_redemptions (code,
                                                        order_id)
VALUES ('PROMO2026',
        100);

COMMIT;