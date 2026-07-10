CREATE TABLE accounts (
                          id TEXT PRIMARY KEY,
                          owner TEXT NOT NULL,
                          balance BIGINT NOT NULL DEFAULT 0,
                          created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE transactions (
                              id BIGSERIAL PRIMARY KEY,
                              debit_account_id TEXT NOT NULL REFERENCES accounts(id),
                              credit_account_id TEXT NOT NULL REFERENCES accounts(id),
                              amount BIGINT NOT NULL CHECK (amount > 0),
                              created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);