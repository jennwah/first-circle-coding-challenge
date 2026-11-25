CREATE TABLE IF NOT EXISTS accounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner TEXT NOT NULL CHECK (btrim(owner) <> ''),
    balance NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id),
    transfer_id UUID,
    type TEXT NOT NULL,
    amount NUMERIC(20, 2) NOT NULL CHECK (amount > 0),
    balance_after NUMERIC(20, 2) NOT NULL CHECK (balance_after >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transactions_type_check CHECK (type IN ('deposit', 'withdrawal', 'transfer_in', 'transfer_out')),
    CONSTRAINT transfer_reference CHECK ((type IN ('transfer_in', 'transfer_out')) = (transfer_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS transactions_account_created_idx ON transactions (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS transactions_transfer_idx ON transactions (transfer_id) WHERE transfer_id IS NOT NULL;
