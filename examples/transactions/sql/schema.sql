CREATE SCHEMA IF NOT EXISTS xpg_transactions_example;

CREATE TABLE IF NOT EXISTS xpg_transactions_example.orders (
    id bigint PRIMARY KEY,
    status text NOT NULL
);

CREATE TABLE IF NOT EXISTS xpg_transactions_example.promo_redemptions (
    code text PRIMARY KEY,
    order_id bigint NOT NULL
);

INSERT INTO xpg_transactions_example.promo_redemptions (code, order_id)
VALUES ('PROMO2026', 100)
ON CONFLICT (code) DO UPDATE
SET order_id = EXCLUDED.order_id;
