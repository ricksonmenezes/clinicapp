CREATE TABLE services (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    price               NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    requires_consultant BOOLEAN NOT NULL DEFAULT TRUE,
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
