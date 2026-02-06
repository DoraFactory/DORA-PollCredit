CREATE TABLE IF NOT EXISTS orders (
  order_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  recipient_address TEXT NOT NULL,
  derivation_index BIGINT NOT NULL,
  credit_requested BIGINT NOT NULL,
  amount_peaka TEXT NOT NULL,
  denom TEXT NOT NULL,
  price_snapshot JSONB NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL,
  paid_at TIMESTAMPTZ,
  tx_hash TEXT,
  credit_issued BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS order_derivation_index_seq START 1;

CREATE UNIQUE INDEX IF NOT EXISTS orders_recipient_address_uq ON orders (recipient_address);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);
CREATE INDEX IF NOT EXISTS orders_expires_at_idx ON orders (expires_at);

CREATE TABLE IF NOT EXISTS payments (
  tx_hash TEXT PRIMARY KEY,
  order_id TEXT NOT NULL REFERENCES orders(order_id),
  from_address TEXT,
  to_address TEXT NOT NULL,
  amount_peaka TEXT NOT NULL,
  denom TEXT NOT NULL,
  height BIGINT NOT NULL,
  block_time TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS payments_order_id_idx ON payments (order_id);
CREATE INDEX IF NOT EXISTS payments_to_address_idx ON payments (to_address);

CREATE TABLE IF NOT EXISTS sync_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
