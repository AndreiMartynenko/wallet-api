CREATE TABLE users (
                        username TEXT PRIMARY KEY,
                        password_hash TEXT NOT NULL,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency_keys (
                                   key TEXT PRIMARY KEY,
                                   response_status INT NOT NULL,
                                   response_body BYTEA NOT NULL,
                                   created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
