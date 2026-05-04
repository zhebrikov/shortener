CREATE TABLE IF NOT EXISTS gophermart_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gophermart_orders (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES gophermart_users (id) ON DELETE CASCADE,
    number TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    accrual NUMERIC(14, 2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gophermart_orders_user_uploaded
    ON gophermart_orders (user_id, uploaded_at DESC);

CREATE INDEX IF NOT EXISTS idx_gophermart_orders_status
    ON gophermart_orders (status)
    WHERE status IN ('NEW', 'PROCESSING');

CREATE TABLE IF NOT EXISTS gophermart_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES gophermart_users (id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    sum NUMERIC(14, 2) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gophermart_withdrawals_user_processed
    ON gophermart_withdrawals (user_id, processed_at DESC);
